package magitrickle

import (
	"fmt"
	"sync"
	"time"

	"magitrickle/utils/intID"
	"magitrickle/utils/netfilterTools"

	"github.com/rs/zerolog/log"
	"github.com/vishvananda/netlink"
)

type TrafficManager struct {
	app *App

	// Map interfaceName -> InterfaceManager
	interfaces map[string]*InterfaceManager
	mu         sync.RWMutex
}

func NewTrafficManager(app *App) *TrafficManager {
	return &TrafficManager{
		app:        app,
		interfaces: make(map[string]*InterfaceManager),
	}
}

// InterfaceManager manages resources for a single network interface
type InterfaceManager struct {
	Name        string
	IPSet       *netfilterTools.IPSet
	IPSetToLink *netfilterTools.IPSetToLink

	// Static Rules State: GroupID -> Set of Subnets
	// We use strings for keys to simplify (subnet.String()) or just store slice
	v4GroupRules map[intID.ID]map[netfilterTools.IPv4Subnet]bool
	v6GroupRules map[intID.ID]map[netfilterTools.IPv6Subnet]bool

	// Dynamic Rules Tracking: GroupID -> Subnet -> Expiration
	v4DynamicRules map[intID.ID]map[netfilterTools.IPv4Subnet]time.Time
	v6DynamicRules map[intID.ID]map[netfilterTools.IPv6Subnet]time.Time

	locker sync.Mutex
}

func (tm *TrafficManager) GetInterfaceManager(ifaceName string) (*InterfaceManager, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if im, ok := tm.interfaces[ifaceName]; ok {
		return im, nil
	}

	// Create new manager
	// Sanitize interface name for ipset name?
	// ipset names are limited in length (~31 chars?).
	// We use a safe prefix.
	ipsetName := "if_" + ifaceName
	// Check length limit if needed, but "if_vpn0" is fine.

	nfHelper := tm.app.nfHelper
	ipset := nfHelper.IPSet(ipsetName)
	ipsetToLink := nfHelper.IPSetToLink(ipsetName, ifaceName, ipset)

	im := &InterfaceManager{
		Name:         ifaceName,
		IPSet:        ipset,
		IPSetToLink:  ipsetToLink,
		v4GroupRules: make(map[intID.ID]map[netfilterTools.IPv4Subnet]bool),
		v6GroupRules: make(map[intID.ID]map[netfilterTools.IPv6Subnet]bool),
		// Dynamic Rules Tracking: GroupID -> Subnet -> Expiration
		v4DynamicRules: make(map[intID.ID]map[netfilterTools.IPv4Subnet]time.Time),
		v6DynamicRules: make(map[intID.ID]map[netfilterTools.IPv6Subnet]time.Time),
	}

	go im.garbageCollector()

	// Initialize the ipset/iptables
	// We should probably loop updates? Or just Init.
	// We'll mimic Group.enable() logic here but for the interface.
	if err := ipsetToLink.ClearIfDisabled(); err != nil {
		return nil, fmt.Errorf("failed to clear iptables for %s: %w", ifaceName, err)
	}
	if err := ipset.Enable(); err != nil {
		return nil, fmt.Errorf("failed to enable ipset for %s: %w", ifaceName, err)
	}
	if err := ipsetToLink.Enable(); err != nil {
		return nil, fmt.Errorf("failed to link ipset using iptables for %s: %w", ifaceName, err)
	}

	tm.interfaces[ifaceName] = im
	return im, nil
}

// AddDynamicIPv4 adds an IP with TTL (from DNS).
// Safe to call concurrently.
func (tm *TrafficManager) AddDynamicIPv4(groupID intID.ID, ifaceName string, subnet netfilterTools.IPv4Subnet, ttlSeconds uint32) error {
	im, err := tm.GetInterfaceManager(ifaceName)
	if err != nil {
		return err
	}

	im.locker.Lock()
	defer im.locker.Unlock()

	// Optimization: Check if this subnet is already statically routed?
	isStatic := false
	for _, rules := range im.v4GroupRules {
		if rules[subnet] {
			isStatic = true
			break
		}
	}
	if isStatic {
		return nil
	}

	// Check if already exists with longer/equal valid TTL?
	// netlink.IpsetAdd handles this (Replace: true).
	// We just update our map tracking.

	if im.v4DynamicRules[groupID] == nil {
		im.v4DynamicRules[groupID] = make(map[netfilterTools.IPv4Subnet]time.Time)
	}
	im.v4DynamicRules[groupID][subnet] = time.Now().Add(time.Duration(ttlSeconds) * time.Second)

	// Ensure interface is enabled since we are adding a rule
	if err := im.ensureEnabled(); err != nil {
		log.Error().Err(err).Str("iface", ifaceName).Msg("failed to ensure interface enabled")
	}

	ttl := netfilterTools.IPSetTimeout(&ttlSeconds)
	return im.IPSet.AddIPv4Subnet(subnet, ttl)
}

func (tm *TrafficManager) AddDynamicIPv6(groupID intID.ID, ifaceName string, subnet netfilterTools.IPv6Subnet, ttlSeconds uint32) error {
	im, err := tm.GetInterfaceManager(ifaceName)
	if err != nil {
		return err
	}

	im.locker.Lock()
	defer im.locker.Unlock()

	isStatic := false
	for _, rules := range im.v6GroupRules {
		if rules[subnet] {
			isStatic = true
			break
		}
	}
	if isStatic {
		return nil
	}

	if im.v6DynamicRules[groupID] == nil {
		im.v6DynamicRules[groupID] = make(map[netfilterTools.IPv6Subnet]time.Time)
	}
	im.v6DynamicRules[groupID][subnet] = time.Now().Add(time.Duration(ttlSeconds) * time.Second)

	// Ensure interface is enabled
	if err := im.ensureEnabled(); err != nil {
		log.Error().Err(err).Str("iface", ifaceName).Msg("failed to ensure interface enabled")
	}

	ttl := netfilterTools.IPSetTimeout(&ttlSeconds)
	return im.IPSet.AddIPv6Subnet(subnet, ttl)
}

// UpdateGroupRules recalculates static rules for a group on a specific interface
func (tm *TrafficManager) UpdateGroupRules(groupID intID.ID, ifaceName string, v4Subnets map[netfilterTools.IPv4Subnet]bool, v6Subnets map[netfilterTools.IPv6Subnet]bool) error {
	im, err := tm.GetInterfaceManager(ifaceName)
	if err != nil {
		return err
	}

	return im.SyncGroup(groupID, v4Subnets, v6Subnets)
}

// RemoveGroupRules cleans up rules for a group (e.g. disable or change interface)
func (tm *TrafficManager) RemoveGroupRules(groupID intID.ID, ifaceName string) error {
	// If interface doesn't exist in our map, nothing to do
	tm.mu.RLock()
	im, ok := tm.interfaces[ifaceName]
	tm.mu.RUnlock()

	if !ok {
		return nil
	}

	if err := im.ClearDynamicRules(groupID); err != nil {
		log.Error().Err(err).Str("iface", ifaceName).Msg("failed to clear dynamic rules")
	}

	return im.SyncGroup(groupID, nil, nil)
}

// NetfilterDHook restores rules after system firewall reload
func (tm *TrafficManager) NetfilterDHook(action, table string) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	for _, im := range tm.interfaces {
		if err := im.IPSetToLink.NetfilterDHook(action, table); err != nil {
			log.Error().Err(err).Str("iface", im.Name).Msg("failed to restore rules in NetfilterDHook")
		}
	}
}

// HandleLinkUpdate propagates link change events to all interface managers
func (tm *TrafficManager) HandleLinkUpdate(event netlink.LinkUpdate) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// We could optimize by checking if event.Name matches im.Name
	// But IPSetToLink internally checks matching interface/index.
	// For efficiency, let's match name if possible
	ifaceName := event.Link.Attrs().Name

	if im, ok := tm.interfaces[ifaceName]; ok {
		if err := im.IPSetToLink.LinkUpdateHook(event); err != nil {
			log.Error().Err(err).Str("iface", ifaceName).Msg("failed to handle link update")
		}
	}
}

// --- InterfaceManager Methods ---

func (im *InterfaceManager) IsStaticallyRoutedIPv4(subnet netfilterTools.IPv4Subnet) bool {
	im.locker.Lock()
	defer im.locker.Unlock()

	for _, rules := range im.v4GroupRules {
		if rules[subnet] {
			return true
		}
	}
	return false
}

func (im *InterfaceManager) IsStaticallyRoutedIPv6(subnet netfilterTools.IPv6Subnet) bool {
	// Note: called with lock held by AddDynamicIPv6 from caller, BUT here we lock again?
	// No, AddDynamicIPv6 in new code calculates lock on `im.locker`.
	// Wait, `im.IsStaticallyRoutedIPv6` locks internally.
	// Recursive locking is NOT supported by sync.Mutex.
	// We must remove locking from `IsStaticallyRoutedIPv6` if we call it from locked context.
	// Or change `AddDynamic` to NOT lock around this call.
	// The original `AddDynamic` logic: `im.GetInterfaceManager` -> `im.IsStaticallyRouted...`.
	// `GetInterfaceManager` locks `tm.mu`, not `im`.
	// `IsStaticallyRouted` locks `im`.
	// In NEW `AddDynamic`, I lock `im` then call `IsStaticallyRouted`. Deadlock!

	// FIX: Access `im.v6GroupRules` directly since we hold lock in caller.
	// I will remove `IsStaticallyRouted` helper calls from `AddDynamic` and inline the check.
	// Leaving this method AS IS for other callers (e.g. tests?).
	// Actually, `SyncGroup` uses `isV4InOtherGroups` internal helper.

	im.locker.Lock()
	defer im.locker.Unlock()

	for _, rules := range im.v6GroupRules {
		if rules[subnet] {
			return true
		}
	}
	return false
}

func (im *InterfaceManager) ClearDynamicRules(groupID intID.ID) error {
	im.locker.Lock()
	defer im.locker.Unlock()

	// Clear V4
	if rules, ok := im.v4DynamicRules[groupID]; ok {
		for subnet := range rules {
			// Try to delete from ipset. Ignore errors (might be expired/gone).
			_ = im.IPSet.DelIPv4Subnet(subnet)
		}
		delete(im.v4DynamicRules, groupID)
	}

	// Clear V6
	if rules, ok := im.v6DynamicRules[groupID]; ok {
		for subnet := range rules {
			_ = im.IPSet.DelIPv6Subnet(subnet)
		}
		delete(im.v6DynamicRules, groupID)
	}

	log.Debug().Str("iface", im.Name).Str("gid", groupID.String()).Msg("cleared dynamic rules for group")
	return nil
}

func (im *InterfaceManager) garbageCollector() {
	ticker := time.NewTicker(2 * time.Minute) // Check every 2 mins
	for range ticker.C {
		im.locker.Lock()
		now := time.Now()

		// Clean V4
		for _, rules := range im.v4DynamicRules {
			for subnet, deadline := range rules {
				if now.After(deadline) {
					delete(rules, subnet)
				}
			}
		}
		// Clean V6
		for _, rules := range im.v6DynamicRules {
			for subnet, deadline := range rules {
				if now.After(deadline) {
					delete(rules, subnet)
				}
			}
		}
		im.locker.Unlock()
	}
}

func (im *InterfaceManager) SyncGroup(groupID intID.ID, cv4 map[netfilterTools.IPv4Subnet]bool, cv6 map[netfilterTools.IPv6Subnet]bool) error {
	im.locker.Lock()
	defer im.locker.Unlock()

	// Calculate total active groups count (0 -> 1 transition requires re-enable)
	// Calculate total active groups count (0 -> 1 transition requires re-enable)
	wasActive := im.hasActiveRules()

	// 1. Calculate the 'Before' state of the Union (all groups)
	// (Existing logic kept)...

	// Convert new lists to sets - REMOVED, passing maps directly
	newV4Set := cv4
	if newV4Set == nil {
		newV4Set = make(map[netfilterTools.IPv4Subnet]bool)
	}

	newV6Set := cv6
	if newV6Set == nil {
		newV6Set = make(map[netfilterTools.IPv6Subnet]bool)
	}

	// If transitioning from inactive to active, ensure resources are ready
	// Actually we should check if we ARE acquiring new rules.
	// But `v4GroupRules` is updated later.
	// Let's optimize:
	// If `!wasActive` and `(len(newV4Set) > 0 || len(newV6Set) > 0)`... wait.
	// `SyncGroup` REPLACES the rules for THIS group.
	// So `wasActive` uses OLD state.
	// Let's keep it simple: ensure enabled if we have ANY rules.

	// Pre-Enable check
	// Note: We modifying im.v4GroupRules below.

	oldV4Set := im.v4GroupRules[groupID]
	oldV6Set := im.v6GroupRules[groupID]

	// Update state
	if len(newV4Set) > 0 {
		im.v4GroupRules[groupID] = newV4Set
	} else {
		delete(im.v4GroupRules, groupID)
	}
	if len(newV6Set) > 0 {
		im.v6GroupRules[groupID] = newV6Set
	} else {
		delete(im.v6GroupRules, groupID)
	}

	// Now check if active
	nowActive := im.hasActiveRules()

	if nowActive && !wasActive {
		// Re-enable if it was disabled (or ensure it's enabled)
		if err := im.ensureEnabled(); err != nil {
			log.Error().Err(err).Str("iface", im.Name).Msg("failed to enable interface resources")
		}
	}

	// Diff V4 (Apply changes)
	// To add: in New but not in Old
	for s := range newV4Set {
		if !oldV4Set[s] {
			if !im.isV4InOtherGroups(s, groupID) {
				if err := im.IPSet.AddIPv4Subnet(s, nil); err != nil {
					log.Error().Err(err).Str("subnet", s.String()).Msg("failed to add static subnet")
				}
			}
		}
	}
	// To del: in Old but not in New
	for s := range oldV4Set {
		if !newV4Set[s] {
			if !im.isV4InOtherGroups(s, groupID) {
				if err := im.IPSet.DelIPv4Subnet(s); err != nil {
					log.Error().Err(err).Str("subnet", s.String()).Msg("failed to del static subnet")
				}
			}
		}
	}

	// Diff V6
	for s := range newV6Set {
		if !oldV6Set[s] {
			if !im.isV6InOtherGroups(s, groupID) {
				if err := im.IPSet.AddIPv6Subnet(s, nil); err != nil {
					log.Error().Err(err).Str("subnet", s.String()).Msg("failed to add static subnet v6")
				}
			}
		}
	}
	for s := range oldV6Set {
		if !newV6Set[s] {
			if !im.isV6InOtherGroups(s, groupID) {
				if err := im.IPSet.DelIPv6Subnet(s); err != nil {
					log.Error().Err(err).Str("subnet", s.String()).Msg("failed to del static subnet v6")
				}
			}
		}
	}

	// If no groups left on this interface, flush dynamic rules AND DISABLE
	if !nowActive {
		if err := im.IPSetToLink.Disable(); err != nil {
			log.Error().Err(err).Msg("failed to disable ipset-to-link")
		}
		if err := im.IPSet.Flush(); err != nil {
			log.Error().Err(err).Msg("failed to flush ipset after removing last group")
		} else {
			log.Debug().Msg("flushed ipset due to no active groups")
		}
		// Optional: im.IPSet.Disable()? Might be overkill if we just flushed.
	}

	return nil
}

func (im *InterfaceManager) hasActiveRules() bool {
	if len(im.v4GroupRules) > 0 || len(im.v6GroupRules) > 0 {
		return true
	}
	if len(im.v4DynamicRules) > 0 || len(im.v6DynamicRules) > 0 {
		return true
	}
	return false
}

func (im *InterfaceManager) ensureEnabled() error {
	if err := im.IPSet.Enable(); err != nil {
		return fmt.Errorf("failed to enable ipset: %w", err)
	}
	if err := im.IPSetToLink.Enable(); err != nil {
		return fmt.Errorf("failed to enable ipset-to-link: %w", err)
	}
	return nil
}

func (im *InterfaceManager) isV4InOtherGroups(subnet netfilterTools.IPv4Subnet, ignoreGroupID intID.ID) bool {
	for gid, rules := range im.v4GroupRules {
		if gid == ignoreGroupID {
			continue
		}
		if rules[subnet] {
			return true
		}
	}
	return false
}

func (im *InterfaceManager) isV6InOtherGroups(subnet netfilterTools.IPv6Subnet, ignoreGroupID intID.ID) bool {
	for gid, rules := range im.v6GroupRules {
		if gid == ignoreGroupID {
			continue
		}
		if rules[subnet] {
			return true
		}
	}
	return false
}

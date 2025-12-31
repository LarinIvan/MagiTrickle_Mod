package magitrickle

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"magitrickle/models"
	"magitrickle/utils/netfilterTools"

	"github.com/vishvananda/netlink"
)

var (
	ipv4SubnetRe = regexp.MustCompile(`^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})(?:/(\d{1,2}))?$`)
)

type Group struct {
	*models.Group

	enabled atomic.Bool
	locker  sync.Mutex

	app *App
}

func (g *Group) Enabled() bool {
	return g.enabled.Load()
}

func NewGroup(group *models.Group, app *App) (*Group, error) {
	return &Group{
		Group: group,
		app:   app,
	}, nil
}

func (g *Group) Model() *models.Group {
	return g.Group
}

// Enable activates the group
func (g *Group) Enable() error {
	g.locker.Lock()
	defer g.locker.Unlock()

	if !g.Group.Enable {
		return nil
	}

	if !g.enabled.CompareAndSwap(false, true) {
		return nil
	}

	// Calculate and push rules to TrafficManager
	if err := g.sync(); err != nil {
		g.enabled.Store(false)
		return err
	}

	// Update Trie to include this group's domains
	g.app.RebuildTrie()

	return nil
}

// Disable deactivates the group
func (g *Group) Disable() error {
	g.locker.Lock()
	defer g.locker.Unlock()

	if !g.enabled.CompareAndSwap(true, false) {
		return nil
	}

	// Remove rules from TrafficManager
	if err := g.app.trafficManager.RemoveGroupRules(g.ID, g.Interface); err != nil {
		return fmt.Errorf("failed to remove group rules: %w", err)
	}

	// Update Trie to remove this group's domains
	g.app.RebuildTrie()

	return nil
}

// Sync updates the group configuration (called when Config changes)
func (g *Group) Sync() error {
	g.locker.Lock()
	defer g.locker.Unlock()

	if !g.Enabled() {
		return nil
	}

	// If group is conceptually disabled in config
	if !g.Group.Enable {
		return g.app.trafficManager.RemoveGroupRules(g.ID, g.Interface)
	}

	if err := g.sync(); err != nil {
		return err
	}

	g.app.RebuildTrie()
	return nil
}

func (g *Group) sync() error {
	var v4Subnets []netfilterTools.IPv4Subnet
	var v6Subnets []netfilterTools.IPv6Subnet

RuleLoop:
	for _, domain := range g.Rules {
		if !domain.IsEnabled() {
			continue
		}

		// Parse Subnet Rules directly
		if domain.Type == "subnet" {
			matches := ipv4SubnetRe.FindStringSubmatch(domain.Rule)
			if matches == nil {
				continue
			}

			var addr [4]byte
			for i := 1; i <= 4; i++ {
				n, _ := strconv.Atoi(matches[i])
				if n > 255 {
					continue RuleLoop
				}
				addr[i-1] = uint8(n)
			}

			var cidr uint8
			if matches[5] != "" {
				n, _ := strconv.Atoi(matches[5])
				if n > 32 {
					continue RuleLoop
				}
				cidr = uint8(n)
				// Apply mask
				addr = [4]byte(net.IP(addr[:]).Mask(net.CIDRMask(n, 32)))
			} else {
				// No CIDR means /32 (single IP)
				cidr = 32
			}

			if !(addr == [4]byte{0, 0, 0, 0} && cidr == 0) {
				v4Subnets = append(v4Subnets, netfilterTools.IPv4Subnet{Address: addr, CIDR: cidr})
			} else {
				// 0.0.0.0/1 and 128.0.0.0/1 hack for 0.0.0.0/0
				v4Subnets = append(v4Subnets, netfilterTools.IPv4Subnet{Address: [4]byte{0, 0, 0, 0}, CIDR: 1})
				v4Subnets = append(v4Subnets, netfilterTools.IPv4Subnet{Address: [4]byte{128, 0, 0, 0}, CIDR: 1})
			}
			continue
		}

		// Process "Domain" rules using cache to speed up "hot" enable
		if domain.Type == "domain" {
			knownDomains := g.app.recordsCache.ListKnownDomains()
			for _, known := range knownDomains {
				if domain.IsMatch(known) {
					addresses := g.app.recordsCache.GetAddresses(known)
					for _, addr := range addresses {
						ttl := time.Until(addr.Deadline).Seconds()
						if ttl <= 0 {
							continue
						}

						if len(addr.Address) == net.IPv4len {
							subnet := netfilterTools.IPv4Subnet{Address: [4]byte(addr.Address), CIDR: 32}
							if err := g.app.trafficManager.AddDynamicIPv4(g.ID, g.Interface, subnet, uint32(ttl)); err != nil {
								// Log error but continue
							}
						} else if len(addr.Address) == net.IPv6len {
							subnet := netfilterTools.IPv6Subnet{Address: [16]byte(addr.Address), CIDR: 128}
							if err := g.app.trafficManager.AddDynamicIPv6(g.ID, g.Interface, subnet, uint32(ttl)); err != nil {
								// Log error
							}
						}
					}
				}
			}
			continue
		}
	}

	return g.app.trafficManager.UpdateGroupRules(g.ID, g.Interface, v4Subnets, v6Subnets)
}

// Legacy hooks that might be called (though we aim to remove them from caller too)
func (g *Group) LinkUpdateHook(event netlink.LinkUpdate) error {
	// TrafficManager handles interfaces, but it might need to know about Link Updates?
	// Helper/IPSetToLink usually handles this if we used the generic link hook.
	// But `TrafficManager` owns the `IPSetToLink` now.
	// So `netlink.go` should notify `TrafficManager`, not `Group`.
	return nil
}

func (g *Group) NetfilterDHook(iptType, table string) error {
	return nil
}

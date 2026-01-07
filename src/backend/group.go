package magitrickle

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"magitrickle/models"
	"magitrickle/utils/netfilterTools"

	"github.com/vishvananda/netlink"
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
	newIPv4SubnetList := make(map[netfilterTools.IPv4Subnet]bool)
	newIPv6SubnetList := make(map[netfilterTools.IPv6Subnet]bool)

	for _, domain := range g.Rules {
		if !domain.IsEnabled() {
			continue
		}

		switch domain.Type {
		case "subnet":
			ip, ipNet, err := net.ParseCIDR(domain.Rule)
			if err != nil {
				ip = net.ParseIP(domain.Rule)
				if ip == nil {
					continue
				}

				ip = ip.To4()
				if ip == nil {
					continue
				}

				ipNet = &net.IPNet{
					IP:   ip,
					Mask: net.CIDRMask(32, 32),
				}
			}

			ones, bits := ipNet.Mask.Size()
			if bits != 32 || ones > 32 {
				continue
			}

			var addr [4]byte
			copy(addr[:], ipNet.IP.Mask(ipNet.Mask).To4())
			cidr := uint8(ones)

			if addr == ([4]byte{}) && cidr == 0 {
				newIPv4SubnetList[netfilterTools.IPv4Subnet{
					Address: [4]byte{0x00},
					CIDR:    1,
				}] = true
				newIPv4SubnetList[netfilterTools.IPv4Subnet{
					Address: [4]byte{0x80},
					CIDR:    1,
				}] = true
			} else {
				newIPv4SubnetList[netfilterTools.IPv4Subnet{
					Address: addr,
					CIDR:    cidr,
				}] = true
			}

		case "subnet6":
			ip, ipNet, err := net.ParseCIDR(domain.Rule)
			if err != nil {
				ip = net.ParseIP(domain.Rule)
				if ip == nil {
					continue
				}

				ip = ip.To16()
				if ip == nil {
					continue
				}

				ipNet = &net.IPNet{
					IP:   ip,
					Mask: net.CIDRMask(128, 128),
				}
			}

			ones, bits := ipNet.Mask.Size()
			if bits != 128 || ones > 128 {
				continue
			}

			var addr [16]byte
			copy(addr[:], ipNet.IP.Mask(ipNet.Mask).To16())
			cidr := uint8(ones)

			if addr == ([16]byte{}) && cidr == 0 {
				newIPv6SubnetList[netfilterTools.IPv6Subnet{
					Address: [16]byte{0x00},
					CIDR:    1,
				}] = true
				newIPv6SubnetList[netfilterTools.IPv6Subnet{
					Address: [16]byte{0x80},
					CIDR:    1,
				}] = true
			} else {
				newIPv6SubnetList[netfilterTools.IPv6Subnet{
					Address: addr,
					CIDR:    cidr,
				}] = true
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

	// Pass maps directly to TrafficManager (Refactor: changed to bool maps)
	return g.app.trafficManager.UpdateGroupRules(g.ID, g.Interface, newIPv4SubnetList, newIPv6SubnetList)
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

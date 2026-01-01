package magitrickle

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"magitrickle/app"
	"magitrickle/constant"
	"magitrickle/logstream"
	"magitrickle/models"
	"magitrickle/utils/dnsMITMProxy"
	"magitrickle/utils/netfilterTools"
	"magitrickle/utils/recordsCache"
	"magitrickle/utils/trie"

	"github.com/IGLOU-EU/go-wildcard/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	ErrAlreadyRunning           = errors.New("already running")
	ErrGroupIDConflict          = errors.New("group id conflict")
	ErrRuleIDConflict           = errors.New("rule id conflict")
	ErrConfigUnsupportedVersion = errors.New("config unsupported version")
)

// App – основная структура ядра приложения
type App struct {
	enabled atomic.Bool

	config   models.AppConfig
	settings models.SettingsConfig

	dnsMITM        *dnsMITMProxy.DNSMITMProxy
	nfHelper       *netfilterTools.Helper
	recordsCache   *recordsCache.Records
	groups         []*Group
	dnsOverrider   *netfilterTools.PortRemap
	trafficManager *TrafficManager
	domainTrie     atomic.Value // holds *trie.Trie

	interfaceAliases map[string]string

	regexpEnabled     atomic.Bool
	regexpRules       []*RegexpRule
	regexpRulesLocker sync.RWMutex

	wildcardEnabled     atomic.Bool
	wildcardRules       []*WildcardRule
	wildcardRulesLocker sync.RWMutex
	broadcaster         *logstream.Broadcaster
}

// New создаёт новый экземпляр App
func New(broadcaster *logstream.Broadcaster) *App {
	a := &App{
		config:           constant.DefaultAppConfig,
		interfaceAliases: make(map[string]string),
		broadcaster:      broadcaster,
	}
	if err := a.LoadConfig(); err != nil {
		log.Error().Err(err).Msg("failed to load config file")
	}
	// Load interface aliases
	if err := a.LoadInterfaceConfig(); err != nil {
		log.Error().Err(err).Msg("failed to load interface aliases")
	}
	// Load settings
	if err := a.LoadSettingsConfig(); err != nil {
		log.Error().Err(err).Msg("failed to load settings config")
	}

	a.trafficManager = NewTrafficManager(a)
	a.domainTrie.Store(trie.New())
	return a
}

func (a *App) RebuildTrie() {
	a.SyncAllRules()
}

func (a *App) Trie() *trie.Trie {
	return a.domainTrie.Load().(*trie.Trie)
}

// Config возвращает конфигурацию
func (a *App) Config() models.AppConfig {
	return a.config
}

// Settings returns the current settings
func (a *App) Settings() models.SettingsConfig {
	return a.settings
}

// SetSettings updates the settings
func (a *App) SetSettings(s models.SettingsConfig) {
	a.settings = s
	a.regexpEnabled.Store(s.EnableRegexp)

	if s.LogLevel != "" {
		if level, err := zerolog.ParseLevel(s.LogLevel); err == nil {
			zerolog.SetGlobalLevel(level)
		}
	}

	// Update Wildcard enablement
	a.wildcardEnabled.Store(s.EnableWildcard)

	// If enabled, rebuild list
	if s.EnableRegexp || s.EnableWildcard {
		a.RebuildTrie()
	} else {
		// Optimization: if both disabled, we might want to clear them?
		// But RebuildTrie (SyncAllRules) handles it by checking flags.
		a.RebuildTrie()
	}
}

// Restart triggers a service restart (via exit)
// Restart triggers a service restart (via init.d script)
func (a *App) Restart() {
	log.Info().Msg("restart requested via API")

	go func() {
		// Wait a bit to ensure the HTTP response is sent
		time.Sleep(500 * time.Millisecond)

		log.Info().Msg("Executing /opt/etc/init.d/S99magitrickle restart")
		cmd := exec.Command("/opt/etc/init.d/S99magitrickle", "restart")
		// Detach the process so it survives our death (if the script kills us)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

		if err := cmd.Start(); err != nil {
			log.Error().Err(err).Msg("Failed to start restart script, falling back to exit")
		}

		// Release the process resources
		go func() {
			_ = cmd.Wait()
		}()

		// Give the script a moment to do its thing (like stopping us), otherwise exit
		time.Sleep(2 * time.Second)
		log.Info().Msg("Exiting process now")
		os.Exit(0)
	}()
}

// Groups возвращает список групп
func (a *App) Groups() []app.Group {
	groups := make([]app.Group, len(a.groups))
	for i, g := range a.groups {
		groups[i] = g
	}
	return groups
}

// ClearGroups отключает все группы и очищает список
func (a *App) ClearGroups() {
	for _, g := range a.groups {
		_ = g.Disable()
	}
	a.groups = a.groups[:0]
}

// AddGroup добавляет новую группу
func (a *App) AddGroup(groupModel *models.Group) error {
	for _, group := range a.groups {
		if groupModel.ID == group.ID {
			return ErrGroupIDConflict
		}
	}
	// Проверка уникальности rule.ID внутри группы.
	dup := make(map[[4]byte]struct{})
	for _, rule := range groupModel.Rules {
		if _, exists := dup[rule.ID]; exists {
			return ErrRuleIDConflict
		}
		dup[rule.ID] = struct{}{}
	}

	grp, err := NewGroup(groupModel, a)
	if err != nil {
		return fmt.Errorf("failed to create group: %w", err)
	}
	a.groups = append(a.groups, grp)

	log.Info().
		Str("id", grp.ID.String()).
		Str("name", grp.Name).
		Msg("added group")

	// если приложение уже запущено – включаем группу и выполняем синхронизацию
	if a.enabled.Load() {
		if err = grp.Enable(); err != nil {
			return fmt.Errorf("failed to enable group: %w", err)
		}
		if err = grp.Sync(); err != nil {
			return fmt.Errorf("failed to sync group: %w", err)
		}
	}
	return nil
}

// RemoveGroupByIndex удаляет группу по индексу
func (a *App) RemoveGroupByIndex(idx int) {
	a.groups = append(a.groups[:idx], a.groups[idx+1:]...)
}

// ListInterfaces возвращает список сетевых интерфейсов, удовлетворяющих заданным критериям
func (a *App) ListInterfaces() ([]net.Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}

	if a.config.ShowAllInterfaces {
		return interfaces, nil
	}

	var filteredInterfaces []net.Interface
	for _, iface := range interfaces {
		if iface.Flags&net.FlagPointToPoint == 0 || slices.Contains(constant.IgnoredInterfaces, iface.Name) {
			continue
		}
		filteredInterfaces = append(filteredInterfaces, iface)
	}
	return filteredInterfaces, nil
}

// DnsOverrider возвращает dnsOverrider
func (a *App) DnsOverrider() *netfilterTools.PortRemap {
	return a.dnsOverrider
}

// TrafficManager returns the traffic manager instance
func (a *App) TrafficManager() *TrafficManager {
	return a.trafficManager
}

// InterfaceAliases returns the map of interface aliases
func (a *App) InterfaceAliases() map[string]string {
	// Return a copy to avoid race conditions
	aliases := make(map[string]string, len(a.interfaceAliases))
	for k, v := range a.interfaceAliases {
		aliases[k] = v
	}
	return aliases
}

// SetInterfaceAliases sets the map of interface aliases
func (a *App) SetInterfaceAliases(aliases map[string]string) {
	a.interfaceAliases = make(map[string]string, len(aliases))
	for k, v := range aliases {
		a.interfaceAliases[k] = v
	}
}

// NetfilterDHook restores rules after system firewall reload
func (a *App) NetfilterDHook(action, table string) error {
	if a.trafficManager != nil {
		a.trafficManager.NetfilterDHook(action, table)
	}
	return nil
}

// --- Sync Logic ---

// --- Sync Logic ---

func (a *App) SyncAllRules() {
	newTrie := trie.New()
	var newRegexps []*RegexpRule
	var newWildcards []*WildcardRule

	useRegexp := a.regexpEnabled.Load()

	for _, g := range a.groups {
		if !g.Enabled() {
			continue
		}
		for _, rule := range g.Rules {
			if !rule.IsEnabled() {
				continue
			}

			if rule.Type == "domain" {
				newTrie.Insert(rule.Rule, g)
			} else if rule.Type == "wildcard" && (strings.Contains(rule.Rule, "*") || strings.Contains(rule.Rule, "?")) {
				newWildcards = append(newWildcards, &WildcardRule{
					Rule:  rule.Rule,
					Group: g,
				})
			} else if rule.Type == "namespace" || rule.Type == "wildcard" {
				cleanDomain := strings.TrimPrefix(rule.Rule, "*.")
				newTrie.Insert(cleanDomain, g)
			} else if useRegexp && (rule.Type == "regexp" || rule.Type == "regex") {
				if r, err := regexp.Compile(rule.Rule); err == nil {
					newRegexps = append(newRegexps, &RegexpRule{
						Re:    r,
						Group: g,
					})
				} else {
					log.Warn().Err(err).Str("rule", rule.Rule).Str("group", g.Name).Msg("failed to compile regexp rule")
				}
			}
		}
	}
	a.domainTrie.Store(newTrie)

	a.regexpRulesLocker.Lock()
	a.regexpRules = newRegexps
	a.regexpRulesLocker.Unlock()

	a.wildcardRulesLocker.Lock()
	a.wildcardRules = newWildcards
	a.wildcardRulesLocker.Unlock()

	log.Debug().Int("regexps", len(newRegexps)).Int("wildcards", len(newWildcards)).Msg("Rules synced (Trie + Wildcard + Regexp)")
}

func (a *App) SyncRegexpOnly() {
	var newRegexps []*RegexpRule

	// If disabled, just clear
	if !a.regexpEnabled.Load() {
		a.DisableRegexp()
		return
	}

	for _, g := range a.groups {
		if !g.Enabled() {
			continue
		}
		for _, rule := range g.Rules {
			if !rule.IsEnabled() {
				continue
			}
			if rule.Type == "regexp" || rule.Type == "regex" {
				if r, err := regexp.Compile(rule.Rule); err == nil {
					newRegexps = append(newRegexps, &RegexpRule{
						Re:    r,
						Group: g,
					})
				} else {
					log.Warn().Err(err).Str("rule", rule.Rule).Str("group", g.Name).Msg("failed to compile regexp rule")
				}
			}
		}
	}

	a.regexpRulesLocker.Lock()
	a.regexpRules = newRegexps
	a.regexpRulesLocker.Unlock()
	log.Debug().Int("regexps", len(newRegexps)).Msg("Rules synced (Regexp Only)")
}

func (a *App) DisableRegexp() {
	a.regexpRulesLocker.Lock()
	a.regexpRules = nil
	a.regexpRulesLocker.Unlock()
	log.Debug().Msg("Regexp rules cleared")
}

func (a *App) SearchRegexp(domain string) (*Group, bool) {
	if !a.regexpEnabled.Load() {
		return nil, false
	}

	a.regexpRulesLocker.RLock()
	defer a.regexpRulesLocker.RUnlock()

	for _, rr := range a.regexpRules {
		if rr.Re.MatchString(domain) {
			return rr.Group, true
		}
	}
	return nil, false
}

func (a *App) LogBroadcaster() *logstream.Broadcaster {
	return a.broadcaster
}

func (a *App) SearchWildcard(domain string) (*Group, bool) {
	if !a.wildcardEnabled.Load() {
		return nil, false
	}

	a.wildcardRulesLocker.RLock()
	defer a.wildcardRulesLocker.RUnlock()

	for _, wr := range a.wildcardRules {
		if wildcard.Match(wr.Rule, domain) {
			return wr.Group, true
		}
	}
	return nil, false
}

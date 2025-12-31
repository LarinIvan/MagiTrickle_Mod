package app

import (
	"context"
	"net"

	"magitrickle/config"
	"magitrickle/logstream"
	"magitrickle/models"
	"magitrickle/utils/netfilterTools"
)

type Main interface {
	Config() models.AppConfig
	Groups() []Group
	ClearGroups()
	AddGroup(groupModel *models.Group) error
	RemoveGroupByIndex(idx int)
	ListInterfaces() ([]net.Interface, error)
	DnsOverrider() *netfilterTools.PortRemap
	LoadConfig() error
	SaveConfig() error
	ImportConfig(cfg config.Config) error
	ExportConfig() config.Config
	Start(ctx context.Context) (err error)
	NetfilterDHook(action, table string) error
	InterfaceAliases() map[string]string
	SetInterfaceAliases(aliases map[string]string)
	LoadInterfaceConfig() error
	SaveInterfaceConfig() error
	Settings() models.SettingsConfig
	SetSettings(s models.SettingsConfig)
	LoadSettingsConfig() error
	SaveSettingsConfig() error
	Restart()
	LogBroadcaster() *logstream.Broadcaster
}

type Group interface {
	Enabled() bool
	Model() *models.Group
	Enable() error
	Disable() error
	Sync() error
}

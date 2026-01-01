package models

type SettingsConfig struct {
	EnableRegexp     bool   `yaml:"enable_regexp" json:"enable_regexp"`
	LogLevel         string `yaml:"log_level" json:"log_level"`
	ShowInterfaceIPs bool   `yaml:"show_interface_ips" json:"show_interface_ips"`
	AutoCheckUpdates bool   `yaml:"auto_check_updates" json:"auto_check_updates"`
	EnableWildcard   bool   `yaml:"enable_wildcard" json:"enable_wildcard"`
}

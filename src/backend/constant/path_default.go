//go:build !entware && !openwrt

package constant

const (
	AppConfigDir = "/etc/magitrickle"
	AppShareDir  = "/usr/share/magitrickle"
	AppStateDir  = "/var/lib/magitrickle"
	PIDPath      = "/var/run/magitrickle.pid"
	SockPath     = "/var/run/magitrickle.sock"

	InitScriptPath  = "/etc/init.d/magitrickle"
	OpkgUpgradeArgs = ""

	// Package update settings
	PackageURLTemplate = ""
	RepoURL            = "" // Not used for default platform
	RepoConfPath       = "" // Not used for default platform
	PlatformName       = "default"
)

//go:build openwrt

package constant

const (
	AppConfigDir = "/etc/magitrickle"
	AppShareDir  = "/usr/share/magitrickle"
	AppStateDir  = "/etc/magitrickle/state"
	PIDPath      = "/var/run/magitrickle.pid"
	SockPath     = "/var/run/magitrickle.sock"

	InitScriptPath  = "/etc/init.d/magitrickle"
	OpkgUpgradeArgs = "--force-checksum"

	// Package update settings
	PackageURLTemplate = "https://github.com/LarinIvan/MagiTrickle_Mod/releases/latest/download/magitrickle_mod_openwrt-%s.ipk"
	RepoURL            = "" // Not used for OpenWRT (direct package URL installation)
	RepoConfPath       = "" // Not used for OpenWRT
	PlatformName       = "openwrt"
)

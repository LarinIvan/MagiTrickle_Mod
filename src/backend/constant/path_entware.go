//go:build entware

package constant

const (
	AppConfigDir = "/opt/etc/magitrickle"
	AppShareDir  = "/opt/usr/share/magitrickle"
	AppStateDir  = "/opt/var/lib/magitrickle"
	PIDPath      = "/opt/var/run/magitrickle.pid"
	SockPath     = "/opt/var/run/magitrickle.sock"

	InitScriptPath  = "/opt/etc/init.d/S99magitrickle"
	OpkgUpgradeArgs = ""

	// Package update settings (empty for Entware - uses standard opkg upgrade)
	PackageURLTemplate = ""
	RepoURL            = "https://github.com/LarinIvan/MagiTrickle_Mod/releases/latest/download"
	RepoConfPath       = "/opt/etc/opkg/magitrickle_mod.conf"
	PlatformName       = "entware"
)

//go:build !linux

package diagnostics

import "github.com/rs/zerolog/log"

func bindToDevice(fd uintptr, ifaceName string) {
	// No-op on non-Linux systems
	log.Debug().Str("interface", ifaceName).Msg("skipping SO_BINDTODEVICE on non-linux system")
}

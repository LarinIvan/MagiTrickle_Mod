//go:build linux

package diagnostics

import (
	"syscall"

	"github.com/rs/zerolog/log"
)

func bindToDevice(fd uintptr, ifaceName string) {
	if err := syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, ifaceName); err != nil {
		log.Warn().Err(err).Str("interface", ifaceName).Msg("failed to bind socket to device")
	}
}

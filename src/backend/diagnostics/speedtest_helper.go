package diagnostics

import (
	neturl "net/url"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/showwin/speedtest-go/speedtest"
	"github.com/showwin/speedtest-go/speedtest/transport"
)

// measurePacketLoss executes the packet loss test with protocol fallback
func measurePacketLoss(target *speedtest.Server) float64 {
	// Host sanitization and fallback
	baseHost := target.Host

	// Fallback: If Host is empty (common in FetchServerByID results), try to extract from URL
	if baseHost == "" && target.URL != "" {
		if u, err := neturl.Parse(target.URL); err == nil {
			baseHost = u.Host
		}
	}

	// Strip existing schemes to start clean
	baseHost = strings.TrimPrefix(baseHost, "http://")
	baseHost = strings.TrimPrefix(baseHost, "https://")
	baseHost = strings.TrimPrefix(baseHost, "tcp://")
	baseHost = strings.TrimPrefix(baseHost, "udp://")

	protocols := []string{"", "tcp://", "http://", "https://", "udp://"}

	var lastLoss float64
	var success bool

	for _, proto := range protocols {
		currentHost := proto + baseHost
		analyzer := speedtest.NewPacketLossAnalyzer(nil)

		log.Debug().Str("host", currentHost).Msg("Attempting packet loss test")

		err := analyzer.Run(currentHost, func(pl *transport.PLoss) {
			lastLoss = pl.Loss()
		})

		if err == nil {
			success = true
			break // Success
		}

		log.Warn().Err(err).Str("host", currentHost).Msg("Packet loss attempt failed")
	}

	if !success {
		log.Error().Msg("All packet loss attempts failed")

		return -1
	}
	return lastLoss
}

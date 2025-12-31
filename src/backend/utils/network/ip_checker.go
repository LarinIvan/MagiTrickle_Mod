//go:build linux

package network

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// CheckExternalIP determines the external IP address by sending a request
// through the specified network interface.
func CheckExternalIP(ifaceName string) (string, error) {
	// API services to check IP (primary and fallback)
	services := []string{
		"https://api.ipify.org",
		"http://ifconfig.me/ip",
		"http://ipecho.net/plain",
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: 7 * time.Second,
					Control: func(network, address string, c syscall.RawConn) error {
						return c.Control(func(fd uintptr) {
							// For Linux/Entware, we use SO_BINDTODEVICE to force traffic through the interface.
							// Note: This requires root privileges (which the service usually has).
							syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, ifaceName)
						})
					},
				}
				return d.DialContext(ctx, network, addr)
			},
		},
		Timeout: 20 * time.Second,
	}

	var lastErr error
	for _, service := range services {
		resp, err := client.Get(service)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}

		ip := strings.TrimSpace(string(body))
		// Basic validation to ensure we got something looking like an IP
		if net.ParseIP(ip) == nil {
			lastErr = fmt.Errorf("invalid IP response from %s: %s", service, ip)
			continue
		}

		return ip, nil
	}

	return "", fmt.Errorf("failed to check external IP via %s: %v", ifaceName, lastErr)
}

//go:build !linux

package network

import "fmt"

func CheckExternalIP(ifaceName string) (string, error) {
	return "", fmt.Errorf("external IP check is only supported on Linux")
}

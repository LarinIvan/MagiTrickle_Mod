//go:build linux || darwin
// +build linux darwin

package updater

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"github.com/rs/zerolog/log"
)

// CheckForUpdates checks if a new version of magitrickle_mod is available.
// It returns the new version string if an update is found, or an empty string otherwise.
func CheckForUpdates() (string, error) {
	// Step 0: Refresh repo config (Optional but recommended by user)
	// We run this to ensure the repo URL is up to date (e.g. if we changed release structure)
	log.Info().Msg("Running add_repo.sh to ensure repository is valid...")
	// Note: We use -O- to output to stdout and pine to sh.
	// We use the develop branch as requested, or main/master if preferred. Using develop as per user request.
	refreshCmd := exec.Command("sh", "-c", "wget -O- https://raw.githubusercontent.com/LarinIvan/MagiTrickle_Mod/develop/add_repo.sh | sh")
	if out, err := refreshCmd.CombinedOutput(); err != nil {
		log.Warn().Err(err).Str("output", string(out)).Msg("Failed to refresh repository configuration (add_repo.sh), continuing anyway")
		// We continue, because maybe the repo is already fine suitable
	}

	// Step 1: Update opkg lists
	log.Info().Msg("Running opkg update...")
	if out, err := exec.Command("opkg", "update").CombinedOutput(); err != nil {
		return "", fmt.Errorf("opkg update failed: %w (output: %s)", err, string(out))
	}

	// Step 2: List upgradable packages
	log.Info().Msg("Checking for upgradable packages...")
	cmd := exec.Command("opkg", "list-upgradable")
	var out bytes.Buffer
	cmd.Stdout = &out
	// We ignore stderr or exit codes because list-upgradable might generally exit 0 even if empty
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("opkg list-upgradable failed: %w", err)
	}

	// Step 3: Parse output in Go
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := scanner.Text()
		// Format is typically: package_name - old_version - new_version
		// e.g.: magitrickle_mod - v0.3.0-(mod-0.0.6)-1 - v0.3.0-(mod-0.0.7)-1
		// We use " - " as delimiter to avoid splitting on hyphens inside the version string.
		if strings.HasPrefix(line, "magitrickle_mod") {
			parts := strings.Split(line, " - ")
			if len(parts) >= 3 {
				newVer := strings.TrimSpace(parts[2])
				// Strip opkg revision suffix (e.g. "-1") if present
				if idx := strings.LastIndex(newVer, "-"); idx != -1 {
					// Check if correctly formatted (e.g. ends with digit)
					// Simple approach: if it looks like -1, -2 etc, strip it.
					// Since our versions are complex v0.3.0-(mod-0.0.9), we need to be careful.
					// Our version tag doesn't end in -Number, but opkg appends it.
					// So stripping the last -N segment is safe for our naming scheme.
					newVer = newVer[:idx]
				}
				log.Info().Str("new_version", newVer).Msg("Found update for magitrickle_mod")
				return newVer, nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to scan opkg output: %w", err)
	}

	log.Info().Msg("No updates found for magitrickle_mod")
	return "", nil
}

// RunUpdate triggers the background update process.
// It executes a detached shell script that stops the service, updates, and restarts it.
func RunUpdate() error {
	log.Info().Msg("Initiating background update process...")

	// Construct the command
	// We use nohup and & to detach.
	// We allow a sleep to ensure the HTTP response returns before we kill the server.
	// Added extra sleeps and split commands for robustness.
	// Note: We use 'opkg upgrade' specifically.
	// Restart logic: sleep 3s, stop, update, sleep 5s, start.
	// Then try 5 times to start if not running ("alive" check).
	// Then monitor for 30s (stabilization) to restart if it crashes.
	script := "sleep 3; /opt/etc/init.d/S99magitrickle stop; opkg update; opkg upgrade magitrickle_mod; sleep 5; /opt/etc/init.d/S99magitrickle start; for i in $(seq 1 5); do sleep 2; if /opt/etc/init.d/S99magitrickle status | grep -q \"alive\"; then break; fi; /opt/etc/init.d/S99magitrickle start; done; for i in $(seq 1 6); do sleep 5; if ! /opt/etc/init.d/S99magitrickle status | grep -q \"alive\"; then /opt/etc/init.d/S99magitrickle start; fi; done"

	// We pass the entire command string to sh -c
	fullScript := fmt.Sprintf("%s > /dev/null 2>&1 &", script)
	cmd := exec.Command("sh", "-c", fullScript)

	// Detach process group if possible (SysProcAttr) to ensure it survives parent death
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start background update: %w", err)
	}

	log.Info().Msg("Background update command started successfully")
	return nil
}

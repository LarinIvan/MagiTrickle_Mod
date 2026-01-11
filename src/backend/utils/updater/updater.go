//go:build linux || darwin
// +build linux darwin

package updater

import (
	"fmt"
	"magitrickle/constant"
	"os/exec"
	"strings"
	"syscall"

	"github.com/rs/zerolog/log"
)

// CheckForUpdates checks if a new version of magitrickle_mod is available.
// It returns the new version string if an update is found, or an empty string otherwise.
// Uses GitHub API to get latest release tag - works for both OpenWRT and Entware.
func CheckForUpdates() (string, error) {
	log.Info().Msg("Checking for updates...")

	// Step 1: Get current installed version
	versionCmd := exec.Command("sh", "-c", "opkg list-installed magitrickle_mod | awk '{print $3}'")
	versionOut, err := versionCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get installed package version: %w", err)
	}

	currentVersion := strings.TrimSpace(string(versionOut))
	if currentVersion == "" {
		return "", fmt.Errorf("could not determine current version (package not installed?)")
	}

	log.Info().Str("current_version", currentVersion).Msg("Current installed version")

	// Step 2: Get latest version from GitHub API
	log.Info().Msg("Fetching latest version from GitHub API...")

	apiCmd := exec.Command("sh", "-c",
		"wget -qO- https://api.github.com/repos/LarinIvan/MagiTrickle_Mod/releases/latest | grep '\"tag_name\"' | head -n1 | cut -d'\"' -f4")
	apiOut, err := apiCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest version from GitHub API: %w", err)
	}

	latestVersion := strings.TrimSpace(string(apiOut))
	if latestVersion == "" {
		return "", fmt.Errorf("could not get latest version from GitHub API (empty response)")
	}

	log.Info().Str("latest_version", latestVersion).Msg("Found latest version in repository")

	// Step 3: Strip opkg revision suffix from current version (e.g. "-1")
	// Latest version from GitHub tag doesn't have this suffix
	currentVersionStripped := currentVersion
	if idx := strings.LastIndex(currentVersion, "-"); idx != -1 {
		// Check if it's a numeric suffix (opkg revision)
		suffix := currentVersion[idx+1:]
		if _, err := fmt.Sscanf(suffix, "%d", new(int)); err == nil {
			currentVersionStripped = currentVersion[:idx]
		}
	}

	// Step 4: Compare versions
	if latestVersion != currentVersionStripped {
		log.Info().
			Str("current", currentVersionStripped).
			Str("latest", latestVersion).
			Msg("Update available!")
		return latestVersion, nil
	}

	log.Info().Msg("No updates found (already on latest version)")
	return "", nil
}

// RunUpdate triggers the background update process.
// It executes a detached shell script that stops the service, updates, and restarts it.
func RunUpdate() error {
	log.Info().Msg("Initiating background update process...")

	initScript := constant.InitScriptPath
	var updateCmd string

	// For OpenWRT: use direct package URL installation
	// For Entware: update repo config inline, then use standard opkg upgrade
	if constant.PackageURLTemplate != "" {
		// OpenWRT path: detect architecture and construct direct URL
		log.Info().Msg("Detecting architecture for direct package download...")

		// Detect architecture using opkg print-architecture
		// Filter out 'all' and 'noarch', sort by priority, take highest
		archCmd := exec.Command("sh", "-c",
			"opkg print-architecture | awk '{if (NF==3) print $2, $3; else print $1, $2}' | grep -v -E '^(all|noarch)' | sort -n -k 2 | tail -n 1 | awk '{print $1}'")
		archOut, err := archCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to detect architecture: %w", err)
		}

		arch := strings.TrimSpace(string(archOut))
		if arch == "" {
			return fmt.Errorf("could not detect architecture (empty output from opkg)")
		}

		log.Info().Str("architecture", arch).Msg("Detected architecture")

		// Construct package URL
		packageURL := fmt.Sprintf(constant.PackageURLTemplate, arch)
		log.Info().Str("url", packageURL).Msg("Package URL constructed")

		// Build update command with direct URL installation
		// Use --force-reinstall to ensure update even if same version
		updateCmd = fmt.Sprintf("opkg install '%s' --force-checksum --force-reinstall", packageURL)
	} else {
		// Entware path: update repo config inline, then standard opkg upgrade
		// This updates only the repository configuration without running full installation script
		updateRepoCmd := fmt.Sprintf("echo 'src/gz magitrickle_mod %s' > %s", constant.RepoURL, constant.RepoConfPath)

		upgradeArgs := ""
		if constant.OpkgUpgradeArgs != "" {
			upgradeArgs = " " + constant.OpkgUpgradeArgs
		}
		updateCmd = fmt.Sprintf("%s; opkg update; opkg upgrade magitrickle_mod%s", updateRepoCmd, upgradeArgs)
	}

	// Construct the full command sequence
	// Sequence: sleep 3s (allow HTTP response) → stop service → update → sleep 5s → start service
	// Then retry start 5 times if not alive, then monitor for 30s
	cmdString := fmt.Sprintf(
		"sleep 1; %s stop; %s; sleep 2; %s start; for i in $(seq 1 5); do sleep 2; if %s status | grep -q \"alive\"; then break; fi; %s start; done; for i in $(seq 1 6); do sleep 5; if ! %s status | grep -q \"alive\"; then %s start; fi; done",
		initScript, updateCmd, initScript, initScript, initScript, initScript, initScript,
	)

	// Run in background, detached
	fullScript := fmt.Sprintf("%s > /dev/null 2>&1 &", cmdString)
	cmd := exec.Command("sh", "-c", fullScript)

	// Detach process group to ensure it survives parent death
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start background update: %w", err)
	}

	log.Info().Msg("Background update command started successfully")
	return nil
}

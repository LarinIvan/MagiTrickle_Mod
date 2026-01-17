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

func CheckForUpdates() (string, error) {
	log.Info().Msg("Checking for updates...")

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
	log.Info().Msg("Fetching latest version from GitHub API...")

	apiCmd := exec.Command("sh", "-c",
		`wget -qO- -U "MagiTrickle" https://api.github.com/repos/LarinIvan/MagiTrickle_Mod/releases/latest | awk -F'"' '{for(i=1;i<=NF;i++)if($i=="tag_name"){print $(i+2);exit}}'`)
	apiOut, err := apiCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest version from GitHub API: %w", err)
	}

	latestVersion := strings.TrimSpace(string(apiOut))
	if latestVersion == "" {
		return "", fmt.Errorf("could not get latest version from GitHub API (empty response)")
	}

	log.Info().Str("latest_version", latestVersion).Msg("Found latest version in repository")

	currentVersionStripped := currentVersion
	if idx := strings.LastIndex(currentVersion, "-"); idx != -1 {
		suffix := currentVersion[idx+1:]
		if _, err := fmt.Sscanf(suffix, "%d", new(int)); err == nil {
			currentVersionStripped = currentVersion[:idx]
		}
	}

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

func RunUpdate() error {
	log.Info().Msg(`Initiating background update process...`)

	initScript := constant.InitScriptPath

	updateCmd := `wget -qO- https://raw.githubusercontent.com/LarinIvan/MagiTrickle_Mod/develop/add_repo.sh | sh`

	// sleep 1s (allow HTTP response) → stop service → update → sleep 2s → start service
	// Then retry start 5 times if not alive, then monitor for 30s
	cmdString := fmt.Sprintf(
		`sleep 1; %s stop; %s; sleep 2; %s start; for i in $(seq 1 5); do sleep 2; if %s status | grep -q "alive"; then break; fi; %s start; done; for i in $(seq 1 6); do sleep 5; if ! %s status | grep -q "alive"; then %s start; fi; done`,
		initScript, updateCmd, initScript, initScript, initScript, initScript, initScript,
	)

	// Run in background, detached
	fullScript := fmt.Sprintf(`%s > /dev/null 2>&1 &`, cmdString)
	cmd := exec.Command(`sh`, `-c`, fullScript)

	// Detach process group to ensure it survives parent death
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf(`failed to start background update: %w`, err)
	}

	log.Info().Msg(`Background update command started successfully`)
	return nil
}

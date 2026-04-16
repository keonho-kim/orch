package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func LooksLikeRepoRoot(path string) bool {
	markers := []string{
		filepath.Join(path, settingsFileName),
		filepath.Join(path, legacySettingsName),
		filepath.Join(path, ".git"),
		filepath.Join(path, "go.mod"),
	}
	for _, marker := range markers {
		if fileExists(marker) || dirExists(marker) {
			return true
		}
	}
	return false
}

func managedSettingsPath() string {
	if override := strings.TrimSpace(os.Getenv("ORCH_MANAGED_SETTINGS")); override != "" {
		return override
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join("/Library/Application Support", "orch", "managed-settings.json")
	case "windows":
		programData := strings.TrimSpace(os.Getenv("ProgramData"))
		if programData == "" {
			programData = filepath.Join(os.Getenv("SystemDrive")+"\\", "ProgramData")
		}
		return filepath.Join(programData, "orch", "managed-settings.json")
	default:
		return filepath.Join("/etc", "orch", "managed-settings.json")
	}
}

func DiscoverRepoRoot(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve start path: %w", err)
	}

	info, err := os.Stat(current)
	if err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}

	for {
		if LooksLikeRepoRoot(current) {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", nil
		}
		current = parent
	}
}

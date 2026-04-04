package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func ExistingLegacyConfigFiles(paths Paths) []string {
	files := []string{
		paths.LegacyProjectSettingsFile,
		paths.LegacyUserSettingsFile,
		paths.LegacyLocalSettingsFile,
		paths.LegacyManagedSettingsFile,
	}
	result := make([]string, 0, len(files))
	for _, path := range files {
		if path == "" || path == paths.ConfigFile {
			continue
		}
		if fileExists(path) {
			result = append(result, path)
		}
	}
	return result
}

func managedSettingsPath() string {
	if override := strings.TrimSpace(os.Getenv("ORCH_MANAGED_SETTINGS")); override != "" {
		return override
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join("/Library/Application Support", appDirName, "managed-settings.json")
	case "windows":
		programData := strings.TrimSpace(os.Getenv("ProgramData"))
		if programData == "" {
			programData = filepath.Join(os.Getenv("SystemDrive")+"\\", "ProgramData")
		}
		return filepath.Join(programData, appDirName, "managed-settings.json")
	default:
		return filepath.Join("/etc", appDirName, "managed-settings.json")
	}
}

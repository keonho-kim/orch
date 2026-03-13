package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"orch/domain"
)

const (
	runtimeAssetDirName = "runtime-asset"
	bootstrapDirName    = "bootstrap"
	settingsFileName    = "orch.settings.json"
	testWorkspaceName   = "test-workspace"
	appDirName          = "orch"
	dbFileName          = "state.db"
)

type Paths struct {
	RepoRoot        string
	RuntimeAssetDir string
	BootstrapAssets string
	SettingsFile    string
	UserConfigDir   string
	DBPath          string
	TestWorkspace   string
}

func ResolvePaths(repoRoot string) (Paths, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user config dir: %w", err)
	}

	userConfigDir := filepath.Join(configDir, appDirName)
	return Paths{
		RepoRoot:        repoRoot,
		RuntimeAssetDir: filepath.Join(repoRoot, runtimeAssetDirName),
		BootstrapAssets: filepath.Join(repoRoot, runtimeAssetDirName, bootstrapDirName),
		SettingsFile:    filepath.Join(repoRoot, settingsFileName),
		UserConfigDir:   userConfigDir,
		DBPath:          filepath.Join(userConfigDir, dbFileName),
		TestWorkspace:   filepath.Join(repoRoot, testWorkspaceName),
	}, nil
}

func EnsureRuntimePaths(paths Paths) error {
	for _, path := range []string{paths.RuntimeAssetDir, paths.UserConfigDir, paths.TestWorkspace} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", path, err)
		}
	}

	return EnsureSettingsFile(paths)
}

func EnsureSettingsFile(paths Paths) error {
	if _, err := os.Stat(paths.SettingsFile); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat settings file: %w", err)
	}

	return SaveSettings(paths, defaultSettings())
}

func LoadSettings(paths Paths) (domain.Settings, error) {
	data, err := os.ReadFile(paths.SettingsFile)
	if err != nil {
		return domain.Settings{}, fmt.Errorf("read settings file: %w", err)
	}

	var settings domain.Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return domain.Settings{}, fmt.Errorf("parse settings file: %w", err)
	}
	settings.Normalize()

	if settings.DefaultProvider != "" {
		if _, err := domain.ParseProvider(settings.DefaultProvider.String()); err != nil {
			return domain.Settings{}, fmt.Errorf("parse default provider: %w", err)
		}
	}

	return settings, nil
}

func SaveSettings(paths Paths, settings domain.Settings) error {
	settings.Normalize()

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(paths.SettingsFile, data, 0o644); err != nil {
		return fmt.Errorf("write settings file: %w", err)
	}

	return nil
}

func defaultSettings() domain.Settings {
	settings := domain.Settings{}
	settings.Normalize()
	return settings
}

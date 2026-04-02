package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/keonho-kim/orch/domain"
)

const (
	runtimeAssetDirName = "runtime-asset"
	bootstrapDirName    = "bootstrap"
	settingsFileName    = "orch.settings.json"
	testWorkspaceName   = "test-workspace"
	localStateDirName   = ".orch"
	sessionsDirName     = "sessions"
	appDirName          = "orch"
	dbFileName          = "state.db"
)

type Paths struct {
	RepoRoot            string
	RuntimeAssetDir     string
	BootstrapAssets     string
	SettingsFile        string
	ProjectSettingsFile string
	UserSettingsFile    string
	LocalSettingsFile   string
	ManagedSettingsFile string
	UserConfigDir       string
	DBPath              string
	TestWorkspace       string
	LocalStateDir       string
	APIDir              string
	SessionsDir         string
}

func ResolvePaths(repoRoot string) (Paths, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user config dir: %w", err)
	}

	userConfigDir := filepath.Join(configDir, appDirName)
	return Paths{
		RepoRoot:            repoRoot,
		RuntimeAssetDir:     filepath.Join(repoRoot, runtimeAssetDirName),
		BootstrapAssets:     filepath.Join(repoRoot, runtimeAssetDirName, bootstrapDirName),
		SettingsFile:        filepath.Join(repoRoot, settingsFileName),
		ProjectSettingsFile: filepath.Join(repoRoot, settingsFileName),
		UserSettingsFile:    filepath.Join(userConfigDir, "settings.json"),
		LocalSettingsFile:   filepath.Join(repoRoot, localStateDirName, "settings.local.json"),
		ManagedSettingsFile: managedSettingsPath(),
		UserConfigDir:       userConfigDir,
		DBPath:              filepath.Join(userConfigDir, dbFileName),
		TestWorkspace:       filepath.Join(repoRoot, testWorkspaceName),
		LocalStateDir:       filepath.Join(repoRoot, localStateDirName),
		APIDir:              filepath.Join(repoRoot, localStateDirName, "api"),
		SessionsDir:         filepath.Join(repoRoot, localStateDirName, sessionsDirName),
	}, nil
}

func EnsureRuntimePaths(paths Paths) error {
	for _, path := range []string{paths.RuntimeAssetDir, paths.UserConfigDir, paths.TestWorkspace, paths.LocalStateDir, paths.APIDir, paths.SessionsDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", path, err)
		}
	}

	return nil
}

func EnsureSettingsFile(paths Paths) error {
	if _, err := os.Stat(paths.ProjectSettingsFile); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat settings file: %w", err)
	}

	return SaveSettings(paths, defaultSettings())
}

func LoadSettings(paths Paths) (domain.Settings, error) {
	resolved, err := LoadResolvedSettings(paths)
	if err != nil {
		return domain.Settings{}, err
	}
	return resolved.Effective, nil
}

func SaveSettings(paths Paths, settings domain.Settings) error {
	return SaveScopeSettings(paths, ScopeProject, ScopeSettingsFromDomainSettings(settings))
}

func defaultSettings() domain.Settings {
	settings := domain.Settings{}
	settings.Normalize()
	return settings
}

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
	settingsFileName    = "orch.toml"
	testWorkspaceName   = "test-workspace"
	localStateDirName   = ".orch"
	sessionsDirName     = "sessions"
	appDirName          = "orch"
	dbFileName          = "state.db"
)

type Paths struct {
	RepoRoot                  string
	RuntimeAssetDir           string
	BootstrapAssets           string
	ConfigFile                string
	LegacyProjectSettingsFile string
	LegacyUserSettingsFile    string
	LegacyLocalSettingsFile   string
	LegacyManagedSettingsFile string
	UserConfigDir             string
	DBPath                    string
	TestWorkspace             string
	LocalStateDir             string
	APIDir                    string
	SessionsDir               string
}

func ResolvePaths(repoRoot string) (Paths, error) {
	return ResolvePathsWithConfigFile(repoRoot, "")
}

func ResolvePathsWithConfigFile(repoRoot string, configFile string) (Paths, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user config dir: %w", err)
	}

	userConfigDir := filepath.Join(configDir, appDirName)
	resolvedConfigFile := filepath.Join(repoRoot, settingsFileName)
	if configFile != "" {
		if filepath.IsAbs(configFile) {
			resolvedConfigFile = filepath.Clean(configFile)
		} else {
			resolvedConfigFile = filepath.Join(repoRoot, configFile)
		}
	}

	return Paths{
		RepoRoot:                  repoRoot,
		RuntimeAssetDir:           filepath.Join(repoRoot, runtimeAssetDirName),
		BootstrapAssets:           filepath.Join(repoRoot, runtimeAssetDirName, bootstrapDirName),
		ConfigFile:                resolvedConfigFile,
		LegacyProjectSettingsFile: filepath.Join(repoRoot, "orch.settings.json"),
		LegacyUserSettingsFile:    filepath.Join(userConfigDir, "settings.json"),
		LegacyLocalSettingsFile:   filepath.Join(repoRoot, localStateDirName, "settings.local.json"),
		LegacyManagedSettingsFile: managedSettingsPath(),
		UserConfigDir:             userConfigDir,
		DBPath:                    filepath.Join(userConfigDir, dbFileName),
		TestWorkspace:             filepath.Join(repoRoot, testWorkspaceName),
		LocalStateDir:             filepath.Join(repoRoot, localStateDirName),
		APIDir:                    filepath.Join(repoRoot, localStateDirName, "api"),
		SessionsDir:               filepath.Join(repoRoot, localStateDirName, sessionsDirName),
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

func LoadSettings(paths Paths) (domain.Settings, error) {
	state, err := LoadConfigState(paths)
	if err != nil {
		return domain.Settings{}, err
	}
	return state.Settings, nil
}

func SaveSettings(paths Paths, settings domain.Settings) error {
	return SaveDocument(paths, DocumentFromSettings(settings))
}

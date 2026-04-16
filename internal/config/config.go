package config

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/keonho-kim/orch/internal/assets"
	"github.com/keonho-kim/orch/domain"
)

const (
<<<<<<< HEAD
	runtimeAssetDirName = "runtime-asset"
	bootstrapDirName    = "bootstrap"
	settingsFileName    = "orch.toml"
	testWorkspaceName   = "test-workspace"
	localStateDirName   = ".orch"
=======
	settingsFileName    = "orch.env.toml"
	legacySettingsName  = "orch.settings.json"
	legacyLocalStateDir = ".orch"
>>>>>>> cef7a8c (update)
	sessionsDirName     = "sessions"
	appDirName          = ".orch"
	dbFileName          = "state.db"
)

type Paths struct {
<<<<<<< HEAD
	RepoRoot        string
	RuntimeAssetDir string
	BootstrapAssets string
	ConfigFile      string
	UserConfigDir   string
	DBPath          string
	TestWorkspace   string
	LocalStateDir   string
	APIDir          string
	SessionsDir     string
}

func ResolvePaths(repoRoot string) (Paths, error) {
	return ResolvePathsWithConfigFile(repoRoot, "")
}

func ResolvePathsWithConfigFile(repoRoot string, configFile string) (Paths, error) {
=======
	RepoRoot               string
	ORCHHome               string
	RuntimeAssetDir        string
	AssetsRoot             string
	BootstrapAssets        string
	OTToolsAssets          string
	SettingsFile           string
	ProjectSettingsFile    string
	GlobalSettingsFile     string
	UserSettingsFile       string
	UserConfigDir          string
	LocalSettingsFile      string
	ManagedSettingsFile    string
	LegacyProjectSettings  string
	LegacyLocalSettings    string
	LegacyUserSettingsFile string
	LegacyManagedSettings  string
	DBPath                 string
	LogsDir                string
	RuntimeBinDir          string
	WorkspacesDir          string
	WorkspaceID            string
	TestWorkspace          string
	LocalStateDir          string
	APIDir                 string
	SessionsDir            string
}

func ResolvePaths(repoRoot string) (Paths, error) {
	orchHome, err := ResolveORCHHome()
	if err != nil {
		return Paths{}, err
	}

	absoluteRepoRoot := ""
	if strings.TrimSpace(repoRoot) != "" {
		absoluteRepoRoot, err = filepath.Abs(repoRoot)
		if err != nil {
			return Paths{}, fmt.Errorf("resolve repo root: %w", err)
		}
	}

	assetsRoot := assets.VersionedRoot(orchHome)
	workspaceID := workspaceID(absoluteRepoRoot)
	workspaceStateDir := ""
	workspaceRuntimeDir := ""
	apiDir := ""
	sessionsDir := ""
	projectSettingsFile := ""
	legacyProjectSettings := ""
	legacyLocalSettings := ""
	if absoluteRepoRoot != "" {
		workspaceStateDir = filepath.Join(orchHome, "workspaces", workspaceID)
		workspaceRuntimeDir = filepath.Join(workspaceStateDir, "runtime")
		apiDir = filepath.Join(workspaceStateDir, "api")
		sessionsDir = filepath.Join(workspaceStateDir, sessionsDirName)
		projectSettingsFile = filepath.Join(absoluteRepoRoot, settingsFileName)
		legacyProjectSettings = filepath.Join(absoluteRepoRoot, legacySettingsName)
		legacyLocalSettings = filepath.Join(absoluteRepoRoot, legacyLocalStateDir, "settings.local.json")
	}

>>>>>>> cef7a8c (update)
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user config dir: %w", err)
	}
<<<<<<< HEAD

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
		RepoRoot:        repoRoot,
		RuntimeAssetDir: filepath.Join(repoRoot, runtimeAssetDirName),
		BootstrapAssets: filepath.Join(repoRoot, runtimeAssetDirName, bootstrapDirName),
		ConfigFile:      resolvedConfigFile,
		UserConfigDir:   userConfigDir,
		DBPath:          filepath.Join(userConfigDir, dbFileName),
		TestWorkspace:   filepath.Join(repoRoot, testWorkspaceName),
		LocalStateDir:   filepath.Join(repoRoot, localStateDirName),
		APIDir:          filepath.Join(repoRoot, localStateDirName, "api"),
		SessionsDir:     filepath.Join(repoRoot, localStateDirName, sessionsDirName),
=======
	legacyUserConfigDir := filepath.Join(configDir, "orch")
	return Paths{
		RepoRoot:               absoluteRepoRoot,
		ORCHHome:               orchHome,
		RuntimeAssetDir:        assetsRoot,
		AssetsRoot:             assetsRoot,
		BootstrapAssets:        filepath.Join(assetsRoot, "bootstrap"),
		OTToolsAssets:          filepath.Join(assetsRoot, "tools", "ot"),
		SettingsFile:           projectSettingsFile,
		ProjectSettingsFile:    projectSettingsFile,
		GlobalSettingsFile:     filepath.Join(orchHome, settingsFileName),
		UserSettingsFile:       filepath.Join(orchHome, settingsFileName),
		UserConfigDir:          orchHome,
		LocalSettingsFile:      projectSettingsFile,
		ManagedSettingsFile:    filepath.Join(orchHome, settingsFileName),
		LegacyProjectSettings:  legacyProjectSettings,
		LegacyLocalSettings:    legacyLocalSettings,
		LegacyUserSettingsFile: filepath.Join(legacyUserConfigDir, "settings.json"),
		LegacyManagedSettings:  managedSettingsPath(),
		DBPath:                 filepath.Join(orchHome, dbFileName),
		LogsDir:                filepath.Join(orchHome, "logs"),
		RuntimeBinDir:          filepath.Join(orchHome, "runtime", "bin"),
		WorkspacesDir:          filepath.Join(orchHome, "workspaces"),
		WorkspaceID:            workspaceID,
		TestWorkspace:          workspaceRuntimeDir,
		LocalStateDir:          workspaceStateDir,
		APIDir:                 apiDir,
		SessionsDir:            sessionsDir,
>>>>>>> cef7a8c (update)
	}, nil
}

func EnsureRuntimePaths(paths Paths) error {
	if _, err := assets.EnsureMaterialized(paths.ORCHHome); err != nil {
		return err
	}

	for _, path := range []string{paths.ORCHHome, paths.LogsDir, paths.RuntimeBinDir, paths.WorkspacesDir, paths.TestWorkspace, paths.LocalStateDir, paths.APIDir, paths.SessionsDir} {
		if strings.TrimSpace(path) == "" {
			continue
		}
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

func ResolveORCHHome() (string, error) {
	if override := strings.TrimSpace(os.Getenv("ORCH_HOME")); override != "" {
		resolved, err := filepath.Abs(override)
		if err != nil {
			return "", fmt.Errorf("resolve ORCH_HOME: %w", err)
		}
		return resolved, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(homeDir, appDirName), nil
}

func workspaceID(repoRoot string) string {
	trimmed := strings.TrimSpace(repoRoot)
	if trimmed == "" {
		return ""
	}
	hash := sha1.Sum([]byte(trimmed))
	suffix := hex.EncodeToString(hash[:])[:12]
	base := filepath.Base(trimmed)
	if base == "." || base == string(filepath.Separator) || strings.TrimSpace(base) == "" {
		base = "workspace"
	}
	return sanitizeWorkspaceComponent(base) + "-" + suffix
}

func sanitizeWorkspaceComponent(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "workspace"
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "workspace"
	}
	return result
}

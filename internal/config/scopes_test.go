package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/keonho-kim/orch/domain"
)

func TestLoadResolvedSettingsMergesScopesByPrecedenceAndTracksOrigins(t *testing.T) {
	managedPath := filepath.Join(t.TempDir(), "managed-settings.json")
	t.Setenv("ORCH_MANAGED_SETTINGS", managedPath)

	repoRoot := t.TempDir()
	paths, err := ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}

	userSettings := ScopeSettings{}
	mustSetKey(t, &userSettings, KeyDefaultProvider, "ollama")
	mustSetKey(t, &userSettings, ProviderModelKey(domain.ProviderOllama), "user-model")
	if err := SaveScopeSettings(paths, ScopeUser, userSettings); err != nil {
		t.Fatalf("save user settings: %v", err)
	}

	projectSettings := ScopeSettings{}
	mustSetKey(t, &projectSettings, ProviderModelKey(domain.ProviderOllama), "project-model")
	mustSetKey(t, &projectSettings, KeyReactRalphIter, "5")
	if err := SaveScopeSettings(paths, ScopeProject, projectSettings); err != nil {
		t.Fatalf("save project settings: %v", err)
	}

	localSettings := ScopeSettings{}
	mustSetKey(t, &localSettings, ProviderModelKey(domain.ProviderOllama), "local-model")
	mustSetKey(t, &localSettings, KeyCompactThresholdK, "150")
	if err := SaveScopeSettings(paths, ScopeLocal, localSettings); err != nil {
		t.Fatalf("save local settings: %v", err)
	}

	managedSettings := ScopeSettings{}
	mustSetKey(t, &managedSettings, ProviderModelKey(domain.ProviderOllama), "managed-model")
	mustSetKey(t, &managedSettings, KeySelfDrivingMode, "true")
	writeScopeFile(t, paths.ManagedSettingsFile, managedSettings)

	resolved, err := LoadResolvedSettings(paths)
	if err != nil {
		t.Fatalf("load resolved settings: %v", err)
	}

	if resolved.Effective.DefaultProvider != domain.ProviderOllama {
		t.Fatalf("unexpected default provider: %s", resolved.Effective.DefaultProvider)
	}
	if resolved.Effective.ConfigFor(domain.ProviderOllama).Model != "managed-model" {
		t.Fatalf("unexpected merged model: %q", resolved.Effective.ConfigFor(domain.ProviderOllama).Model)
	}
	if !resolved.Effective.SelfDrivingMode {
		t.Fatal("expected managed self-driving mode to win")
	}
	if resolved.Effective.ReactRalphIter != 5 {
		t.Fatalf("expected project react Ralph iter to win, got %d", resolved.Effective.ReactRalphIter)
	}
	if resolved.Effective.CompactThresholdK != 150 {
		t.Fatalf("expected local compact threshold to win, got %d", resolved.Effective.CompactThresholdK)
	}
	if resolved.Sources[KeyDefaultProvider].Scope != ScopeUser {
		t.Fatalf("expected user source for default provider, got %+v", resolved.Sources[KeyDefaultProvider])
	}
	if resolved.Sources[ProviderModelKey(domain.ProviderOllama)].Scope != ScopeManaged {
		t.Fatalf("expected managed source for Ollama model, got %+v", resolved.Sources[ProviderModelKey(domain.ProviderOllama)])
	}
	if resolved.Sources[KeyReactRalphIter].Scope != ScopeProject {
		t.Fatalf("expected project source for react Ralph iter, got %+v", resolved.Sources[KeyReactRalphIter])
	}
	if resolved.Sources[KeyCompactThresholdK].Scope != ScopeLocal {
		t.Fatalf("expected local source for compact threshold, got %+v", resolved.Sources[KeyCompactThresholdK])
	}
}

func TestUnsetScopeSettingsFallsBackAndRemovesEmptyFile(t *testing.T) {
	t.Setenv("ORCH_MANAGED_SETTINGS", filepath.Join(t.TempDir(), "managed-settings.json"))

	repoRoot := t.TempDir()
	paths, err := ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}

	projectSettings := ScopeSettings{}
	mustSetKey(t, &projectSettings, ProviderModelKey(domain.ProviderOllama), "project-model")
	if err := SaveScopeSettings(paths, ScopeProject, projectSettings); err != nil {
		t.Fatalf("save project settings: %v", err)
	}

	localSettings := ScopeSettings{}
	mustSetKey(t, &localSettings, ProviderModelKey(domain.ProviderOllama), "local-model")
	if err := SaveScopeSettings(paths, ScopeLocal, localSettings); err != nil {
		t.Fatalf("save local settings: %v", err)
	}

	if err := UnsetScopeSettings(paths, ScopeLocal, []SettingKey{ProviderModelKey(domain.ProviderOllama)}); err != nil {
		t.Fatalf("unset local settings: %v", err)
	}

	resolved, err := LoadResolvedSettings(paths)
	if err != nil {
		t.Fatalf("load resolved settings: %v", err)
	}
	if resolved.Effective.ConfigFor(domain.ProviderOllama).Model != "project-model" {
		t.Fatalf("expected project fallback, got %q", resolved.Effective.ConfigFor(domain.ProviderOllama).Model)
	}
	if _, err := os.Stat(paths.LocalSettingsFile); !os.IsNotExist(err) {
		t.Fatalf("expected local settings file to be removed, stat err=%v", err)
	}
}

func TestSaveScopeSettingsLocalAddsGitExcludeEntry(t *testing.T) {
	t.Setenv("ORCH_MANAGED_SETTINGS", filepath.Join(t.TempDir(), "managed-settings.json"))

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("create git dir: %v", err)
	}

	paths, err := ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}

	localSettings := ScopeSettings{}
	mustSetKey(t, &localSettings, ProviderModelKey(domain.ProviderOllama), "local-model")
	if err := SaveScopeSettings(paths, ScopeLocal, localSettings); err != nil {
		t.Fatalf("save local settings: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoRoot, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("read git exclude: %v", err)
	}
	if string(data) != ".orch/settings.local.json\n" {
		t.Fatalf("unexpected git exclude contents: %q", string(data))
	}
}

func TestLoadResolvedSettingsToleratesMissingScopeFiles(t *testing.T) {
	t.Setenv("ORCH_MANAGED_SETTINGS", filepath.Join(t.TempDir(), "managed-settings.json"))

	repoRoot := t.TempDir()
	paths, err := ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}

	resolved, err := LoadResolvedSettings(paths)
	if err != nil {
		t.Fatalf("load resolved settings: %v", err)
	}
	if resolved.Effective.ConfigFor(domain.ProviderOllama).BaseURL != "http://localhost:11434/v1" {
		t.Fatalf("expected builtin Ollama base URL, got %q", resolved.Effective.ConfigFor(domain.ProviderOllama).BaseURL)
	}
	if resolved.Sources[ProviderBaseURLKey(domain.ProviderOllama)].Scope != ScopeBuiltin {
		t.Fatalf("expected builtin source, got %+v", resolved.Sources[ProviderBaseURLKey(domain.ProviderOllama)])
	}
}

func TestLooksLikeRepoRootDoesNotRequireProjectSettingsFile(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	bootstrapDir := filepath.Join(repoRoot, runtimeAssetDirName, bootstrapDirName)
	if err := os.MkdirAll(bootstrapDir, 0o755); err != nil {
		t.Fatalf("create bootstrap dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bootstrapDir, "AGENTS.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "go.mod"), []byte("module example.com/test\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	if !LooksLikeRepoRoot(repoRoot) {
		t.Fatal("expected repo root detection without orch.settings.json")
	}
}

func mustSetKey(t *testing.T, settings *ScopeSettings, key SettingKey, value string) {
	t.Helper()
	if err := settings.SetKey(key, value); err != nil {
		t.Fatalf("set %s: %v", key, err)
	}
}

func writeScopeFile(t *testing.T, path string, settings ScopeSettings) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create scope dir: %v", err)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatalf("marshal scope settings: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write scope settings: %v", err)
	}
}

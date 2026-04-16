package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/keonho-kim/orch/domain"
)

func TestLoadResolvedSettingsMergesGlobalAndProjectTOML(t *testing.T) {
	orchHome := filepath.Join(t.TempDir(), ".orch-home")
	t.Setenv("ORCH_HOME", orchHome)
	configHome := filepath.Join(t.TempDir(), ".config-home")
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir git dir: %v", err)
	}

	paths, err := ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}

	global := ScopeSettings{}
	mustSetKey(t, &global, KeyDefaultProvider, "ollama")
	mustSetKey(t, &global, ProviderModelKey(domain.ProviderOllama), "global-model")
	mustSetKey(t, &global, KeyReactRalphIter, "5")
	global.Env = &EnvLayersPatch{
		Global:        map[string]string{"GLOBAL_TOKEN": "global"},
		GlobalDefined: true,
	}
	if err := SaveScopeSettings(paths, ScopeGlobal, global); err != nil {
		t.Fatalf("save global settings: %v", err)
	}

	project := ScopeSettings{}
	mustSetKey(t, &project, ProviderModelKey(domain.ProviderOllama), "project-model")
	mustSetKey(t, &project, KeyCompactThresholdK, "150")
	project.Env = &EnvLayersPatch{
		Global:        map[string]string{"GLOBAL_TOKEN": "project"},
		Worker:        map[string]string{"WORKER_TOKEN": "worker"},
		GlobalDefined: true,
		WorkerDefined: true,
	}
	if err := SaveScopeSettings(paths, ScopeProject, project); err != nil {
		t.Fatalf("save project settings: %v", err)
	}

	resolved, err := LoadResolvedSettings(paths)
	if err != nil {
		t.Fatalf("load resolved settings: %v", err)
	}

	if resolved.Effective.DefaultProvider != domain.ProviderOllama {
		t.Fatalf("unexpected default provider: %s", resolved.Effective.DefaultProvider)
	}
	if resolved.Effective.ConfigFor(domain.ProviderOllama).Model != "project-model" {
		t.Fatalf("unexpected merged model: %q", resolved.Effective.ConfigFor(domain.ProviderOllama).Model)
	}
	if resolved.Effective.ReactRalphIter != 5 {
		t.Fatalf("unexpected merged react ralph iter: %d", resolved.Effective.ReactRalphIter)
	}
	if resolved.Effective.CompactThresholdK != 150 {
		t.Fatalf("unexpected merged compact threshold: %d", resolved.Effective.CompactThresholdK)
	}
	if resolved.EffectiveEnv.Global["GLOBAL_TOKEN"] != "project" {
		t.Fatalf("unexpected global env merge: %+v", resolved.EffectiveEnv.Global)
	}
	if resolved.EffectiveEnv.Worker["WORKER_TOKEN"] != "worker" {
		t.Fatalf("unexpected worker env merge: %+v", resolved.EffectiveEnv.Worker)
	}
	if resolved.Sources[ProviderModelKey(domain.ProviderOllama)].Scope != ScopeProject {
		t.Fatalf("expected project source for model, got %+v", resolved.Sources[ProviderModelKey(domain.ProviderOllama)])
	}
}

func TestUnsetScopeSettingsRemovesProjectFileWhenEmpty(t *testing.T) {
	orchHome := filepath.Join(t.TempDir(), ".orch-home")
	t.Setenv("ORCH_HOME", orchHome)
	configHome := filepath.Join(t.TempDir(), ".config-home")
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	repoRoot := t.TempDir()
	paths, err := ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}

	project := ScopeSettings{}
	mustSetKey(t, &project, ProviderModelKey(domain.ProviderOllama), "project-model")
	if err := SaveScopeSettings(paths, ScopeProject, project); err != nil {
		t.Fatalf("save project settings: %v", err)
	}
	if err := UnsetScopeSettings(paths, ScopeProject, []SettingKey{ProviderModelKey(domain.ProviderOllama)}); err != nil {
		t.Fatalf("unset project settings: %v", err)
	}
	if _, err := os.Stat(paths.ProjectSettingsFile); !os.IsNotExist(err) {
		t.Fatalf("expected project settings file removal, stat err=%v", err)
	}
}

func TestMigrateLegacyJSONCreatesGlobalAndProjectTOML(t *testing.T) {
	orchHome := filepath.Join(t.TempDir(), ".orch-home")
	t.Setenv("ORCH_HOME", orchHome)
	configHome := filepath.Join(t.TempDir(), ".config-home")
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	repoRoot := t.TempDir()
	paths, err := ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(paths.LegacyUserSettingsFile), 0o755); err != nil {
		t.Fatalf("mkdir legacy user dir: %v", err)
	}
	if err := os.WriteFile(paths.LegacyUserSettingsFile, []byte(`{"default_provider":"ollama","providers":{"ollama":{"model":"legacy-user"}}}`), 0o644); err != nil {
		t.Fatalf("write legacy user settings: %v", err)
	}
	if err := os.WriteFile(paths.LegacyProjectSettings, []byte(`{"providers":{"ollama":{"model":"legacy-project"}}}`), 0o644); err != nil {
		t.Fatalf("write legacy project settings: %v", err)
	}

	if err := MigrateLegacyJSON(paths); err != nil {
		t.Fatalf("migrate legacy json: %v", err)
	}

	resolved, err := LoadResolvedSettings(paths)
	if err != nil {
		t.Fatalf("load resolved settings after migration: %v", err)
	}
	if resolved.Effective.ConfigFor(domain.ProviderOllama).Model != "legacy-project" {
		t.Fatalf("unexpected migrated effective model: %q", resolved.Effective.ConfigFor(domain.ProviderOllama).Model)
	}
	if _, err := os.Stat(paths.GlobalSettingsFile); err != nil {
		t.Fatalf("expected global TOML file: %v", err)
	}
	if _, err := os.Stat(paths.ProjectSettingsFile); err != nil {
		t.Fatalf("expected project TOML file: %v", err)
	}
	if _, err := os.Stat(paths.LegacyUserSettingsFile); !os.IsNotExist(err) {
		t.Fatalf("expected legacy user settings to be archived, stat err=%v", err)
	}
	if _, err := os.Stat(paths.LegacyProjectSettings); !os.IsNotExist(err) {
		t.Fatalf("expected legacy project settings to be archived, stat err=%v", err)
	}
}

func TestLooksLikeRepoRootSupportsTOMLAndGitMarkers(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir git dir: %v", err)
	}
	if !LooksLikeRepoRoot(repoRoot) {
		t.Fatal("expected repo root detection from .git marker")
	}
}

func mustSetKey(t *testing.T, settings *ScopeSettings, key SettingKey, value string) {
	t.Helper()
	if err := settings.SetKey(key, value); err != nil {
		t.Fatalf("set %s: %v", key, err)
	}
}

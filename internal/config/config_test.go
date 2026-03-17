package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/keonho-kim/orch/domain"
)

func TestResolvePaths(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	paths, err := ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	if paths.TestWorkspace != filepath.Join(repoRoot, "test-workspace") {
		t.Fatalf("unexpected test workspace path: %s", paths.TestWorkspace)
	}
}

func TestSaveAndLoadSettings(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	paths, err := ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}

	settings := domain.Settings{
		DefaultProvider: domain.ProviderOllama,
		Providers: domain.ProviderCatalog{
			Ollama: domain.ProviderSettings{
				BaseURL: "http://localhost:11434/v1",
				Model:   "qwen2.5-coder",
			},
			VLLM: domain.ProviderSettings{
				BaseURL:   "http://localhost:8000/v1",
				Model:     "deepseek-coder",
				APIKeyEnv: "VLLM_API_KEY",
			},
		},
		ApprovalPolicy:    domain.ApprovalConfirmMutations,
		SelfDrivingMode:   true,
		AutoTranslate:     true,
		ReactRalphIter:    5,
		PlanRalphIter:     7,
		CompactThresholdK: 150,
	}
	if err := SaveSettings(paths, settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	loaded, err := LoadSettings(paths)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if loaded.DefaultProvider != domain.ProviderOllama {
		t.Fatalf("unexpected default provider: %s", loaded.DefaultProvider)
	}
	if loaded.ConfigFor(domain.ProviderOllama).Model != "qwen2.5-coder" {
		t.Fatalf("unexpected ollama model: %s", loaded.ConfigFor(domain.ProviderOllama).Model)
	}
	if !loaded.SelfDrivingMode {
		t.Fatal("expected self-driving mode to round-trip")
	}
	if !loaded.AutoTranslate {
		t.Fatal("expected auto-translate to round-trip")
	}
	if loaded.ReactRalphIter != 5 || loaded.PlanRalphIter != 7 {
		t.Fatalf("unexpected Ralph settings: react=%d plan=%d", loaded.ReactRalphIter, loaded.PlanRalphIter)
	}
	if loaded.CompactThresholdK != 150 {
		t.Fatalf("unexpected compact threshold: %d", loaded.CompactThresholdK)
	}
}

func TestLoadSettingsIgnoresLegacyKeys(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	paths, err := ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}

	if err := os.WriteFile(paths.SettingsFile, []byte("{\n  \"default_engine\": \"codex\"\n}\n"), 0o644); err != nil {
		t.Fatalf("write settings file: %v", err)
	}

	loaded, err := LoadSettings(paths)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if loaded.DefaultProvider != "" {
		t.Fatalf("expected no migrated provider, got %s", loaded.DefaultProvider)
	}
}

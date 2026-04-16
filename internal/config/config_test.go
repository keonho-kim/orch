package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keonho-kim/orch/domain"
)

func TestResolvePathsUsesORCHHomeAndGlobalWorkspaceState(t *testing.T) {
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
	if paths.ORCHHome != orchHome {
		t.Fatalf("unexpected ORCH home: %q", paths.ORCHHome)
	}
	if paths.GlobalSettingsFile != filepath.Join(orchHome, settingsFileName) {
		t.Fatalf("unexpected global settings file: %q", paths.GlobalSettingsFile)
	}
	if filepath.Dir(paths.TestWorkspace) != filepath.Join(orchHome, "workspaces", paths.WorkspaceID) {
		t.Fatalf("unexpected workspace runtime path: %q", paths.TestWorkspace)
	}
	if paths.SessionsDir != filepath.Join(orchHome, "workspaces", paths.WorkspaceID, "sessions") {
		t.Fatalf("unexpected sessions dir: %q", paths.SessionsDir)
	}
	if paths.ConfigFile != filepath.Join(repoRoot, "orch.toml") {
		t.Fatalf("unexpected config file path: %s", paths.ConfigFile)
	}
}

func TestSaveAndLoadSettingsRoundTripThroughProjectTOML(t *testing.T) {
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

	settings := domain.Settings{
		DefaultProvider: domain.ProviderChatGPT,
		Providers: domain.ProviderCatalog{
<<<<<<< HEAD
			Ollama: domain.ProviderSettings{
				Endpoint:  "http://localhost:11434/v1",
				Model:     "qwen2.5-coder",
				Reasoning: "high",
			},
			ChatGPT: domain.ProviderSettings{
				Endpoint:  "https://api.openai.com/v1",
				Model:     "gpt-5.3-codex",
				APIKey:    "secret-openai-key",
				Reasoning: "xhigh",
=======
			ChatGPT: domain.ProviderSettings{
				BaseURL: "https://api.openai.com/v1",
				Model:   "gpt-4.1",
				Auth: domain.ProviderAuth{
					Kind:  domain.ProviderAuthEnv,
					Env:   "OPENAI_API_KEY",
					Value: "",
				},
>>>>>>> cef7a8c (update)
			},
		},
		ApprovalPolicy:    domain.ApprovalConfirmMutations,
		SelfDrivingMode:   true,
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
	if loaded.DefaultProvider != domain.ProviderChatGPT {
		t.Fatalf("unexpected default provider: %s", loaded.DefaultProvider)
	}
<<<<<<< HEAD
	if loaded.ConfigFor(domain.ProviderOllama).Reasoning != "high" {
		t.Fatalf("unexpected ollama reasoning: %q", loaded.ConfigFor(domain.ProviderOllama).Reasoning)
	}
	if loaded.ConfigFor(domain.ProviderChatGPT).APIKey != "secret-openai-key" {
		t.Fatalf("unexpected ChatGPT API key: %q", loaded.ConfigFor(domain.ProviderChatGPT).APIKey)
=======
	if loaded.ConfigFor(domain.ProviderChatGPT).Model != "gpt-4.1" {
		t.Fatalf("unexpected model: %s", loaded.ConfigFor(domain.ProviderChatGPT).Model)
	}
	if loaded.ConfigFor(domain.ProviderChatGPT).Auth.Env != "OPENAI_API_KEY" {
		t.Fatalf("unexpected auth env: %s", loaded.ConfigFor(domain.ProviderChatGPT).Auth.Env)
>>>>>>> cef7a8c (update)
	}
	if !loaded.SelfDrivingMode {
		t.Fatal("expected self-driving mode to round-trip")
	}
}

<<<<<<< HEAD
func TestLoadDocumentReturnsDefaultDocumentWithoutConfigFile(t *testing.T) {
	setTestConfigHome(t)
=======
func TestDiscoverRepoRootFindsNearestConfigOrGitRoot(t *testing.T) {
	t.Parallel()
>>>>>>> cef7a8c (update)

	repoRoot := t.TempDir()
	nested := filepath.Join(repoRoot, "a", "b", "c")
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir git: %v", err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

<<<<<<< HEAD
	document, err := LoadDocument(paths)
	if err != nil {
		t.Fatalf("load document: %v", err)
	}
	if document.Provider != "" {
		t.Fatalf("expected default document, got %+v", document)
	}
}

func TestSaveSettingsAddsGitExcludeEntry(t *testing.T) {
	setTestConfigHome(t)

	repoRoot := t.TempDir()
	paths, err := ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git", "info"), 0o755); err != nil {
		t.Fatalf("create git info: %v", err)
	}

	if err := SaveSettings(paths, domain.Settings{}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoRoot, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("read exclude: %v", err)
	}
	if string(data) != "orch.toml\n" {
		t.Fatalf("unexpected exclude contents: %q", string(data))
	}
}

func TestLooksLikeRepoRootRequiresBootstrapAndRepoMarker(t *testing.T) {
	repoRoot := t.TempDir()
	if LooksLikeRepoRoot(repoRoot) {
		t.Fatal("expected missing bootstrap assets to fail repo root detection")
	}

	bootstrapPath := filepath.Join(repoRoot, "runtime-asset", "bootstrap", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(bootstrapPath), 0o755); err != nil {
		t.Fatalf("mkdir bootstrap: %v", err)
	}
	if err := os.WriteFile(bootstrapPath, []byte("bootstrap"), 0o644); err != nil {
		t.Fatalf("write bootstrap: %v", err)
	}
	if LooksLikeRepoRoot(repoRoot) {
		t.Fatal("expected repo root marker to be required")
	}

	if err := os.WriteFile(filepath.Join(repoRoot, "go.mod"), []byte("module example.com/test\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if !LooksLikeRepoRoot(repoRoot) {
		t.Fatal("expected go.mod marker to satisfy repo root detection")
	}
}

func TestMarshalDocumentRedactsAPIKeys(t *testing.T) {
	document := DefaultDocument()
	document.Provider = "chatgpt"
	document.Providers.ChatGPT.APIKey = "1234567890abcdefghijklmnopqrstuvwxyz"

	data, err := MarshalDocument(document, true)
	if err != nil {
		t.Fatalf("marshal document: %v", err)
	}
	output := string(data)
	if !strings.Contains(output, `api_key = "1234567890***vwxyz"`) {
		t.Fatalf("expected masked API key, got %s", output)
	}
}

func setTestConfigHome(t *testing.T) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
}
=======
	discovered, err := DiscoverRepoRoot(nested)
	if err != nil {
		t.Fatalf("discover repo root: %v", err)
	}
	if discovered != repoRoot {
		t.Fatalf("unexpected discovered repo root: %q", discovered)
	}
}
>>>>>>> cef7a8c (update)

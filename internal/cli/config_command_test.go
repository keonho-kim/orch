package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
)

func TestParseCommandConfigList(t *testing.T) {
	t.Parallel()

	command, err := parseCommand([]string{"config", "--list", "--env-file", "custom.toml", "--workspace", "/repo"})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if command.name != "config-list" {
		t.Fatalf("unexpected command: %+v", command)
	}
	if command.repoRoot != "/repo" || command.configFile != "custom.toml" {
		t.Fatalf("unexpected global flags: %+v", command)
	}
}

func TestParseCommandRejectsLegacyConfigListAlias(t *testing.T) {
	t.Parallel()

	if _, err := parseCommand([]string{"config", "list"}); err == nil {
		t.Fatal("expected legacy config list alias to fail")
	}
}

func TestParseCommandConfigSetWithUnifiedFlags(t *testing.T) {
	t.Parallel()

	command, err := parseCommand([]string{
		"config",
		"--provider", "chatgpt",
		"--model", "gpt-5.3-codex",
		"--reasoning", "xhigh",
		"--endpoint", "https://api.openai.com/v1",
		"--api-key", "secret",
		"--self-driving-mode=false",
	})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if command.name != "config-set" {
		t.Fatalf("unexpected command: %+v", command)
	}
	if command.configCommand.patch.Provider == nil || *command.configCommand.patch.Provider != "chatgpt" {
		t.Fatalf("unexpected provider patch: %+v", command.configCommand.patch)
	}
	providerPatch := command.configCommand.patch.Providers[domain.ProviderChatGPT]
	if providerPatch.Reasoning == nil || *providerPatch.Reasoning != "xhigh" {
		t.Fatalf("unexpected reasoning patch: %+v", command.configCommand.patch)
	}
}

func TestParseCommandRejectsProviderScopedFlagsWithoutProvider(t *testing.T) {
	t.Parallel()

	if _, err := parseCommand([]string{"config", "--model", "qwen3.5:35b"}); err == nil {
		t.Fatal("expected --model without --provider to fail")
	}
	if _, err := parseCommand([]string{"config", "--endpoint", "http://localhost:11434/v1"}); err == nil {
		t.Fatal("expected --endpoint without --provider to fail")
	}
}

func TestRunConfigListPrintsMaskedTOML(t *testing.T) {
	setTestConfigHome(t)

	repoRoot := t.TempDir()
	paths, err := resolveAppPaths(repoRoot, "")
	if err != nil {
		t.Fatalf("resolve app paths: %v", err)
	}
	if err := config.SaveDocument(paths, config.Document{
		Provider: "chatgpt",
		Providers: config.ProviderCatalogDocument{
			ChatGPT: config.ProviderDocument{Endpoint: "https://api.openai.com/v1", Model: "gpt-5.3-codex", APIKey: "1234567890abcdefghijklmnopqrstuvwxyz", Reasoning: "xhigh"},
		},
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	var output bytes.Buffer
	if err := runConfigList(repoRoot, "", &output); err != nil {
		t.Fatalf("run config list: %v", err)
	}
	if !strings.Contains(output.String(), `api_key = "1234567890***vwxyz"`) {
		t.Fatalf("expected masked API key, got %s", output.String())
	}
}

func TestRunConfigUpdateWritesUnifiedConfig(t *testing.T) {
	setTestConfigHome(t)

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git", "info"), 0o755); err != nil {
		t.Fatalf("create git info: %v", err)
	}

	if err := runConfigUpdate(repoRoot, "", configCommandState{
		patch: ollamaPatch(),
	}); err != nil {
		t.Fatalf("run config update: %v", err)
	}

	paths, err := resolveAppPaths(repoRoot, "")
	if err != nil {
		t.Fatalf("resolve app paths: %v", err)
	}
	settings, err := config.LoadSettings(paths)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if settings.DefaultProvider != "ollama" {
		t.Fatalf("unexpected default provider: %s", settings.DefaultProvider)
	}
	if settings.ConfigFor("ollama").Reasoning != "high" {
		t.Fatalf("unexpected ollama reasoning: %q", settings.ConfigFor("ollama").Reasoning)
	}
}

func ollamaPatch() config.DocumentPatch {
	provider := domain.ProviderOllama.String()
	model := "qwen3.5:35b"
	endpoint := "http://localhost:11434/v1"
	reasoning := "high"

	patch := config.DocumentPatch{Provider: &provider}
	patch.SetProviderPatch(domain.ProviderOllama, config.ProviderPatch{
		Model:     &model,
		Endpoint:  &endpoint,
		Reasoning: &reasoning,
	})
	return patch
}

func setTestConfigHome(t *testing.T) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
}

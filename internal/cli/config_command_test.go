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

<<<<<<< HEAD
	command, err := parseCommand([]string{"config", "--list", "--env-file", "custom.toml", "--workspace", "/repo"})
=======
	command, err := parseCommand([]string{"config", "--list"})
>>>>>>> cef7a8c (update)
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if command.name != "config-list" {
		t.Fatalf("unexpected command: %+v", command)
	}
<<<<<<< HEAD
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
=======
	if command.configCommand.scope != config.ScopeEffective {
		t.Fatalf("expected effective scope, got %s", command.configCommand.scope)
	}
}

func TestParseCommandConfigSetUsesGlobalAndProjectScopes(t *testing.T) {
>>>>>>> cef7a8c (update)
	t.Parallel()

	command, err := parseCommand([]string{
		"config",
<<<<<<< HEAD
		"--provider", "chatgpt",
		"--model", "gpt-5.3-codex",
		"--reasoning", "xhigh",
		"--endpoint", "https://api.openai.com/v1",
		"--api-key", "secret",
		"--self-driving-mode=false",
=======
		"--scope", "global",
		"--provider", "chatgpt",
		"--model", "gpt-4.1",
		"--chatgpt-base-url", "https://api.openai.com/v1",
		"--chatgpt-api-key-env", "OPENAI_API_KEY",
>>>>>>> cef7a8c (update)
	})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
<<<<<<< HEAD
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
=======
	if command.configCommand.scope != config.ScopeGlobal {
		t.Fatalf("expected global scope, got %s", command.configCommand.scope)
	}
	value, ok := command.configCommand.patch.ValueForKey(config.ProviderAPIKeyEnvKey(domain.ProviderChatGPT))
	if !ok || value != "OPENAI_API_KEY" {
		t.Fatalf("unexpected auth env patch: ok=%t value=%q", ok, value)
	}
}

func TestParseCommandConfigMigrate(t *testing.T) {
	t.Parallel()

	command, err := parseCommand([]string{"config", "migrate"})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if command.name != "config-migrate" {
		t.Fatalf("unexpected command: %+v", command)
	}
}

func TestRunConfigListPrintsEffectiveSettings(t *testing.T) {
	orchHome := filepath.Join(t.TempDir(), ".orch-home")
	t.Setenv("ORCH_HOME", orchHome)
	configHome := filepath.Join(t.TempDir(), ".config-home")
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
>>>>>>> cef7a8c (update)

	repoRoot := t.TempDir()
	paths, err := resolveAppPaths(repoRoot, "")
	if err != nil {
		t.Fatalf("resolve app paths: %v", err)
	}
<<<<<<< HEAD
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
=======

	global := config.ScopeSettings{}
	if err := global.SetKey(config.KeyDefaultProvider, "chatgpt"); err != nil {
		t.Fatalf("set default provider: %v", err)
	}
	if err := global.SetKey(config.ProviderModelKey(domain.ProviderChatGPT), "gpt-4.1"); err != nil {
		t.Fatalf("set model: %v", err)
	}
	if err := global.SetKey(config.ProviderAPIKeyEnvKey(domain.ProviderChatGPT), "OPENAI_API_KEY"); err != nil {
		t.Fatalf("set auth env: %v", err)
	}
	if err := config.SaveScopeSettings(paths, config.ScopeGlobal, global); err != nil {
		t.Fatalf("save global config: %v", err)
	}

	var output bytes.Buffer
	if err := runConfigList(repoRoot, configCommandState{scope: config.ScopeEffective, showOrigin: true}, &output); err != nil {
		t.Fatalf("run config list: %v", err)
	}
	rendered := output.String()
	if !strings.Contains(rendered, "orch.default_provider=chatgpt") {
		t.Fatalf("unexpected config list output: %q", rendered)
	}
	if !strings.Contains(rendered, "provider.chatgpt.auth.env=OPENAI_API_KEY") {
		t.Fatalf("expected auth env in config list, got %q", rendered)
	}
	if !strings.Contains(rendered, "origin=global:"+paths.GlobalSettingsFile) {
		t.Fatalf("expected origin info in config list, got %q", rendered)
	}
}

func TestRunConfigUpdatePersistsProjectTOML(t *testing.T) {
	orchHome := filepath.Join(t.TempDir(), ".orch-home")
	t.Setenv("ORCH_HOME", orchHome)
	configHome := filepath.Join(t.TempDir(), ".config-home")
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
>>>>>>> cef7a8c (update)

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git", "info"), 0o755); err != nil {
		t.Fatalf("create git info: %v", err)
	}

<<<<<<< HEAD
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
=======
	state := configCommandState{
		scope: config.ScopeProject,
		patch: config.ScopeSettings{},
	}
	if err := state.patch.SetKey(config.KeyDefaultProvider, "ollama"); err != nil {
		t.Fatalf("set default provider: %v", err)
	}
	if err := state.patch.SetKey(config.ProviderModelKey(domain.ProviderOllama), "qwen3.5:35b"); err != nil {
		t.Fatalf("set model: %v", err)
	}
	if err := runConfigUpdate(repoRoot, state); err != nil {
		t.Fatalf("run config update: %v", err)
	}

	loaded, err := config.LoadScopeSettings(paths, config.ScopeProject)
	if err != nil {
		t.Fatalf("load project config: %v", err)
	}
	if value, ok := loaded.ValueForKey(config.ProviderModelKey(domain.ProviderOllama)); !ok || value != "qwen3.5:35b" {
		t.Fatalf("unexpected saved model: ok=%t value=%q", ok, value)
	}
}

func TestRunConfigMigrateConvertsLegacyJSON(t *testing.T) {
	orchHome := filepath.Join(t.TempDir(), ".orch-home")
	t.Setenv("ORCH_HOME", orchHome)
	configHome := filepath.Join(t.TempDir(), ".config-home")
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	repoRoot := t.TempDir()
	paths, err := resolveAppPaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve app paths: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.LegacyUserSettingsFile), 0o755); err != nil {
		t.Fatalf("mkdir legacy user dir: %v", err)
	}
	if err := os.WriteFile(paths.LegacyUserSettingsFile, []byte(`{"default_provider":"ollama","providers":{"ollama":{"model":"legacy-model"}}}`), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	if err := runConfigMigrate(repoRoot); err != nil {
		t.Fatalf("run config migrate: %v", err)
	}
	if _, err := os.Stat(paths.GlobalSettingsFile); err != nil {
		t.Fatalf("expected global TOML file after migrate: %v", err)
	}
}
>>>>>>> cef7a8c (update)

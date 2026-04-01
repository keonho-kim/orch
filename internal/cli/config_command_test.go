package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
	sqlitestore "github.com/keonho-kim/orch/internal/store/sqlite"
)

func TestParseCommandConfigListDefaultsToEffectiveScope(t *testing.T) {
	t.Parallel()

	command, err := parseCommand([]string{"config", "--list"})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if command.name != "config-list" || command.repoRoot != "." {
		t.Fatalf("unexpected config list command: %+v", command)
	}
	if command.configCommand.scope != config.ScopeEffective {
		t.Fatalf("expected effective scope, got %s", command.configCommand.scope)
	}
	if command.configCommand.showOrigin {
		t.Fatal("did not expect show-origin to be enabled")
	}
}

func TestParseCommandConfigListSupportsScopeOriginAndWorkspace(t *testing.T) {
	t.Parallel()

	command, err := parseCommand([]string{"config", "--list", "--scope", "project", "--show-origin", "--workspace", "/repo"})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if command.name != "config-list" || command.repoRoot != "/repo" {
		t.Fatalf("unexpected config list command: %+v", command)
	}
	if command.configCommand.scope != config.ScopeProject {
		t.Fatalf("expected project scope, got %s", command.configCommand.scope)
	}
	if !command.configCommand.showOrigin {
		t.Fatal("expected show-origin to be enabled")
	}
}

func TestParseCommandConfigListAliasStillWorks(t *testing.T) {
	t.Parallel()

	command, err := parseCommand([]string{"config", "list"})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if command.name != "config-list" {
		t.Fatalf("unexpected config list alias command: %+v", command)
	}
}

func TestParseCommandConfigSetWithScopeUnsetAndCloudFlags(t *testing.T) {
	t.Parallel()

	command, err := parseCommand([]string{
		"config",
		"--scope", "local",
		"--provider", "chatgpt",
		"--model", "gpt-4.1",
		"--chatgpt-base-url", "https://api.openai.com/v1",
		"--chatgpt-api-key-env", "OPENAI_API_KEY",
		"--self-driving-mode=false",
		"--plan-ralph-iter=7",
		"--unset", "providers.vllm.model",
	})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}

	if command.name != "config-set" {
		t.Fatalf("unexpected command name: %+v", command)
	}
	if command.configCommand.scope != config.ScopeLocal {
		t.Fatalf("expected local scope, got %s", command.configCommand.scope)
	}
	value, ok := command.configCommand.patch.ValueForKey(config.KeyDefaultProvider)
	if !ok || value != "chatgpt" {
		t.Fatalf("unexpected default provider patch: ok=%t value=%q", ok, value)
	}
	value, ok = command.configCommand.patch.ValueForKey(config.ProviderModelKey(domain.ProviderChatGPT))
	if !ok || value != "gpt-4.1" {
		t.Fatalf("unexpected ChatGPT model patch: ok=%t value=%q", ok, value)
	}
	value, ok = command.configCommand.patch.ValueForKey(config.ProviderBaseURLKey(domain.ProviderChatGPT))
	if !ok || value != "https://api.openai.com/v1" {
		t.Fatalf("unexpected ChatGPT base URL patch: ok=%t value=%q", ok, value)
	}
	value, ok = command.configCommand.patch.ValueForKey(config.ProviderAPIKeyEnvKey(domain.ProviderChatGPT))
	if !ok || value != "OPENAI_API_KEY" {
		t.Fatalf("unexpected ChatGPT API key env patch: ok=%t value=%q", ok, value)
	}
	value, ok = command.configCommand.patch.ValueForKey(config.KeySelfDrivingMode)
	if !ok || value != "false" {
		t.Fatalf("unexpected self-driving patch: ok=%t value=%q", ok, value)
	}
	value, ok = command.configCommand.patch.ValueForKey(config.KeyPlanRalphIter)
	if !ok || value != "7" {
		t.Fatalf("unexpected plan Ralph patch: ok=%t value=%q", ok, value)
	}
	if len(command.configCommand.unsetKeys) != 1 || command.configCommand.unsetKeys[0] != config.ProviderModelKey(domain.ProviderVLLM) {
		t.Fatalf("unexpected unset keys: %+v", command.configCommand.unsetKeys)
	}
}

func TestParseCommandRejectsBareConfig(t *testing.T) {
	t.Parallel()

	if _, err := parseCommand([]string{"config"}); err == nil {
		t.Fatal("expected error for bare config command")
	}
}

func TestParseCommandRejectsListCombinedWithWriteFlags(t *testing.T) {
	t.Parallel()

	if _, err := parseCommand([]string{"config", "--list", "--provider=ollama"}); err == nil {
		t.Fatal("expected --list with write flags to fail")
	}
}

func TestParseCommandRejectsShowOriginWithoutList(t *testing.T) {
	t.Parallel()

	if _, err := parseCommand([]string{"config", "--show-origin", "--provider=ollama"}); err == nil {
		t.Fatal("expected --show-origin without --list to fail")
	}
}

func TestParseCommandRejectsModelWithoutProvider(t *testing.T) {
	t.Parallel()

	if _, err := parseCommand([]string{"config", "--model", "qwen3.5:35b"}); err == nil {
		t.Fatal("expected --model without --provider to fail")
	}
}

func TestParseCommandRejectsWriteToManagedScope(t *testing.T) {
	t.Parallel()

	if _, err := parseCommand([]string{"config", "--scope", "managed", "--provider=ollama", "--model=qwen3.5:35b"}); err == nil {
		t.Fatal("expected managed scope write to fail")
	}
}

func TestParseCommandRejectsUnknownConfigArgument(t *testing.T) {
	t.Parallel()

	if _, err := parseCommand([]string{"config", "set"}); err == nil {
		t.Fatal("expected unknown config argument to fail")
	}
}

func TestRunConfigListPrintsEffectiveSettingsWithOrigins(t *testing.T) {
	setTestConfigHome(t)

	repoRoot := t.TempDir()
	paths, err := resolveAppPaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve app paths: %v", err)
	}

	userSettings := config.ScopeSettings{}
	mustSetScopeValue(t, &userSettings, config.KeyDefaultProvider, "chatgpt")
	mustSetScopeValue(t, &userSettings, config.ProviderModelKey(domain.ProviderChatGPT), "gpt-4.1")
	if err := config.SaveScopeSettings(paths, config.ScopeUser, userSettings); err != nil {
		t.Fatalf("save user settings: %v", err)
	}

	projectSettings := config.ScopeSettings{}
	mustSetScopeValue(t, &projectSettings, config.ProviderBaseURLKey(domain.ProviderChatGPT), "https://api.openai.com/v1")
	mustSetScopeValue(t, &projectSettings, config.KeyCompactThresholdK, "150")
	if err := config.SaveScopeSettings(paths, config.ScopeProject, projectSettings); err != nil {
		t.Fatalf("save project settings: %v", err)
	}

	localSettings := config.ScopeSettings{}
	mustSetScopeValue(t, &localSettings, config.KeySelfDrivingMode, "true")
	if err := config.SaveScopeSettings(paths, config.ScopeLocal, localSettings); err != nil {
		t.Fatalf("save local settings: %v", err)
	}

	managedSettings := config.ScopeSettings{}
	mustSetScopeValue(t, &managedSettings, config.KeyApprovalPolicy, string(domain.ApprovalConfirmMutations))
	mustSetScopeValue(t, &managedSettings, config.ProviderAPIKeyEnvKey(domain.ProviderChatGPT), "ORG_OPENAI_KEY")
	writeManagedSettingsFile(t, paths.ManagedSettingsFile, managedSettings)

	var output bytes.Buffer
	if err := runConfigList(repoRoot, configCommandState{
		scope:      config.ScopeEffective,
		showOrigin: true,
	}, &output); err != nil {
		t.Fatalf("run config list: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != len(config.AllSettingKeys()) {
		t.Fatalf("expected %d config lines, got %d", len(config.AllSettingKeys()), len(lines))
	}
	lineMap := configLineMap(output.String())
	assertLineContains(t, lineMap["default_provider"], "default_provider=chatgpt")
	assertLineContains(t, lineMap["default_provider"], "\torigin=user:"+paths.UserSettingsFile)
	assertLineContains(t, lineMap["providers.chatgpt.base_url"], "providers.chatgpt.base_url=https://api.openai.com/v1")
	assertLineContains(t, lineMap["providers.chatgpt.base_url"], "\torigin=project:"+paths.ProjectSettingsFile)
	assertLineContains(t, lineMap["providers.chatgpt.api_key_env"], "providers.chatgpt.api_key_env=ORG_OPENAI_KEY")
	assertLineContains(t, lineMap["providers.chatgpt.api_key_env"], "\torigin=managed:"+paths.ManagedSettingsFile)
	assertLineContains(t, lineMap["self_driving_mode"], "self_driving_mode=true")
	assertLineContains(t, lineMap["self_driving_mode"], "\torigin=local:"+paths.LocalSettingsFile)
	assertLineContains(t, lineMap["providers.ollama.base_url"], "providers.ollama.base_url=http://localhost:11434/v1")
	assertLineContains(t, lineMap["providers.ollama.base_url"], "\torigin=builtin")
	assertLineContains(t, lineMap["providers.bedrock.base_url"], "providers.bedrock.base_url=")
	assertLineContains(t, lineMap["providers.bedrock.base_url"], "\torigin=builtin")
}

func TestRunConfigListPrintsRawScopeValues(t *testing.T) {
	setTestConfigHome(t)

	repoRoot := t.TempDir()
	paths, err := resolveAppPaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve app paths: %v", err)
	}

	projectSettings := config.ScopeSettings{}
	mustSetScopeValue(t, &projectSettings, config.KeyDefaultProvider, "ollama")
	mustSetScopeValue(t, &projectSettings, config.ProviderModelKey(domain.ProviderOllama), "qwen3.5:35b")
	if err := config.SaveScopeSettings(paths, config.ScopeProject, projectSettings); err != nil {
		t.Fatalf("save project settings: %v", err)
	}

	var output bytes.Buffer
	if err := runConfigList(repoRoot, configCommandState{scope: config.ScopeProject}, &output); err != nil {
		t.Fatalf("run config list: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != len(config.AllSettingKeys()) {
		t.Fatalf("expected %d config lines, got %d", len(config.AllSettingKeys()), len(lines))
	}
	lineMap := configLineMap(output.String())
	if lineMap["default_provider"] != "default_provider=ollama" {
		t.Fatalf("unexpected default provider line: %q", lineMap["default_provider"])
	}
	if lineMap["providers.ollama.model"] != "providers.ollama.model=qwen3.5:35b" {
		t.Fatalf("unexpected Ollama model line: %q", lineMap["providers.ollama.model"])
	}
	if lineMap["providers.ollama.base_url"] != "providers.ollama.base_url=" {
		t.Fatalf("expected unset base URL in project scope list, got %q", lineMap["providers.ollama.base_url"])
	}
}

func TestRunConfigUpdateAppliesProjectScopeAndPersistsDefaultProvider(t *testing.T) {
	setTestConfigHome(t)

	repoRoot := t.TempDir()
	paths, err := resolveAppPaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve app paths: %v", err)
	}

	initialProject := config.ScopeSettings{}
	mustSetScopeValue(t, &initialProject, config.KeyDefaultProvider, "vllm")
	mustSetScopeValue(t, &initialProject, config.ProviderBaseURLKey(domain.ProviderChatGPT), "https://api.openai.com/v1")
	mustSetScopeValue(t, &initialProject, config.ProviderAPIKeyEnvKey(domain.ProviderChatGPT), "OPENAI_API_KEY")
	mustSetScopeValue(t, &initialProject, config.ProviderModelKey(domain.ProviderChatGPT), "old-gpt")
	mustSetScopeValue(t, &initialProject, config.KeySelfDrivingMode, "true")
	if err := config.SaveScopeSettings(paths, config.ScopeProject, initialProject); err != nil {
		t.Fatalf("save project settings: %v", err)
	}

	patch := config.ScopeSettings{}
	mustSetScopeValue(t, &patch, config.KeyDefaultProvider, "chatgpt")
	mustSetScopeValue(t, &patch, config.ProviderModelKey(domain.ProviderChatGPT), "gpt-4.1")
	mustSetScopeValue(t, &patch, config.KeySelfDrivingMode, "false")
	if err := runConfigUpdate(repoRoot, configCommandState{
		scope: config.ScopeProject,
		patch: patch,
	}); err != nil {
		t.Fatalf("run config update: %v", err)
	}

	resolved, err := config.LoadResolvedSettings(paths)
	if err != nil {
		t.Fatalf("load resolved settings: %v", err)
	}
	if resolved.Effective.DefaultProvider != domain.ProviderChatGPT {
		t.Fatalf("unexpected default provider: %s", resolved.Effective.DefaultProvider)
	}
	chatgpt := resolved.Effective.ConfigFor(domain.ProviderChatGPT)
	if chatgpt.BaseURL != "https://api.openai.com/v1" || chatgpt.APIKeyEnv != "OPENAI_API_KEY" || chatgpt.Model != "gpt-4.1" {
		t.Fatalf("unexpected ChatGPT settings: %+v", chatgpt)
	}
	if resolved.Effective.SelfDrivingMode {
		t.Fatal("expected self-driving mode to be false")
	}

	store, err := sqlitestore.Open(paths.DBPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	stored, err := store.LoadSettings(context.Background())
	if err != nil {
		t.Fatalf("load stored settings: %v", err)
	}
	if stored.DefaultProvider != domain.ProviderChatGPT {
		t.Fatalf("unexpected stored default provider: %s", stored.DefaultProvider)
	}
}

func TestRunConfigUpdateUnsetFallsBackToLowerScope(t *testing.T) {
	setTestConfigHome(t)

	repoRoot := t.TempDir()
	paths, err := resolveAppPaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve app paths: %v", err)
	}

	projectSettings := config.ScopeSettings{}
	mustSetScopeValue(t, &projectSettings, config.ProviderModelKey(domain.ProviderOllama), "project-model")
	if err := config.SaveScopeSettings(paths, config.ScopeProject, projectSettings); err != nil {
		t.Fatalf("save project settings: %v", err)
	}

	localSettings := config.ScopeSettings{}
	mustSetScopeValue(t, &localSettings, config.ProviderModelKey(domain.ProviderOllama), "local-model")
	if err := config.SaveScopeSettings(paths, config.ScopeLocal, localSettings); err != nil {
		t.Fatalf("save local settings: %v", err)
	}

	if err := runConfigUpdate(repoRoot, configCommandState{
		scope:     config.ScopeLocal,
		unsetKeys: []config.SettingKey{config.ProviderModelKey(domain.ProviderOllama)},
	}); err != nil {
		t.Fatalf("run config update: %v", err)
	}

	resolved, err := config.LoadResolvedSettings(paths)
	if err != nil {
		t.Fatalf("load resolved settings: %v", err)
	}
	if resolved.Effective.ConfigFor(domain.ProviderOllama).Model != "project-model" {
		t.Fatalf("expected fallback to project model, got %q", resolved.Effective.ConfigFor(domain.ProviderOllama).Model)
	}

	localScope, err := config.LoadScopeSettings(paths, config.ScopeLocal)
	if err != nil {
		t.Fatalf("load local scope: %v", err)
	}
	if _, ok := localScope.ValueForKey(config.ProviderModelKey(domain.ProviderOllama)); ok {
		t.Fatalf("expected local override to be removed: %+v", localScope)
	}
	if _, err := os.Stat(paths.LocalSettingsFile); !os.IsNotExist(err) {
		t.Fatalf("expected local settings file to be removed, stat err=%v", err)
	}
}

func TestRunConfigUpdateMigratesLegacyDefaultProviderToUserScope(t *testing.T) {
	setTestConfigHome(t)

	repoRoot := t.TempDir()
	paths, err := resolveAppPaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve app paths: %v", err)
	}

	store, err := sqlitestore.Open(paths.DBPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.SaveDefaultProvider(context.Background(), domain.ProviderAzure); err != nil {
		t.Fatalf("save legacy default provider: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	patch := config.ScopeSettings{}
	mustSetScopeValue(t, &patch, config.ProviderModelKey(domain.ProviderChatGPT), "gpt-4.1")
	if err := runConfigUpdate(repoRoot, configCommandState{
		scope: config.ScopeProject,
		patch: patch,
	}); err != nil {
		t.Fatalf("run config update: %v", err)
	}

	userScope, err := config.LoadScopeSettings(paths, config.ScopeUser)
	if err != nil {
		t.Fatalf("load user scope: %v", err)
	}
	value, ok := userScope.ValueForKey(config.KeyDefaultProvider)
	if !ok || value != "azure" {
		t.Fatalf("expected migrated default provider in user scope, got ok=%t value=%q", ok, value)
	}

	resolved, err := config.LoadResolvedSettings(paths)
	if err != nil {
		t.Fatalf("load resolved settings: %v", err)
	}
	if resolved.Effective.DefaultProvider != domain.ProviderAzure {
		t.Fatalf("expected effective default provider to remain Azure, got %s", resolved.Effective.DefaultProvider)
	}
}

func mustSetScopeValue(t *testing.T, settings *config.ScopeSettings, key config.SettingKey, value string) {
	t.Helper()
	if err := settings.SetKey(key, value); err != nil {
		t.Fatalf("set %s: %v", key, err)
	}
}

func writeManagedSettingsFile(t *testing.T, path string, settings config.ScopeSettings) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create managed settings dir: %v", err)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatalf("marshal managed settings: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write managed settings: %v", err)
	}
}

func configLineMap(output string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, _, _ := strings.Cut(line, "=")
		result[key] = line
	}
	return result
}

func assertLineContains(t *testing.T, line string, want string) {
	t.Helper()
	if !strings.Contains(line, want) {
		t.Fatalf("expected line %q to contain %q", line, want)
	}
}

func setTestConfigHome(t *testing.T) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("ORCH_MANAGED_SETTINGS", filepath.Join(home, "managed-settings.json"))
}

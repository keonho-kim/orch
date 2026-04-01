package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
	sqlitestore "github.com/keonho-kim/orch/internal/store/sqlite"
)

const configUsage = "usage: orch config --list [--workspace <path>] [--scope <managed|user|project|local|effective>] [--show-origin] | orch config [--workspace <path>] [--scope <user|project|local>] [--provider=<provider>] [--model=<name>] [--unset <key>] [provider flags...]"

type configCommandState struct {
	scope      config.Scope
	showOrigin bool
	unsetKeys  []config.SettingKey
	patch      config.ScopeSettings
}

type providerFlagSpec struct {
	provider     domain.Provider
	prefix       string
	exposeAPIKey bool
}

type multiStringFlag []string

var providerFlagSpecs = []providerFlagSpec{
	{provider: domain.ProviderOllama, prefix: "ollama"},
	{provider: domain.ProviderVLLM, prefix: "vllm", exposeAPIKey: true},
	{provider: domain.ProviderGemini, prefix: "gemini", exposeAPIKey: true},
	{provider: domain.ProviderVertex, prefix: "vertex", exposeAPIKey: true},
	{provider: domain.ProviderBedrock, prefix: "bedrock", exposeAPIKey: true},
	{provider: domain.ProviderClaude, prefix: "claude", exposeAPIKey: true},
	{provider: domain.ProviderAzure, prefix: "azure", exposeAPIKey: true},
	{provider: domain.ProviderChatGPT, prefix: "chatgpt", exposeAPIKey: true},
}

func (m *multiStringFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiStringFlag) Set(value string) error {
	*m = append(*m, strings.TrimSpace(value))
	return nil
}

func visitedConfigFlags(flagSet *flag.FlagSet) map[string]bool {
	visited := make(map[string]bool)
	flagSet.Visit(func(item *flag.Flag) {
		visited[item.Name] = true
	})
	return visited
}

func parseConfigCommand(args []string) (command, error) {
	repoRoot, rest, err := parseWorkspaceFlag(args, ".")
	if err != nil {
		return command{}, err
	}
	if len(rest) == 0 {
		return command{}, fmt.Errorf(configUsage)
	}
	if len(rest) == 1 && rest[0] == "list" {
		rest = []string{"--list"}
	}

	flagSet := flag.NewFlagSet("config", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var listValue bool
	var scopeValue string
	var showOriginValue bool
	var providerValue string
	var modelValue string
	var approvalPolicyValue string
	var selfDrivingModeValue bool
	var reactRalphIterValue int
	var planRalphIterValue int
	var compactThresholdKValue int
	var unsetValues multiStringFlag

	flagSet.BoolVar(&listValue, "list", false, "")
	flagSet.StringVar(&scopeValue, "scope", "", "")
	flagSet.BoolVar(&showOriginValue, "show-origin", false, "")
	flagSet.Var(&unsetValues, "unset", "")
	flagSet.StringVar(&providerValue, "provider", "", "")
	flagSet.StringVar(&modelValue, "model", "", "")
	flagSet.StringVar(&approvalPolicyValue, "approval-policy", "", "")
	flagSet.BoolVar(&selfDrivingModeValue, "self-driving-mode", false, "")
	flagSet.IntVar(&reactRalphIterValue, "react-ralph-iter", 0, "")
	flagSet.IntVar(&planRalphIterValue, "plan-ralph-iter", 0, "")
	flagSet.IntVar(&compactThresholdKValue, "compact-threshold-k", 0, "")

	baseURLValues := make(map[domain.Provider]*string, len(providerFlagSpecs))
	modelValues := make(map[domain.Provider]*string, len(providerFlagSpecs))
	apiKeyValues := make(map[domain.Provider]*string, len(providerFlagSpecs))
	for _, spec := range providerFlagSpecs {
		baseURLValues[spec.provider] = new(string)
		modelValues[spec.provider] = new(string)
		flagSet.StringVar(baseURLValues[spec.provider], spec.prefix+"-base-url", "", "")
		flagSet.StringVar(modelValues[spec.provider], spec.prefix+"-model", "", "")
		if spec.exposeAPIKey {
			apiKeyValues[spec.provider] = new(string)
			flagSet.StringVar(apiKeyValues[spec.provider], spec.prefix+"-api-key-env", "", "")
		}
	}

	if err := flagSet.Parse(rest); err != nil {
		return command{}, err
	}
	if extra := flagSet.Args(); len(extra) > 0 {
		return command{}, fmt.Errorf("unexpected config arguments: %s", strings.Join(extra, " "))
	}

	scope, err := parseConfigScope(scopeValue, listValue)
	if err != nil {
		return command{}, err
	}

	visited := visitedConfigFlags(flagSet)
	if listValue {
		if err := validateListFlags(visited); err != nil {
			return command{}, err
		}
		return command{
			name:     "config-list",
			repoRoot: repoRoot,
			configCommand: configCommandState{
				scope:      scope,
				showOrigin: showOriginValue,
			},
		}, nil
	}
	if showOriginValue {
		return command{}, fmt.Errorf("--show-origin is only valid with --list")
	}

	patch, err := buildConfigPatch(
		visited,
		providerValue,
		modelValue,
		approvalPolicyValue,
		selfDrivingModeValue,
		reactRalphIterValue,
		planRalphIterValue,
		compactThresholdKValue,
		baseURLValues,
		modelValues,
		apiKeyValues,
	)
	if err != nil {
		return command{}, err
	}
	unsetKeys, err := parseUnsetKeys(unsetValues)
	if err != nil {
		return command{}, err
	}
	if err := validateUnsetKeys(patch, unsetKeys); err != nil {
		return command{}, err
	}
	if patch.IsEmpty() && len(unsetKeys) == 0 {
		return command{}, fmt.Errorf(configUsage)
	}

	return command{
		name:     "config-set",
		repoRoot: repoRoot,
		configCommand: configCommandState{
			scope:     scope,
			unsetKeys: unsetKeys,
			patch:     patch,
		},
	}, nil
}

func parseConfigScope(raw string, list bool) (config.Scope, error) {
	if strings.TrimSpace(raw) == "" {
		if list {
			return config.ScopeEffective, nil
		}
		return config.ScopeProject, nil
	}

	scope, err := config.ParseScope(raw)
	if err != nil {
		return "", err
	}
	if list {
		return scope, nil
	}
	if scope == config.ScopeManaged || scope == config.ScopeEffective || scope == config.ScopeBuiltin {
		return "", fmt.Errorf("%s scope is read-only", scope)
	}
	return scope, nil
}

func validateListFlags(visited map[string]bool) error {
	for key := range visited {
		switch key {
		case "list", "scope", "show-origin":
			continue
		default:
			return fmt.Errorf("--list may not be combined with write flags")
		}
	}
	return nil
}

func buildConfigPatch(
	visited map[string]bool,
	providerValue string,
	modelValue string,
	approvalPolicyValue string,
	selfDrivingModeValue bool,
	reactRalphIterValue int,
	planRalphIterValue int,
	compactThresholdKValue int,
	baseURLValues map[domain.Provider]*string,
	modelValues map[domain.Provider]*string,
	apiKeyValues map[domain.Provider]*string,
) (config.ScopeSettings, error) {
	patch := config.ScopeSettings{}

	var selectedProvider domain.Provider
	if visited["provider"] {
		providerValue = strings.TrimSpace(providerValue)
		if providerValue == "" {
			return config.ScopeSettings{}, fmt.Errorf("--provider is required")
		}
		parsedProvider, err := domain.ParseProvider(providerValue)
		if err != nil {
			return config.ScopeSettings{}, err
		}
		selectedProvider = parsedProvider
		patch.DefaultProvider = stringPtr(parsedProvider.String())
	}

	for _, spec := range providerFlagSpecs {
		baseURLFlag := spec.prefix + "-base-url"
		modelFlag := spec.prefix + "-model"
		apiKeyFlag := spec.prefix + "-api-key-env"

		if visited[baseURLFlag] {
			if err := patch.SetKey(config.ProviderBaseURLKey(spec.provider), strings.TrimSpace(*baseURLValues[spec.provider])); err != nil {
				return config.ScopeSettings{}, err
			}
		}
		if visited[modelFlag] {
			if err := patch.SetKey(config.ProviderModelKey(spec.provider), strings.TrimSpace(*modelValues[spec.provider])); err != nil {
				return config.ScopeSettings{}, err
			}
		}
		if spec.exposeAPIKey && visited[apiKeyFlag] {
			if err := patch.SetKey(config.ProviderAPIKeyEnvKey(spec.provider), strings.TrimSpace(*apiKeyValues[spec.provider])); err != nil {
				return config.ScopeSettings{}, err
			}
		}
	}

	if visited["approval-policy"] {
		if err := patch.SetKey(config.KeyApprovalPolicy, strings.TrimSpace(approvalPolicyValue)); err != nil {
			return config.ScopeSettings{}, err
		}
	}
	if visited["self-driving-mode"] {
		if err := patch.SetKey(config.KeySelfDrivingMode, fmt.Sprintf("%t", selfDrivingModeValue)); err != nil {
			return config.ScopeSettings{}, err
		}
	}
	if visited["react-ralph-iter"] {
		if reactRalphIterValue < 0 {
			return config.ScopeSettings{}, fmt.Errorf("--react-ralph-iter must be >= 0")
		}
		if err := patch.SetKey(config.KeyReactRalphIter, fmt.Sprintf("%d", reactRalphIterValue)); err != nil {
			return config.ScopeSettings{}, err
		}
	}
	if visited["plan-ralph-iter"] {
		if planRalphIterValue < 0 {
			return config.ScopeSettings{}, fmt.Errorf("--plan-ralph-iter must be >= 0")
		}
		if err := patch.SetKey(config.KeyPlanRalphIter, fmt.Sprintf("%d", planRalphIterValue)); err != nil {
			return config.ScopeSettings{}, err
		}
	}
	if visited["compact-threshold-k"] {
		if compactThresholdKValue < 0 {
			return config.ScopeSettings{}, fmt.Errorf("--compact-threshold-k must be >= 0")
		}
		if err := patch.SetKey(config.KeyCompactThresholdK, fmt.Sprintf("%d", compactThresholdKValue)); err != nil {
			return config.ScopeSettings{}, err
		}
	}
	if visited["model"] {
		if !visited["provider"] {
			return config.ScopeSettings{}, fmt.Errorf("--model requires --provider")
		}
		key := config.ProviderModelKey(selectedProvider)
		if raw, ok := patch.ValueForKey(key); ok && raw != strings.TrimSpace(modelValue) {
			return config.ScopeSettings{}, fmt.Errorf("--model conflicts with --%s-model", providerFlagPrefix(selectedProvider))
		}
		if err := patch.SetKey(key, strings.TrimSpace(modelValue)); err != nil {
			return config.ScopeSettings{}, err
		}
	}

	return patch, nil
}

func parseUnsetKeys(values []string) ([]config.SettingKey, error) {
	keys := make([]config.SettingKey, 0, len(values))
	for _, raw := range values {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		key, err := config.ParseSettingKey(raw)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func validateUnsetKeys(patch config.ScopeSettings, keys []config.SettingKey) error {
	for _, key := range keys {
		if _, ok := patch.ValueForKey(key); ok {
			return fmt.Errorf("--unset conflicts with explicit value for %s", key)
		}
	}
	return nil
}

func runConfigList(repoRoot string, state configCommandState, stdout io.Writer) error {
	paths, resolved, err := loadCLIResolvedSettings(repoRoot)
	if err != nil {
		return err
	}

	lines := make([]string, 0, len(config.AllSettingKeys()))
	switch state.scope {
	case config.ScopeEffective:
		for _, key := range config.AllSettingKeys() {
			value, _ := configValueForKey(resolved.Effective, key)
			lines = append(lines, formatConfigLine(key, value, resolved.Sources[key], state.showOrigin))
		}
	default:
		scopeSettings := resolved.Scopes[state.scope]
		scopeFile := resolved.Files[state.scope]
		for _, key := range config.AllSettingKeys() {
			value, _ := scopeSettings.ValueForKey(key)
			source := config.SourceInfo{}
			if value != "" || key == config.KeySelfDrivingMode || key == config.KeyReactRalphIter || key == config.KeyPlanRalphIter || key == config.KeyCompactThresholdK {
				if _, ok := scopeSettings.ValueForKey(key); ok {
					source = config.SourceInfo{Scope: state.scope, File: scopeFile}
				}
			}
			lines = append(lines, formatConfigLine(key, value, source, state.showOrigin))
		}
	}

	if _, err := io.WriteString(stdout, strings.Join(lines, "\n")+"\n"); err != nil {
		return fmt.Errorf("write config list: %w", err)
	}
	_ = paths
	return nil
}

func runConfigUpdate(repoRoot string, state configCommandState) error {
	paths, resolved, store, err := loadCLIConfigContext(repoRoot)
	if err != nil {
		return err
	}
	defer store.Close()

	if err := migrateLegacyDefaultProvider(paths, store, resolved); err != nil {
		return err
	}

	scopeSettings, err := config.LoadScopeSettings(paths, state.scope)
	if err != nil {
		return err
	}
	for _, key := range state.unsetKeys {
		configValueUnset(&scopeSettings, key)
	}
	mergeScopeSettings(&scopeSettings, state.patch)
	if err := config.SaveScopeSettings(paths, state.scope, scopeSettings); err != nil {
		return err
	}

	reloaded, err := config.LoadResolvedSettings(paths)
	if err != nil {
		return err
	}
	if reloaded.Effective.DefaultProvider != "" {
		if err := store.SaveDefaultProvider(context.Background(), reloaded.Effective.DefaultProvider); err != nil {
			return err
		}
	}
	return nil
}

func loadCLIResolvedSettings(repoRoot string) (config.Paths, config.ResolvedSettings, error) {
	paths, err := resolveAppPaths(repoRoot)
	if err != nil {
		return config.Paths{}, config.ResolvedSettings{}, err
	}
	if err := config.EnsureRuntimePaths(paths); err != nil {
		return config.Paths{}, config.ResolvedSettings{}, err
	}
	resolved, err := config.LoadResolvedSettings(paths)
	if err != nil {
		return config.Paths{}, config.ResolvedSettings{}, err
	}
	return paths, resolved, nil
}

func loadCLIConfigContext(repoRoot string) (config.Paths, config.ResolvedSettings, *sqlitestore.Store, error) {
	paths, resolved, err := loadCLIResolvedSettings(repoRoot)
	if err != nil {
		return config.Paths{}, config.ResolvedSettings{}, nil, err
	}
	store, err := sqlitestore.Open(paths.DBPath)
	if err != nil {
		return config.Paths{}, config.ResolvedSettings{}, nil, err
	}
	return paths, resolved, store, nil
}

func migrateLegacyDefaultProvider(paths config.Paths, store *sqlitestore.Store, resolved config.ResolvedSettings) error {
	if _, ok := resolved.Sources[config.KeyDefaultProvider]; ok {
		return nil
	}

	stored, err := store.LoadSettings(context.Background())
	if err != nil || stored.DefaultProvider == "" {
		return nil
	}

	userSettings, err := config.LoadScopeSettings(paths, config.ScopeUser)
	if err != nil {
		return err
	}
	value := stored.DefaultProvider.String()
	userSettings.DefaultProvider = &value
	return config.SaveScopeSettings(paths, config.ScopeUser, userSettings)
}

func mergeScopeSettings(target *config.ScopeSettings, patch config.ScopeSettings) {
	for _, key := range config.AllSettingKeys() {
		value, ok := patch.ValueForKey(key)
		if !ok {
			continue
		}
		_ = target.SetKey(key, value)
	}
}

func configValueUnset(settings *config.ScopeSettings, key config.SettingKey) {
	configValueRemove(settings, key)
}

func configValueRemove(settings *config.ScopeSettings, key config.SettingKey) {
	configValueUnsetImpl(settings, key)
}

func configValueUnsetImpl(settings *config.ScopeSettings, key config.SettingKey) {
	// `config.UnsetScopeSettings` persists; this helper only mutates the in-memory patch.
	switch key {
	case config.KeyDefaultProvider:
		settings.DefaultProvider = nil
	case config.KeyApprovalPolicy:
		settings.ApprovalPolicy = nil
	case config.KeySelfDrivingMode:
		settings.SelfDrivingMode = nil
	case config.KeyReactRalphIter:
		settings.ReactRalphIter = nil
	case config.KeyPlanRalphIter:
		settings.PlanRalphIter = nil
	case config.KeyCompactThresholdK:
		settings.CompactThresholdK = nil
	default:
		provider, attr, ok := parseProviderConfigKey(key)
		if !ok || settings.Providers == nil {
			return
		}
		patch := providerPatchPtr(settings.Providers, provider)
		if patch == nil {
			return
		}
		switch attr {
		case "base_url":
			patch.BaseURL = nil
		case "model":
			patch.Model = nil
		case "api_key_env":
			patch.APIKeyEnv = nil
		}
		if providerPatchEmpty(patch) {
			setProviderPatchPtr(settings.Providers, provider, nil)
		}
		if providerCatalogEmpty(settings.Providers) {
			settings.Providers = nil
		}
	}
}

func configValueForKey(settings domain.Settings, key config.SettingKey) (string, bool) {
	switch key {
	case config.KeyDefaultProvider:
		return settings.DefaultProvider.String(), true
	case config.KeyApprovalPolicy:
		return string(settings.ApprovalPolicy), true
	case config.KeySelfDrivingMode:
		return fmt.Sprintf("%t", settings.SelfDrivingMode), true
	case config.KeyReactRalphIter:
		return fmt.Sprintf("%d", settings.ReactRalphIter), true
	case config.KeyPlanRalphIter:
		return fmt.Sprintf("%d", settings.PlanRalphIter), true
	case config.KeyCompactThresholdK:
		return fmt.Sprintf("%d", settings.CompactThresholdK), true
	default:
		provider, attr, ok := parseProviderConfigKey(key)
		if !ok {
			return "", false
		}
		providerSettings := settings.ConfigFor(provider)
		switch attr {
		case "base_url":
			return providerSettings.BaseURL, true
		case "model":
			return providerSettings.Model, true
		case "api_key_env":
			return providerSettings.APIKeyEnv, true
		default:
			return "", false
		}
	}
}

func formatConfigLine(key config.SettingKey, value string, source config.SourceInfo, showOrigin bool) string {
	line := fmt.Sprintf("%s=%s", key, value)
	if !showOrigin {
		return line
	}
	if source.Scope == "" {
		return line
	}
	if source.File != "" {
		return fmt.Sprintf("%s\torigin=%s:%s", line, source.Scope, source.File)
	}
	return fmt.Sprintf("%s\torigin=%s", line, source.Scope)
}

func parseProviderConfigKey(key config.SettingKey) (domain.Provider, string, bool) {
	raw := strings.TrimPrefix(string(key), "providers.")
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return "", "", false
	}
	provider, err := domain.ParseProvider(parts[0])
	if err != nil {
		return "", "", false
	}
	return provider, parts[1], true
}

func providerFlagPrefix(provider domain.Provider) string {
	for _, spec := range providerFlagSpecs {
		if spec.provider == provider {
			return spec.prefix
		}
	}
	return provider.String()
}

func providerPatchPtr(catalog *config.ProviderCatalogPatch, provider domain.Provider) *config.ProviderSettingsPatch {
	switch provider {
	case domain.ProviderOllama:
		return catalog.Ollama
	case domain.ProviderVLLM:
		return catalog.VLLM
	case domain.ProviderGemini:
		return catalog.Gemini
	case domain.ProviderVertex:
		return catalog.Vertex
	case domain.ProviderBedrock:
		return catalog.Bedrock
	case domain.ProviderClaude:
		return catalog.Claude
	case domain.ProviderAzure:
		return catalog.Azure
	case domain.ProviderChatGPT:
		return catalog.ChatGPT
	default:
		return nil
	}
}

func setProviderPatchPtr(catalog *config.ProviderCatalogPatch, provider domain.Provider, patch *config.ProviderSettingsPatch) {
	switch provider {
	case domain.ProviderOllama:
		catalog.Ollama = patch
	case domain.ProviderVLLM:
		catalog.VLLM = patch
	case domain.ProviderGemini:
		catalog.Gemini = patch
	case domain.ProviderVertex:
		catalog.Vertex = patch
	case domain.ProviderBedrock:
		catalog.Bedrock = patch
	case domain.ProviderClaude:
		catalog.Claude = patch
	case domain.ProviderAzure:
		catalog.Azure = patch
	case domain.ProviderChatGPT:
		catalog.ChatGPT = patch
	}
}

func providerPatchEmpty(patch *config.ProviderSettingsPatch) bool {
	return patch == nil || (patch.BaseURL == nil && patch.Model == nil && patch.APIKeyEnv == nil)
}

func providerCatalogEmpty(catalog *config.ProviderCatalogPatch) bool {
	return catalog == nil ||
		(catalog.Ollama == nil &&
			catalog.VLLM == nil &&
			catalog.Gemini == nil &&
			catalog.Vertex == nil &&
			catalog.Bedrock == nil &&
			catalog.Claude == nil &&
			catalog.Azure == nil &&
			catalog.ChatGPT == nil)
}

func resolveAppPaths(repoRoot string) (config.Paths, error) {
	absoluteRepoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return config.Paths{}, fmt.Errorf("resolve working directory: %w", err)
	}

	paths, err := config.ResolvePaths(absoluteRepoRoot)
	if err != nil {
		return config.Paths{}, err
	}
	return paths, nil
}

func stringPtr(value string) *string {
	return &value
}

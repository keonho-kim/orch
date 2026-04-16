package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
)

func parseConfigCommand(args []string) (command, error) {
	repoRoot, rest, err := parseWorkspaceFlag(args, ".")
	if err != nil {
		return command{}, err
	}
	if len(rest) == 1 && rest[0] == "migrate" {
		return command{name: "config-migrate", repoRoot: repoRoot}, nil
	}
	if len(rest) == 0 {
		return command{}, fmt.Errorf("%s", configUsage)
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
		return command{}, fmt.Errorf("%s", configUsage)
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
	if scope == config.ScopeEffective || scope == config.ScopeBuiltin {
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

func providerFlagPrefix(provider domain.Provider) string {
	for _, spec := range providerFlagSpecs {
		if spec.provider == provider {
			return spec.prefix
		}
	}
	return provider.String()
}

func stringPtr(value string) *string {
	return &value
}

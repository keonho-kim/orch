package config

import (
	"fmt"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

const (
	KeyDefaultProvider   SettingKey = "orch.default_provider"
	KeyApprovalPolicy    SettingKey = "orch.approval_policy"
	KeySelfDrivingMode   SettingKey = "orch.self_driving_mode"
	KeyReactRalphIter    SettingKey = "orch.react_ralph_iter"
	KeyPlanRalphIter     SettingKey = "orch.plan_ralph_iter"
	KeyCompactThresholdK SettingKey = "orch.compact_threshold_k"
	KeyInstallBinDir     SettingKey = "install.bin_dir"
)

func ParseScope(value string) (Scope, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(ScopeEffective):
		return ScopeEffective, nil
	case string(ScopeGlobal), string(ScopeUser), string(ScopeManaged):
		return ScopeGlobal, nil
	case string(ScopeProject), string(ScopeLocal):
		return ScopeProject, nil
	default:
		return "", fmt.Errorf("unsupported config scope %q", value)
	}
}

func EditableScopes() []Scope {
	return []Scope{ScopeGlobal, ScopeProject}
}

func AllSettingKeys() []SettingKey {
	keys := []SettingKey{
		KeyDefaultProvider,
		KeyApprovalPolicy,
		KeySelfDrivingMode,
		KeyReactRalphIter,
		KeyPlanRalphIter,
		KeyCompactThresholdK,
		KeyInstallBinDir,
	}
	for _, provider := range domain.Providers() {
		keys = append(keys,
			ProviderBaseURLKey(provider),
			ProviderModelKey(provider),
			ProviderAuthKindKey(provider),
			ProviderAPIKeyEnvKey(provider),
			ProviderAuthValueKey(provider),
			ProviderAuthFileKey(provider),
		)
	}
	return keys
}

func ProviderBaseURLKey(provider domain.Provider) SettingKey {
	return SettingKey(fmt.Sprintf("provider.%s.base_url", provider.String()))
}

func ProviderModelKey(provider domain.Provider) SettingKey {
	return SettingKey(fmt.Sprintf("provider.%s.model", provider.String()))
}

func ProviderAPIKeyEnvKey(provider domain.Provider) SettingKey {
	return SettingKey(fmt.Sprintf("provider.%s.auth.env", provider.String()))
}

func ProviderAuthKindKey(provider domain.Provider) SettingKey {
	return SettingKey(fmt.Sprintf("provider.%s.auth.kind", provider.String()))
}

func ProviderAuthValueKey(provider domain.Provider) SettingKey {
	return SettingKey(fmt.Sprintf("provider.%s.auth.value", provider.String()))
}

func ProviderAuthFileKey(provider domain.Provider) SettingKey {
	return SettingKey(fmt.Sprintf("provider.%s.auth.file", provider.String()))
}

func ParseSettingKey(value string) (SettingKey, error) {
	key := SettingKey(strings.TrimSpace(value))
	switch key {
	case KeyDefaultProvider, KeyApprovalPolicy, KeySelfDrivingMode, KeyReactRalphIter, KeyPlanRalphIter, KeyCompactThresholdK, KeyInstallBinDir:
		return key, nil
	case "default_provider":
		return KeyDefaultProvider, nil
	case "approval_policy":
		return KeyApprovalPolicy, nil
	case "self_driving_mode":
		return KeySelfDrivingMode, nil
	case "react_ralph_iter":
		return KeyReactRalphIter, nil
	case "plan_ralph_iter":
		return KeyPlanRalphIter, nil
	case "compact_threshold_k":
		return KeyCompactThresholdK, nil
	}
	for _, provider := range domain.Providers() {
		if key == ProviderBaseURLKey(provider) ||
			key == ProviderModelKey(provider) ||
			key == ProviderAPIKeyEnvKey(provider) ||
			key == ProviderAuthKindKey(provider) ||
			key == ProviderAuthValueKey(provider) ||
			key == ProviderAuthFileKey(provider) {
			return key, nil
		}
		if key == SettingKey(fmt.Sprintf("providers.%s.base_url", provider.String())) ||
			key == SettingKey(fmt.Sprintf("providers.%s.model", provider.String())) ||
			key == SettingKey(fmt.Sprintf("providers.%s.api_key_env", provider.String())) {
			switch {
			case strings.HasSuffix(string(key), ".base_url"):
				return ProviderBaseURLKey(provider), nil
			case strings.HasSuffix(string(key), ".model"):
				return ProviderModelKey(provider), nil
			default:
				return ProviderAPIKeyEnvKey(provider), nil
			}
		}
	}
	return "", fmt.Errorf("unsupported setting key %q", value)
}

func (s ScopeSettings) ValueForKey(key SettingKey) (string, bool) {
	switch key {
	case KeyDefaultProvider:
		return stringValue(s.DefaultProvider)
	case KeyApprovalPolicy:
		return stringValue(s.ApprovalPolicy)
	case KeySelfDrivingMode:
		return boolValue(s.SelfDrivingMode)
	case KeyReactRalphIter:
		return intValue(s.ReactRalphIter)
	case KeyPlanRalphIter:
		return intValue(s.PlanRalphIter)
	case KeyCompactThresholdK:
		return intValue(s.CompactThresholdK)
	case KeyInstallBinDir:
		if s.Install == nil {
			return "", false
		}
		return stringValue(s.Install.BinDir)
	default:
		provider, attr, ok := parseProviderSettingKey(key)
		if !ok || s.Providers == nil {
			return "", false
		}
		patch := providerPatch(s.Providers, provider)
		if patch == nil {
			return "", false
		}
		switch attr {
		case "base_url":
			return stringValue(patch.BaseURL)
		case "model":
			return stringValue(patch.Model)
		case "auth.kind":
			if patch.Auth == nil {
				return "", false
			}
			return stringValue(patch.Auth.Kind)
		case "auth.env":
			if patch.Auth == nil {
				return "", false
			}
			return stringValue(patch.Auth.Env)
		case "auth.value":
			if patch.Auth == nil {
				return "", false
			}
			return stringValue(patch.Auth.Value)
		case "auth.file":
			if patch.Auth == nil {
				return "", false
			}
			return stringValue(patch.Auth.File)
		default:
			return "", false
		}
	}
}

func (s *ScopeSettings) SetKey(key SettingKey, value string) error {
	switch key {
	case KeyDefaultProvider:
		s.DefaultProvider = stringPtr(strings.TrimSpace(value))
		return nil
	case KeyApprovalPolicy:
		s.ApprovalPolicy = stringPtr(strings.TrimSpace(value))
		return nil
	case KeySelfDrivingMode:
		valueBool, err := parseBool(value)
		if err != nil {
			return err
		}
		s.SelfDrivingMode = boolPtr(valueBool)
		return nil
	case KeyReactRalphIter:
		valueInt, err := parseInt(value)
		if err != nil {
			return err
		}
		s.ReactRalphIter = intPtr(valueInt)
		return nil
	case KeyPlanRalphIter:
		valueInt, err := parseInt(value)
		if err != nil {
			return err
		}
		s.PlanRalphIter = intPtr(valueInt)
		return nil
	case KeyCompactThresholdK:
		valueInt, err := parseInt(value)
		if err != nil {
			return err
		}
		s.CompactThresholdK = intPtr(valueInt)
		return nil
	case KeyInstallBinDir:
		if s.Install == nil {
			s.Install = &InstallPatch{}
		}
		s.Install.BinDir = stringPtr(strings.TrimSpace(value))
		return nil
	default:
		provider, attr, ok := parseProviderSettingKey(key)
		if !ok {
			return fmt.Errorf("unsupported setting key %q", key)
		}
		if s.Providers == nil {
			s.Providers = &ProviderCatalogPatch{}
		}
		patch := ensureProviderPatch(s.Providers, provider)
		switch attr {
		case "base_url":
			patch.BaseURL = stringPtr(strings.TrimSpace(value))
		case "model":
			patch.Model = stringPtr(strings.TrimSpace(value))
		case "auth.kind":
			if patch.Auth == nil {
				patch.Auth = &ProviderAuthPatch{}
			}
			patch.Auth.Kind = stringPtr(strings.TrimSpace(value))
		case "auth.env":
			if patch.Auth == nil {
				patch.Auth = &ProviderAuthPatch{}
			}
			patch.Auth.Kind = stringPtr(string(domain.ProviderAuthEnv))
			patch.Auth.Env = stringPtr(strings.TrimSpace(value))
			patch.APIKeyEnv = patch.Auth.Env
		case "auth.value":
			if patch.Auth == nil {
				patch.Auth = &ProviderAuthPatch{}
			}
			patch.Auth.Kind = stringPtr(string(domain.ProviderAuthValue))
			patch.Auth.Value = stringPtr(strings.TrimSpace(value))
		case "auth.file":
			if patch.Auth == nil {
				patch.Auth = &ProviderAuthPatch{}
			}
			patch.Auth.Kind = stringPtr(string(domain.ProviderAuthFile))
			patch.Auth.File = stringPtr(strings.TrimSpace(value))
		default:
			return fmt.Errorf("unsupported setting key %q", key)
		}
		return nil
	}
}

func effectiveValueForKey(settings domain.Settings, key SettingKey) (string, bool) {
	switch key {
	case KeyDefaultProvider:
		return settings.DefaultProvider.String(), settings.DefaultProvider != ""
	case KeyApprovalPolicy:
		return string(settings.ApprovalPolicy), true
	case KeySelfDrivingMode:
		return fmt.Sprintf("%t", settings.SelfDrivingMode), true
	case KeyReactRalphIter:
		return fmt.Sprintf("%d", settings.ReactRalphIter), true
	case KeyPlanRalphIter:
		return fmt.Sprintf("%d", settings.PlanRalphIter), true
	case KeyCompactThresholdK:
		return fmt.Sprintf("%d", settings.CompactThresholdK), true
	default:
		provider, attr, ok := parseProviderSettingKey(key)
		if !ok {
			return "", false
		}
		config := settings.ConfigFor(provider)
		switch attr {
		case "base_url":
			return config.BaseURL, true
		case "model":
			return config.Model, true
		case "auth.kind":
			return string(config.Auth.Kind), true
		case "auth.env":
			return config.Auth.Env, true
		case "auth.value":
			if strings.TrimSpace(config.Auth.Value) == "" {
				return "", true
			}
			return "<redacted>", true
		case "auth.file":
			return config.Auth.File, true
		default:
			return "", false
		}
	}
}

func parseProviderSettingKey(key SettingKey) (domain.Provider, string, bool) {
	raw := strings.TrimPrefix(string(key), "provider.")
	parts := strings.Split(raw, ".")
	if len(parts) < 2 {
		return "", "", false
	}
	provider, err := domain.ParseProvider(parts[0])
	if err != nil {
		return "", "", false
	}
	if len(parts) == 2 {
		return provider, parts[1], true
	}
	return provider, strings.Join(parts[1:], "."), true
}

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/keonho-kim/orch/internal/config"
)

func runConfigList(repoRoot string, state configCommandState, stdout io.Writer) error {
	_, resolved, err := loadCLIResolvedSettings(repoRoot)
	if err != nil {
		return err
	}

	lines := make([]string, 0, len(config.AllSettingKeys())+32)
	switch state.scope {
	case config.ScopeEffective:
		for _, key := range config.AllSettingKeys() {
			value, _ := configValueForKey(resolved.Effective, key)
			lines = append(lines, formatConfigLine(key, value, resolved.Sources[key], state.showOrigin))
		}
		lines = append(lines, renderEnvLines(resolved.EffectiveEnv, state.showOrigin, resolved.Files[config.ScopeProject], resolved.Files[config.ScopeGlobal], config.ScopeEffective)...)
	default:
		scopeSettings := resolved.Scopes[state.scope]
		scopeFile := resolved.Files[state.scope]
		for _, key := range config.AllSettingKeys() {
			value, _ := scopeSettings.ValueForKey(key)
			source := config.SourceInfo{Scope: state.scope, File: scopeFile}
			if _, ok := scopeSettings.ValueForKey(key); !ok {
				source = config.SourceInfo{}
			}
			lines = append(lines, formatConfigLine(key, value, source, state.showOrigin))
		}
		lines = append(lines, renderScopeEnvLines(scopeSettings, state.scope, scopeFile, state.showOrigin)...)
	}

	if _, err := io.WriteString(stdout, strings.Join(lines, "\n")+"\n"); err != nil {
		return fmt.Errorf("write config list: %w", err)
	}
	return nil
}

func runConfigUpdate(repoRoot string, state configCommandState) error {
	paths, _, err := loadCLIResolvedSettings(repoRoot)
	if err != nil {
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
		case "auth.kind":
			if patch.Auth != nil {
				patch.Auth.Kind = nil
			}
		case "auth.env":
			if patch.Auth != nil {
				patch.Auth.Env = nil
			}
		case "auth.value":
			if patch.Auth != nil {
				patch.Auth.Value = nil
			}
		case "auth.file":
			if patch.Auth != nil {
				patch.Auth.File = nil
			}
		}
		if providerAuthEmpty(patch.Auth) {
			patch.Auth = nil
		}
		if providerPatchEmpty(patch) {
			setProviderPatchPtr(settings.Providers, provider, nil)
		}
		if providerCatalogEmpty(settings.Providers) {
			settings.Providers = nil
		}
	}
}

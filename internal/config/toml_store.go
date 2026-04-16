package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/keonho-kim/orch/domain"
)

func LoadScopeSettings(paths Paths, scope Scope) (ScopeSettings, error) {
	path, err := scopeFile(paths, scope)
	if err != nil {
		return ScopeSettings{}, err
	}
	if path == "" {
		return ScopeSettings{}, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ScopeSettings{}, nil
	}
	if err != nil {
		return ScopeSettings{}, fmt.Errorf("read %s settings: %w", scope, err)
	}

	var raw rawConfigFile
	meta, err := toml.Decode(string(data), &raw)
	if err != nil {
		return ScopeSettings{}, fmt.Errorf("parse %s settings: %w", scope, err)
	}
	return raw.toScopeSettings(meta, filepath.Dir(path))
}

func SaveScopeSettings(paths Paths, scope Scope, settings ScopeSettings) error {
	scope, err := normalizeWritableScope(scope)
	if err != nil {
		return err
	}
	path, err := scopeFile(paths, scope)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("%s scope does not have a backing file", scope)
	}
	if settings.IsEmpty() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove empty %s settings: %w", scope, err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s settings directory: %w", scope, err)
	}

	raw := newRawConfigFile(settings)
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s settings: %w", scope, err)
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(raw); err != nil {
		return fmt.Errorf("encode %s settings: %w", scope, err)
	}
	if scope == ScopeProject {
		if err := ensureProjectConfigIgnored(paths); err != nil {
			return err
		}
	}
	return nil
}

func UnsetScopeSettings(paths Paths, scope Scope, keys []SettingKey) error {
	scopeSettings, err := LoadScopeSettings(paths, scope)
	if err != nil {
		return err
	}
	for _, key := range keys {
		unsetScopeSetting(&scopeSettings, key)
	}
	return SaveScopeSettings(paths, scope, scopeSettings)
}

func scopeFile(paths Paths, scope Scope) (string, error) {
	switch scope {
	case ScopeGlobal:
		return paths.GlobalSettingsFile, nil
	case ScopeProject:
		return paths.ProjectSettingsFile, nil
	case ScopeEffective, ScopeBuiltin:
		return "", fmt.Errorf("%s settings do not have a backing file", scope)
	default:
		return "", fmt.Errorf("unsupported config scope %q", scope)
	}
}

func ensureProjectConfigIgnored(paths Paths) error {
	if strings.TrimSpace(paths.RepoRoot) == "" {
		return nil
	}
	gitDir := filepath.Join(paths.RepoRoot, ".git")
	if !dirExists(gitDir) {
		return nil
	}
	excludePath := filepath.Join(gitDir, "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return fmt.Errorf("create git exclude directory: %w", err)
	}
	data, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read git exclude: %w", err)
	}
	entry := settingsFileName
	content := string(data)
	if strings.Contains(content, entry) {
		return nil
	}
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += entry + "\n"
	if err := os.WriteFile(excludePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write git exclude: %w", err)
	}
	return nil
}

func unsetScopeSetting(settings *ScopeSettings, key SettingKey) {
	switch key {
	case KeyDefaultProvider:
		settings.DefaultProvider = nil
	case KeyApprovalPolicy:
		settings.ApprovalPolicy = nil
	case KeySelfDrivingMode:
		settings.SelfDrivingMode = nil
	case KeyReactRalphIter:
		settings.ReactRalphIter = nil
	case KeyPlanRalphIter:
		settings.PlanRalphIter = nil
	case KeyCompactThresholdK:
		settings.CompactThresholdK = nil
	case KeyInstallBinDir:
		if settings.Install != nil {
			settings.Install.BinDir = nil
			if settings.Install.isEmpty() {
				settings.Install = nil
			}
		}
	default:
		provider, attr, ok := parseProviderSettingKey(key)
		if !ok || settings.Providers == nil {
			return
		}
		patch := providerPatch(settings.Providers, provider)
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
		if patch.Auth != nil && patch.Auth.isEmpty() {
			patch.Auth = nil
		}
		if patch.isEmpty() {
			setProviderPatch(settings.Providers, provider, nil)
		}
		if settings.Providers.isEmpty() {
			settings.Providers = nil
		}
	}
}

func ScopeSettingsFromDomainSettings(settings domain.Settings) ScopeSettings {
	settings.Normalize()
	result := ScopeSettings{
		Version:           intPtr(1),
		DefaultProvider:   stringPtr(settings.DefaultProvider.String()),
		ApprovalPolicy:    stringPtr(string(settings.ApprovalPolicy)),
		SelfDrivingMode:   boolPtr(settings.SelfDrivingMode),
		ReactRalphIter:    intPtr(settings.ReactRalphIter),
		PlanRalphIter:     intPtr(settings.PlanRalphIter),
		CompactThresholdK: intPtr(settings.CompactThresholdK),
		Providers:         &ProviderCatalogPatch{},
	}
	for _, provider := range domain.Providers() {
		config := settings.ConfigFor(provider)
		patch := &ProviderSettingsPatch{
			BaseURL: stringPtr(config.BaseURL),
			Model:   stringPtr(config.Model),
			Auth: &ProviderAuthPatch{
				Kind:  stringPtr(string(config.Auth.Kind)),
				Value: maybeStringPtr(config.Auth.Value),
				Env:   maybeStringPtr(config.Auth.Env),
				File:  maybeStringPtr(config.Auth.File),
			},
		}
		setProviderPatch(result.Providers, provider, patch)
	}
	return result
}

func newRawConfigFile(settings ScopeSettings) rawConfigFile {
	raw := rawConfigFile{
		Version: settings.Version,
		Install: settings.Install,
	}
	if settings.DefaultProvider != nil || settings.ApprovalPolicy != nil || settings.SelfDrivingMode != nil || settings.ReactRalphIter != nil || settings.PlanRalphIter != nil || settings.CompactThresholdK != nil {
		raw.Orch = &rawOrchConfig{
			DefaultProvider:   settings.DefaultProvider,
			ApprovalPolicy:    settings.ApprovalPolicy,
			SelfDrivingMode:   settings.SelfDrivingMode,
			ReactRalphIter:    settings.ReactRalphIter,
			PlanRalphIter:     settings.PlanRalphIter,
			CompactThresholdK: settings.CompactThresholdK,
		}
	}
	if settings.Providers != nil {
		raw.Provider = make(map[string]*rawProviderConfig)
		for _, provider := range domain.Providers() {
			patch := providerPatch(settings.Providers, provider)
			if patch == nil {
				continue
			}
			raw.Provider[provider.String()] = &rawProviderConfig{
				BaseURL: patch.BaseURL,
				Model:   patch.Model,
				Auth:    syncPatchAuth(patch),
			}
		}
	}
	if settings.Env != nil && !settings.Env.isEmpty() {
		raw.Env = &rawEnvConfig{
			Global:  settings.Env.Global,
			Gateway: settings.Env.Gateway,
			Worker:  settings.Env.Worker,
			OT:      settings.Env.OT,
		}
	}
	return raw
}

func (r rawConfigFile) toScopeSettings(meta toml.MetaData, baseDir string) (ScopeSettings, error) {
	result := ScopeSettings{
		Version: r.Version,
	}
	if r.Orch != nil {
		result.DefaultProvider = r.Orch.DefaultProvider
		result.ApprovalPolicy = r.Orch.ApprovalPolicy
		result.SelfDrivingMode = r.Orch.SelfDrivingMode
		result.ReactRalphIter = r.Orch.ReactRalphIter
		result.PlanRalphIter = r.Orch.PlanRalphIter
		result.CompactThresholdK = r.Orch.CompactThresholdK
	}
	if r.Install != nil || meta.IsDefined("install") {
		result.Install = r.Install
		if result.Install == nil {
			result.Install = &InstallPatch{}
		}
	}
	if len(r.Provider) > 0 || meta.IsDefined("provider") {
		result.Providers = &ProviderCatalogPatch{}
		for _, provider := range domain.Providers() {
			if !meta.IsDefined("provider", provider.String()) {
				continue
			}
			item := r.Provider[provider.String()]
			patch := &ProviderSettingsPatch{}
			if item != nil {
				patch.BaseURL = item.BaseURL
				patch.Model = item.Model
				patch.Auth = item.Auth
				if item.Auth != nil {
					patch.APIKeyEnv = item.Auth.Env
				}
			}
			if meta.IsDefined("provider", provider.String(), "auth") && patch.Auth == nil {
				patch.Auth = &ProviderAuthPatch{}
			}
			setProviderPatch(result.Providers, provider, patch)
		}
		if result.Providers.isEmpty() {
			result.Providers = nil
		}
	}
	if r.Env != nil || meta.IsDefined("env") {
		result.Env = &EnvLayersPatch{}
		if meta.IsDefined("env", "global") {
			result.Env.GlobalDefined = true
			result.Env.Global = cloneStringMap(r.envMap("global"))
		}
		if meta.IsDefined("env", "gateway") {
			result.Env.GatewayDefined = true
			result.Env.Gateway = cloneStringMap(r.envMap("gateway"))
		}
		if meta.IsDefined("env", "worker") {
			result.Env.WorkerDefined = true
			result.Env.Worker = cloneStringMap(r.envMap("worker"))
		}
		if meta.IsDefined("env", "ot") {
			result.Env.OTDefined = true
			result.Env.OT = cloneStringMap(r.envMap("ot"))
		}
	}

	for _, provider := range domain.Providers() {
		config := providerPatch(result.Providers, provider)
		if config == nil || config.Auth == nil {
			continue
		}
		if config.Auth.File != nil && strings.TrimSpace(*config.Auth.File) != "" && !filepath.IsAbs(strings.TrimSpace(*config.Auth.File)) {
			path := filepath.Clean(filepath.Join(baseDir, strings.TrimSpace(*config.Auth.File)))
			config.Auth.File = stringPtr(path)
		}
	}
	return result, nil
}

func (r rawConfigFile) envMap(name string) map[string]string {
	if r.Env == nil {
		return nil
	}
	switch name {
	case "global":
		return r.Env.Global
	case "gateway":
		return r.Env.Gateway
	case "worker":
		return r.Env.Worker
	case "ot":
		return r.Env.OT
	default:
		return nil
	}
}

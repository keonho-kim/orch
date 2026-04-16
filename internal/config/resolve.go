package config

import (
	"fmt"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

func LoadResolvedSettings(paths Paths) (ResolvedSettings, error) {
	if err := MigrateLegacyJSON(paths); err != nil {
		return ResolvedSettings{}, err
	}

	scopeFiles := map[Scope]string{
		ScopeGlobal:  paths.GlobalSettingsFile,
		ScopeProject: paths.ProjectSettingsFile,
	}
	scopes := make(map[Scope]ScopeSettings, len(scopeFiles))
	for scope := range scopeFiles {
		settings, err := LoadScopeSettings(paths, scope)
		if err != nil {
			return ResolvedSettings{}, err
		}
		scopes[scope] = settings
	}
	return ResolveSettings(scopes, scopeFiles), nil
}

func ResolveSettings(scopes map[Scope]ScopeSettings, scopeFiles map[Scope]string) ResolvedSettings {
	effective := defaultSettings()
	sources := make(SourceMap)
	seedBuiltinSources(&effective, sources)
	effectiveInstall := EffectiveInstall{}
	effectiveEnv := EffectiveEnv{
		Global:  map[string]string{},
		Gateway: map[string]string{},
		Worker:  map[string]string{},
		OT:      map[string]string{},
	}

	for _, scope := range []Scope{ScopeGlobal, ScopeProject} {
		applyScopeSettings(&effective, &effectiveInstall, &effectiveEnv, sources, scope, scopes[scope], scopeFiles[scope])
	}
	effective.Normalize()

	expandedScopes := map[Scope]ScopeSettings{
		ScopeGlobal:  scopes[ScopeGlobal],
		ScopeProject: scopes[ScopeProject],
		ScopeManaged: scopes[ScopeGlobal],
		ScopeUser:    scopes[ScopeGlobal],
		ScopeLocal:   scopes[ScopeProject],
	}
	expandedFiles := map[Scope]string{
		ScopeGlobal:  scopeFiles[ScopeGlobal],
		ScopeProject: scopeFiles[ScopeProject],
		ScopeManaged: scopeFiles[ScopeGlobal],
		ScopeUser:    scopeFiles[ScopeGlobal],
		ScopeLocal:   scopeFiles[ScopeProject],
	}

	return ResolvedSettings{
		Effective:        effective,
		EffectiveInstall: effectiveInstall,
		EffectiveEnv:     effectiveEnv,
		Sources:          sources,
		Scopes:           expandedScopes,
		Files:            expandedFiles,
	}
}

func applyScopeSettings(settings *domain.Settings, install *EffectiveInstall, env *EffectiveEnv, sources SourceMap, scope Scope, patch ScopeSettings, file string) {
	if patch.DefaultProvider != nil && strings.TrimSpace(*patch.DefaultProvider) != "" {
		if provider, err := domain.ParseProvider(*patch.DefaultProvider); err == nil {
			settings.DefaultProvider = provider
			sources[KeyDefaultProvider] = SourceInfo{Scope: scope, File: file}
		}
	}
	if patch.ApprovalPolicy != nil {
		settings.ApprovalPolicy = domain.ApprovalPolicy(strings.TrimSpace(*patch.ApprovalPolicy))
		sources[KeyApprovalPolicy] = SourceInfo{Scope: scope, File: file}
	}
	if patch.SelfDrivingMode != nil {
		settings.SelfDrivingMode = *patch.SelfDrivingMode
		sources[KeySelfDrivingMode] = SourceInfo{Scope: scope, File: file}
	}
	if patch.ReactRalphIter != nil && *patch.ReactRalphIter > 0 {
		settings.ReactRalphIter = *patch.ReactRalphIter
		sources[KeyReactRalphIter] = SourceInfo{Scope: scope, File: file}
	}
	if patch.PlanRalphIter != nil && *patch.PlanRalphIter > 0 {
		settings.PlanRalphIter = *patch.PlanRalphIter
		sources[KeyPlanRalphIter] = SourceInfo{Scope: scope, File: file}
	}
	if patch.CompactThresholdK != nil && *patch.CompactThresholdK > 0 {
		settings.CompactThresholdK = *patch.CompactThresholdK
		sources[KeyCompactThresholdK] = SourceInfo{Scope: scope, File: file}
	}
	if patch.Install != nil && patch.Install.BinDir != nil && scope == ScopeGlobal {
		install.BinDir = strings.TrimSpace(*patch.Install.BinDir)
		sources[KeyInstallBinDir] = SourceInfo{Scope: scope, File: file}
	}
	if patch.Providers != nil {
		for _, provider := range domain.Providers() {
			item := providerPatch(patch.Providers, provider)
			if item == nil {
				continue
			}
			target := providerSettingsPtr(settings, provider)
			if item.BaseURL != nil {
				target.BaseURL = strings.TrimSpace(*item.BaseURL)
				sources[ProviderBaseURLKey(provider)] = SourceInfo{Scope: scope, File: file}
			}
			if item.Model != nil {
				target.Model = strings.TrimSpace(*item.Model)
				sources[ProviderModelKey(provider)] = SourceInfo{Scope: scope, File: file}
			}
			if item.Auth != nil {
				applyAuthPatch(target, item.Auth, file)
				if item.Auth.Env != nil {
					sources[ProviderAPIKeyEnvKey(provider)] = SourceInfo{Scope: scope, File: file}
				}
			}
		}
	}
	if patch.Env != nil {
		if patch.Env.GlobalDefined {
			env.Global = mergeStringMap(env.Global, patch.Env.Global)
		}
		if patch.Env.GatewayDefined {
			env.Gateway = mergeStringMap(env.Gateway, patch.Env.Gateway)
		}
		if patch.Env.WorkerDefined {
			env.Worker = mergeStringMap(env.Worker, patch.Env.Worker)
		}
		if patch.Env.OTDefined {
			env.OT = mergeStringMap(env.OT, patch.Env.OT)
		}
	}
}

func seedBuiltinSources(settings *domain.Settings, sources SourceMap) {
	for _, key := range AllSettingKeys() {
		if _, ok := effectiveValueForKey(*settings, key); ok {
			sources[key] = SourceInfo{Scope: ScopeBuiltin}
		}
	}
}

func mergeStringMap(base map[string]string, overlay map[string]string) map[string]string {
	next := make(map[string]string, len(base)+len(overlay))
	for key, value := range base {
		next[key] = value
	}
	for key, value := range overlay {
		if strings.TrimSpace(value) == "" {
			delete(next, key)
			continue
		}
		next[key] = value
	}
	return next
}

func normalizeWritableScope(scope Scope) (Scope, error) {
	switch scope {
	case ScopeGlobal, ScopeProject:
		return scope, nil
	case ScopeUser, ScopeManaged:
		return ScopeGlobal, nil
	case ScopeLocal:
		return ScopeProject, nil
	default:
		return "", fmt.Errorf("%s settings are read-only", scope)
	}
}

func mergeScopeSettings(base ScopeSettings, overlay ScopeSettings) ScopeSettings {
	next := base
	if overlay.Version != nil {
		next.Version = overlay.Version
	}
	if overlay.DefaultProvider != nil {
		next.DefaultProvider = overlay.DefaultProvider
	}
	if overlay.ApprovalPolicy != nil {
		next.ApprovalPolicy = overlay.ApprovalPolicy
	}
	if overlay.SelfDrivingMode != nil {
		next.SelfDrivingMode = overlay.SelfDrivingMode
	}
	if overlay.ReactRalphIter != nil {
		next.ReactRalphIter = overlay.ReactRalphIter
	}
	if overlay.PlanRalphIter != nil {
		next.PlanRalphIter = overlay.PlanRalphIter
	}
	if overlay.CompactThresholdK != nil {
		next.CompactThresholdK = overlay.CompactThresholdK
	}
	if overlay.Install != nil {
		if next.Install == nil {
			next.Install = &InstallPatch{}
		}
		if overlay.Install.BinDir != nil {
			next.Install.BinDir = overlay.Install.BinDir
		}
	}
	if overlay.Providers != nil {
		if next.Providers == nil {
			next.Providers = &ProviderCatalogPatch{}
		}
		for _, provider := range domain.Providers() {
			item := providerPatch(overlay.Providers, provider)
			if item == nil {
				continue
			}
			target := ensureProviderPatch(next.Providers, provider)
			if item.BaseURL != nil {
				target.BaseURL = item.BaseURL
			}
			if item.Model != nil {
				target.Model = item.Model
			}
			if item.APIKeyEnv != nil {
				target.APIKeyEnv = item.APIKeyEnv
			}
			if item.Auth != nil {
				if target.Auth == nil {
					target.Auth = &ProviderAuthPatch{}
				}
				if item.Auth.Kind != nil {
					target.Auth.Kind = item.Auth.Kind
				}
				if item.Auth.Value != nil {
					target.Auth.Value = item.Auth.Value
				}
				if item.Auth.Env != nil {
					target.Auth.Env = item.Auth.Env
				}
				if item.Auth.File != nil {
					target.Auth.File = item.Auth.File
				}
			}
		}
	}
	if overlay.Env != nil {
		if next.Env == nil {
			next.Env = &EnvLayersPatch{}
		}
		if overlay.Env.GlobalDefined {
			next.Env.GlobalDefined = true
			next.Env.Global = mergeStringMap(next.Env.Global, overlay.Env.Global)
		}
		if overlay.Env.GatewayDefined {
			next.Env.GatewayDefined = true
			next.Env.Gateway = mergeStringMap(next.Env.Gateway, overlay.Env.Gateway)
		}
		if overlay.Env.WorkerDefined {
			next.Env.WorkerDefined = true
			next.Env.Worker = mergeStringMap(next.Env.Worker, overlay.Env.Worker)
		}
		if overlay.Env.OTDefined {
			next.Env.OTDefined = true
			next.Env.OT = mergeStringMap(next.Env.OT, overlay.Env.OT)
		}
	}
	return next
}

func diffScopeSettings(effective ScopeSettings, base ScopeSettings) ScopeSettings {
	diff := ScopeSettings{Version: effective.Version}
	if stringPtrValue(base.DefaultProvider) != stringPtrValue(effective.DefaultProvider) {
		diff.DefaultProvider = effective.DefaultProvider
	}
	if stringPtrValue(base.ApprovalPolicy) != stringPtrValue(effective.ApprovalPolicy) {
		diff.ApprovalPolicy = effective.ApprovalPolicy
	}
	if boolPtrValue(base.SelfDrivingMode) != boolPtrValue(effective.SelfDrivingMode) {
		diff.SelfDrivingMode = effective.SelfDrivingMode
	}
	if intPtrValue(base.ReactRalphIter) != intPtrValue(effective.ReactRalphIter) {
		diff.ReactRalphIter = effective.ReactRalphIter
	}
	if intPtrValue(base.PlanRalphIter) != intPtrValue(effective.PlanRalphIter) {
		diff.PlanRalphIter = effective.PlanRalphIter
	}
	if intPtrValue(base.CompactThresholdK) != intPtrValue(effective.CompactThresholdK) {
		diff.CompactThresholdK = effective.CompactThresholdK
	}
	for _, provider := range domain.Providers() {
		basePatch := providerPatch(base.Providers, provider)
		effectivePatch := providerPatch(effective.Providers, provider)
		if effectivePatch == nil {
			continue
		}
		needs := &ProviderSettingsPatch{}
		if stringPtrValue(basePatchValue(basePatch, "base_url")) != stringPtrValue(basePatchValue(effectivePatch, "base_url")) {
			needs.BaseURL = effectivePatch.BaseURL
		}
		if stringPtrValue(basePatchValue(basePatch, "model")) != stringPtrValue(basePatchValue(effectivePatch, "model")) {
			needs.Model = effectivePatch.Model
		}
		if authPatchValue(basePatch, "kind") != authPatchValue(effectivePatch, "kind") ||
			authPatchValue(basePatch, "env") != authPatchValue(effectivePatch, "env") ||
			authPatchValue(basePatch, "value") != authPatchValue(effectivePatch, "value") ||
			authPatchValue(basePatch, "file") != authPatchValue(effectivePatch, "file") {
			needs.Auth = effectivePatch.Auth
		}
		if !needs.isEmpty() {
			if diff.Providers == nil {
				diff.Providers = &ProviderCatalogPatch{}
			}
			setProviderPatch(diff.Providers, provider, needs)
		}
	}
	return diff
}

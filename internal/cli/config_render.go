package cli

import (
	"fmt"
	"strings"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
)

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
		case "auth.kind":
			return string(providerSettings.Auth.Kind), true
		case "auth.env":
			return providerSettings.Auth.Env, true
		case "auth.value":
			if strings.TrimSpace(providerSettings.Auth.Value) == "" {
				return "", true
			}
			return "<redacted>", true
		case "auth.file":
			return providerSettings.Auth.File, true
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
	raw := strings.TrimPrefix(string(key), "provider.")
	parts := strings.Split(raw, ".")
	if len(parts) < 2 {
		return "", "", false
	}
	provider, err := domain.ParseProvider(parts[0])
	if err != nil {
		return "", "", false
	}
	return provider, strings.Join(parts[1:], "."), true
}

func renderScopeEnvLines(scope config.ScopeSettings, scopeName config.Scope, file string, showOrigin bool) []string {
	lines := make([]string, 0, 16)
	if scope.Env == nil {
		return lines
	}
	appendLines := func(prefix string, values map[string]string, defined bool) {
		if !defined {
			return
		}
		if len(values) == 0 {
			lines = append(lines, formatConfigLine(config.SettingKey(prefix), "", config.SourceInfo{Scope: scopeName, File: file}, showOrigin))
			return
		}
		for key, value := range values {
			lines = append(lines, formatConfigLine(config.SettingKey(prefix+"."+key), value, config.SourceInfo{Scope: scopeName, File: file}, showOrigin))
		}
	}
	appendLines("env.global", scope.Env.Global, scope.Env.GlobalDefined)
	appendLines("env.gateway", scope.Env.Gateway, scope.Env.GatewayDefined)
	appendLines("env.worker", scope.Env.Worker, scope.Env.WorkerDefined)
	appendLines("env.ot", scope.Env.OT, scope.Env.OTDefined)
	return lines
}

func renderEnvLines(effective config.EffectiveEnv, showOrigin bool, projectFile string, globalFile string, scope config.Scope) []string {
	lines := make([]string, 0, 16)
	appendLines := func(prefix string, values map[string]string) {
		for key, value := range values {
			sourceFile := globalFile
			if projectFile != "" {
				sourceFile = projectFile
			}
			lines = append(lines, formatConfigLine(config.SettingKey(prefix+"."+key), value, config.SourceInfo{Scope: scope, File: sourceFile}, showOrigin))
		}
	}
	appendLines("env.global", effective.Global)
	appendLines("env.gateway", effective.Gateway)
	appendLines("env.worker", effective.Worker)
	appendLines("env.ot", effective.OT)
	return lines
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
	return patch == nil || (patch.BaseURL == nil && patch.Model == nil && patch.APIKeyEnv == nil && providerAuthEmpty(patch.Auth))
}

func providerAuthEmpty(patch *config.ProviderAuthPatch) bool {
	return patch == nil || (patch.Kind == nil && patch.Env == nil && patch.Value == nil && patch.File == nil)
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

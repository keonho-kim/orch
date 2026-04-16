package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	next := make(map[string]string, len(values))
	for key, value := range values {
		next[key] = value
	}
	return next
}

func applyAuthPatch(target *domain.ProviderSettings, patch *ProviderAuthPatch, file string) {
	if patch.Kind != nil {
		target.Auth.Kind = domain.ProviderAuthKind(strings.TrimSpace(*patch.Kind))
	}
	if patch.Value != nil {
		target.Auth.Value = strings.TrimSpace(*patch.Value)
	}
	if patch.Env != nil {
		target.Auth.Env = strings.TrimSpace(*patch.Env)
	}
	if patch.File != nil {
		target.Auth.File = strings.TrimSpace(*patch.File)
	}
	if strings.TrimSpace(file) != "" {
		target.Auth.BaseDir = filepath.Dir(file)
	}
}

func syncPatchAuth(patch *ProviderSettingsPatch) *ProviderAuthPatch {
	if patch == nil {
		return nil
	}
	if patch.Auth == nil && patch.APIKeyEnv != nil {
		patch.Auth = &ProviderAuthPatch{
			Kind: stringPtr(string(domain.ProviderAuthEnv)),
			Env:  patch.APIKeyEnv,
		}
	}
	if patch.Auth != nil && patch.Auth.Env != nil {
		patch.APIKeyEnv = patch.Auth.Env
	}
	return patch.Auth
}

func providerPatch(catalog *ProviderCatalogPatch, provider domain.Provider) *ProviderSettingsPatch {
	if catalog == nil {
		return nil
	}
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

func ensureProviderPatch(catalog *ProviderCatalogPatch, provider domain.Provider) *ProviderSettingsPatch {
	patch := providerPatch(catalog, provider)
	if patch != nil {
		return patch
	}
	patch = &ProviderSettingsPatch{}
	setProviderPatch(catalog, provider, patch)
	return patch
}

func setProviderPatch(catalog *ProviderCatalogPatch, provider domain.Provider, patch *ProviderSettingsPatch) {
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

func providerSettingsPtr(settings *domain.Settings, provider domain.Provider) *domain.ProviderSettings {
	switch provider {
	case domain.ProviderOllama:
		return &settings.Providers.Ollama
	case domain.ProviderVLLM:
		return &settings.Providers.VLLM
	case domain.ProviderGemini:
		return &settings.Providers.Gemini
	case domain.ProviderVertex:
		return &settings.Providers.Vertex
	case domain.ProviderBedrock:
		return &settings.Providers.Bedrock
	case domain.ProviderClaude:
		return &settings.Providers.Claude
	case domain.ProviderAzure:
		return &settings.Providers.Azure
	case domain.ProviderChatGPT:
		return &settings.Providers.ChatGPT
	default:
		return nil
	}
}

func stringValue(value *string) (string, bool) {
	if value == nil {
		return "", false
	}
	return strings.TrimSpace(*value), true
}

func boolValue(value *bool) (string, bool) {
	if value == nil {
		return "", false
	}
	return fmt.Sprintf("%t", *value), true
}

func intValue(value *int) (string, bool) {
	if value == nil {
		return "", false
	}
	return fmt.Sprintf("%d", *value), true
}

func parseBool(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("expected true or false, got %q", raw)
	}
}

func parseInt(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("expected integer")
	}
	var number int
	if _, err := fmt.Sscanf(value, "%d", &number); err != nil {
		return 0, fmt.Errorf("parse integer %q: %w", raw, err)
	}
	return number, nil
}

func stringPtr(value string) *string {
	return &value
}

func maybeStringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return stringPtr(value)
}

func boolPtr(value bool) *bool {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func boolPtrValue(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}

func intPtrValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func basePatchValue(patch *ProviderSettingsPatch, field string) *string {
	if patch == nil {
		return nil
	}
	switch field {
	case "base_url":
		return patch.BaseURL
	case "model":
		return patch.Model
	default:
		return nil
	}
}

func authPatchValue(patch *ProviderSettingsPatch, field string) string {
	if patch == nil || patch.Auth == nil {
		return ""
	}
	switch field {
	case "kind":
		return stringPtrValue(patch.Auth.Kind)
	case "env":
		return stringPtrValue(patch.Auth.Env)
	case "value":
		return stringPtrValue(patch.Auth.Value)
	case "file":
		return stringPtrValue(patch.Auth.File)
	default:
		return ""
	}
}

func legacyProviderPatch(catalog *legacyProviderCatalogPatch, provider domain.Provider) *legacyProviderSettingsPatch {
	if catalog == nil {
		return nil
	}
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

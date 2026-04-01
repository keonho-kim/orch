package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
)

func resolvedSettingsForModal(settings domain.Settings, projectScope config.Scope) config.ResolvedSettings {
	settings.Normalize()
	scopeFiles := map[config.Scope]string{
		config.ScopeManaged: "",
		config.ScopeUser:    "",
		config.ScopeProject: "orch.settings.json",
		config.ScopeLocal:   ".orch/settings.local.json",
	}
	scopes := map[config.Scope]config.ScopeSettings{
		config.ScopeManaged: {},
		config.ScopeUser:    {},
		config.ScopeProject: {},
		config.ScopeLocal:   {},
	}
	scopes[projectScope] = config.ScopeSettingsFromDomainSettings(settings)
	return config.ResolveSettings(scopes, scopeFiles)
}

func newSettingsFormStateForScope(resolved config.ResolvedSettings, scope config.Scope) settingsFormState {
	effective := resolved.Effective
	scopeSettings := resolved.Scopes[scope]

	state := settingsFormState{
		scope:          scope,
		resolved:       resolved,
		provider:       effective.DefaultProvider,
		selfDriving:    effective.SelfDrivingMode,
		focus:          fieldProvider,
		inputs:         make(map[settingsField]textinput.Model),
		providerSet:    false,
		selfDrivingSet: false,
	}
	if state.provider == "" {
		state.provider = domain.ProviderOllama
	}
	if raw, ok := scopeSettings.ValueForKey(config.KeyDefaultProvider); ok && strings.TrimSpace(raw) != "" {
		if provider, err := domain.ParseProvider(raw); err == nil {
			state.provider = provider
			state.providerSet = true
		}
	}
	if raw, ok := scopeSettings.ValueForKey(config.KeySelfDrivingMode); ok {
		state.selfDriving = raw == "true"
		state.selfDrivingSet = true
	}

	for _, field := range settingsFieldOrder {
		if !state.isTextField(field) {
			continue
		}
		key, ok := settingsFieldKey(field)
		if !ok {
			continue
		}
		raw, hasRaw := scopeSettings.ValueForKey(key)
		input := newSettingsInput(settingsFieldLabel(field), "")
		if hasRaw {
			input.SetValue(raw)
		}
		if effectiveValue, ok := effectiveSettingValue(effective, key); ok {
			input.Placeholder = effectiveValue
		}
		state.inputs[field] = input
	}

	state.focusField(state.focus)
	return state
}

func (s settingsModalState) isEditableScope() bool {
	return s.scope == config.ScopeUser || s.scope == config.ScopeProject || s.scope == config.ScopeLocal
}

func (s *settingsModalState) setScope(scope config.Scope) {
	s.scope = scope
	s.form = newSettingsFormStateForScope(s.resolved, scope)
}

func nextSettingsScope(current config.Scope, step int) config.Scope {
	if len(settingsScopeOrder) == 0 {
		return current
	}
	index := 0
	for scopeIndex, scope := range settingsScopeOrder {
		if scope == current {
			index = scopeIndex
			break
		}
	}
	index = (index + step) % len(settingsScopeOrder)
	if index < 0 {
		index += len(settingsScopeOrder)
	}
	return settingsScopeOrder[index]
}

func (s settingsFormState) isEditable() bool {
	return s.scope == config.ScopeUser || s.scope == config.ScopeProject || s.scope == config.ScopeLocal
}

func (s settingsFormState) buildScopeSettings() config.ScopeSettings {
	patch := config.ScopeSettings{}
	if s.providerSet {
		value := s.provider.String()
		patch.DefaultProvider = &value
	}
	if s.selfDrivingSet {
		value := s.selfDriving
		patch.SelfDrivingMode = &value
	}
	for _, field := range settingsFieldOrder {
		if !s.isTextField(field) {
			continue
		}
		key, ok := settingsFieldKey(field)
		if !ok {
			continue
		}
		value := strings.TrimSpace(s.inputs[field].Value())
		if value == "" {
			continue
		}
		_ = patch.SetKey(key, value)
	}
	return patch
}

func (s settingsFormState) previewResolved() config.ResolvedSettings {
	scopes := make(map[config.Scope]config.ScopeSettings, len(s.resolved.Scopes))
	for scope, settings := range s.resolved.Scopes {
		scopes[scope] = settings
	}
	scopes[s.scope] = s.buildScopeSettings()

	files := make(map[config.Scope]string, len(s.resolved.Files))
	for scope, file := range s.resolved.Files {
		files[scope] = file
	}
	return config.ResolveSettings(scopes, files)
}

func (s settingsFormState) fieldLocked(field settingsField) bool {
	if !s.isEditable() {
		return false
	}
	key, ok := settingsFieldKey(field)
	if !ok {
		return false
	}
	source, ok := s.resolved.Sources[key]
	return ok && source.Scope == config.ScopeManaged
}

func (s settingsFormState) displayProvider() domain.Provider {
	return s.provider
}

func (s settingsFormState) displaySelfDriving() bool {
	return s.selfDriving
}

func (s settingsFormState) fieldDisplayValue(field settingsField) string {
	switch settingsFieldSpecs[field].kind {
	case settingsFieldKindProvider:
		label := s.displayProvider().DisplayName()
		if s.isEditable() && !s.providerSet {
			return "Inherited: " + label
		}
		return label
	case settingsFieldKindToggle:
		value := onOffLabel(s.displaySelfDriving())
		if s.isEditable() && !s.selfDrivingSet {
			return "Inherited: " + value
		}
		return value
	default:
		input := s.inputs[field]
		value := strings.TrimSpace(input.Value())
		if value != "" {
			return value
		}
		if strings.TrimSpace(input.Placeholder) == "" {
			return ""
		}
		if s.isEditable() {
			return "Inherited: " + input.Placeholder
		}
		return input.Placeholder
	}
}

func (s *settingsFormState) unsetFocusedField() string {
	switch s.focus {
	case fieldProvider:
		s.providerSet = false
		s.provider = s.resolved.Effective.DefaultProvider
		if s.provider == "" {
			s.provider = domain.ProviderOllama
		}
		return "Provider now inherits from lower scopes."
	case fieldSelfDriving:
		s.selfDrivingSet = false
		s.selfDriving = s.resolved.Effective.SelfDrivingMode
		return "Self-driving mode now inherits from lower scopes."
	default:
		if !s.isTextField(s.focus) {
			return ""
		}
		input := s.inputs[s.focus]
		input.SetValue("")
		s.inputs[s.focus] = input
		return fmt.Sprintf("%s now inherits from lower scopes.", settingsFieldLabel(s.focus))
	}
}

func settingsFieldKey(field settingsField) (config.SettingKey, bool) {
	switch field {
	case fieldProvider:
		return config.KeyDefaultProvider, true
	case fieldSelfDriving:
		return config.KeySelfDrivingMode, true
	case fieldOllamaBaseURL:
		return config.ProviderBaseURLKey(domain.ProviderOllama), true
	case fieldOllamaModel:
		return config.ProviderModelKey(domain.ProviderOllama), true
	case fieldVLLMBaseURL:
		return config.ProviderBaseURLKey(domain.ProviderVLLM), true
	case fieldVLLMModel:
		return config.ProviderModelKey(domain.ProviderVLLM), true
	case fieldVLLMAPIKeyEnv:
		return config.ProviderAPIKeyEnvKey(domain.ProviderVLLM), true
	case fieldGeminiBaseURL:
		return config.ProviderBaseURLKey(domain.ProviderGemini), true
	case fieldGeminiModel:
		return config.ProviderModelKey(domain.ProviderGemini), true
	case fieldGeminiAPIKeyEnv:
		return config.ProviderAPIKeyEnvKey(domain.ProviderGemini), true
	case fieldVertexBaseURL:
		return config.ProviderBaseURLKey(domain.ProviderVertex), true
	case fieldVertexModel:
		return config.ProviderModelKey(domain.ProviderVertex), true
	case fieldVertexAPIKeyEnv:
		return config.ProviderAPIKeyEnvKey(domain.ProviderVertex), true
	case fieldBedrockBaseURL:
		return config.ProviderBaseURLKey(domain.ProviderBedrock), true
	case fieldBedrockModel:
		return config.ProviderModelKey(domain.ProviderBedrock), true
	case fieldBedrockAPIKeyEnv:
		return config.ProviderAPIKeyEnvKey(domain.ProviderBedrock), true
	case fieldClaudeBaseURL:
		return config.ProviderBaseURLKey(domain.ProviderClaude), true
	case fieldClaudeModel:
		return config.ProviderModelKey(domain.ProviderClaude), true
	case fieldClaudeAPIKeyEnv:
		return config.ProviderAPIKeyEnvKey(domain.ProviderClaude), true
	case fieldAzureBaseURL:
		return config.ProviderBaseURLKey(domain.ProviderAzure), true
	case fieldAzureModel:
		return config.ProviderModelKey(domain.ProviderAzure), true
	case fieldAzureAPIKeyEnv:
		return config.ProviderAPIKeyEnvKey(domain.ProviderAzure), true
	case fieldChatGPTBaseURL:
		return config.ProviderBaseURLKey(domain.ProviderChatGPT), true
	case fieldChatGPTModel:
		return config.ProviderModelKey(domain.ProviderChatGPT), true
	case fieldChatGPTAPIKeyEnv:
		return config.ProviderAPIKeyEnvKey(domain.ProviderChatGPT), true
	case fieldReactRalphIter:
		return config.KeyReactRalphIter, true
	case fieldPlanRalphIter:
		return config.KeyPlanRalphIter, true
	case fieldCompactThreshold:
		return config.KeyCompactThresholdK, true
	default:
		return "", false
	}
}

func settingsScopeLabel(scope config.Scope) string {
	switch scope {
	case config.ScopeEffective:
		return "Effective"
	case config.ScopeManaged:
		return "Managed"
	case config.ScopeUser:
		return "User"
	case config.ScopeProject:
		return "Project"
	case config.ScopeLocal:
		return "Local"
	default:
		return strings.Title(string(scope))
	}
}

func effectiveSettingValue(settings domain.Settings, key config.SettingKey) (string, bool) {
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
		provider, attr, ok := parseProviderConfigKeyForUI(key)
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

func parseProviderConfigKeyForUI(key config.SettingKey) (domain.Provider, string, bool) {
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

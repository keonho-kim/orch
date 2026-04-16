package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
)

func newSettingsFormStateFromConfig(state config.ConfigState) settingsFormState {
	settings := state.Settings
	settings.Normalize()

	form := settingsFormState{
		configState:  state,
		provider:     settings.DefaultProvider,
		selfDriving:  settings.SelfDrivingMode,
		focus:        fieldProvider,
		inputs:       make(map[settingsField]textinput.Model),
		confirmation: nil,
	}
	if form.provider == "" {
		form.provider = domain.ProviderOllama
	}

	for _, field := range settingsFieldOrder {
		if !form.isTextField(field) {
			continue
		}
		input := newSettingsInput(settingsFieldLabel(field), settingsFieldValue(settings, field))
		form.inputs[field] = input
	}

	form.focusField(form.focus)
	return form
}

<<<<<<< HEAD
func (s settingsFormState) buildSettings(base domain.Settings) domain.Settings {
	settings := base
	settings.Normalize()
	settings.DefaultProvider = s.provider
	settings.SelfDrivingMode = s.selfDriving
	settings.ReactRalphIter = parsePositiveInt(s.inputs[fieldReactRalphIter].Value(), settings.ReactRalphIter)
	settings.PlanRalphIter = parsePositiveInt(s.inputs[fieldPlanRalphIter].Value(), settings.PlanRalphIter)
	settings.CompactThresholdK = parsePositiveInt(s.inputs[fieldCompactThreshold].Value(), settings.CompactThresholdK)

	for _, provider := range domain.Providers() {
		target := settings.Providers.Provider(provider)
		target.Endpoint = strings.TrimSpace(s.inputs[providerField(provider, providerFieldKindEndpoint)].Value())
		target.Model = strings.TrimSpace(s.inputs[providerField(provider, providerFieldKindModel)].Value())
		target.APIKey = strings.TrimSpace(s.inputs[providerField(provider, providerFieldKindAPIKey)].Value())
		target.Reasoning = strings.TrimSpace(s.inputs[providerField(provider, providerFieldKindReasoning)].Value())
	}

	settings.Normalize()
	return settings
}

func settingsFieldValue(settings domain.Settings, field settingsField) string {
=======
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
>>>>>>> cef7a8c (update)
	switch field {
	case fieldReactRalphIter:
		return integerFieldValue(settings.ReactRalphIter)
	case fieldPlanRalphIter:
		return integerFieldValue(settings.PlanRalphIter)
	case fieldCompactThreshold:
		return integerFieldValue(settings.CompactThresholdK)
	}

	provider, kind, ok := providerFieldKind(field)
	if !ok {
		return ""
	}
	config := settings.ConfigFor(provider)
	switch kind {
	case providerFieldKindEndpoint:
		return config.Endpoint
	case providerFieldKindModel:
		return config.Model
	case providerFieldKindAPIKey:
		return config.APIKey
	case providerFieldKindReasoning:
		return config.Reasoning
	default:
		return ""
	}
}

<<<<<<< HEAD
func providerFieldKind(field settingsField) (domain.Provider, providerSettingFieldKind, bool) {
	spec, ok := settingsFieldSpecs[field]
	if !ok || spec.provider == "" {
=======
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
		value := string(scope)
		if value == "" {
			return ""
		}
		return strings.ToUpper(value[:1]) + value[1:]
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
>>>>>>> cef7a8c (update)
		return "", "", false
	}
	return spec.provider, spec.providerFieldKind, true
}

func integerFieldValue(value int) string {
	return strconv.Itoa(value)
}

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

func providerFieldKind(field settingsField) (domain.Provider, providerSettingFieldKind, bool) {
	spec, ok := settingsFieldSpecs[field]
	if !ok || spec.provider == "" {
		return "", "", false
	}
	return spec.provider, spec.providerFieldKind, true
}

func integerFieldValue(value int) string {
	return strconv.Itoa(value)
}

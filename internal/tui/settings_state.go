package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
)

type settingsField string
type settingsMode int
type settingsSetupStep int
type settingsFieldKind int
type settingsFieldGroup int
type ollamaURLMode int
type providerSettingFieldKind string

const (
	fieldProvider         settingsField = "provider"
	fieldSelfDriving      settingsField = "self_driving"
	fieldReactRalphIter   settingsField = "react_ralph_iter"
	fieldPlanRalphIter    settingsField = "plan_ralph_iter"
	fieldCompactThreshold settingsField = "compact_threshold"
)

const (
	settingsModeForm settingsMode = iota
	settingsModeSetup
)

const (
	settingsSetupStepProvider settingsSetupStep = iota
	settingsSetupStepOllamaURL
	settingsSetupStepOllamaModel
)

const (
	settingsFieldKindProvider settingsFieldKind = iota
	settingsFieldKindToggle
	settingsFieldKindText
)

const (
	settingsFieldGroupGeneral settingsFieldGroup = iota
	settingsFieldGroupProvider
	settingsFieldGroupRalph
)

const (
	ollamaURLDefault ollamaURLMode = iota
	ollamaURLCustom
)

const (
	providerFieldKindEndpoint  providerSettingFieldKind = "endpoint"
	providerFieldKindModel     providerSettingFieldKind = "model"
	providerFieldKindAPIKey    providerSettingFieldKind = "api_key"
	providerFieldKindReasoning providerSettingFieldKind = "reasoning"
)

type settingsFieldSpec struct {
	label             string
	kind              settingsFieldKind
	group             settingsFieldGroup
	provider          domain.Provider
	providerFieldKind providerSettingFieldKind
}

type providerChangeConfirmation struct {
	pendingProvider domain.Provider
}

type settingsFormState struct {
	configState  config.ConfigState
	provider     domain.Provider
	selfDriving  bool
	focus        settingsField
	inputs       map[settingsField]textinput.Model
	confirmation *providerChangeConfirmation
}

type settingsSetupState struct {
	step       settingsSetupStep
	urlMode    ollamaURLMode
	urlInput   textinput.Model
	checking   bool
	models     []string
	modelIndex int
	baseURL    string
}

type settingsModalState struct {
	visible     bool
	mode        settingsMode
	configState config.ConfigState
	form        settingsFormState
	setup       settingsSetupState
}

var settingsFieldOrder = buildSettingsFieldOrder()

var settingsFieldGroups = []settingsFieldGroup{
	settingsFieldGroupGeneral,
	settingsFieldGroupProvider,
	settingsFieldGroupRalph,
}

var settingsFieldGroupTitles = map[settingsFieldGroup]string{
	settingsFieldGroupGeneral:  "GENERAL",
	settingsFieldGroupProvider: "PROVIDER",
	settingsFieldGroupRalph:    "RALPH LOOP",
}

var settingsFieldSpecs = buildSettingsFieldSpecs()

func buildSettingsFieldOrder() []settingsField {
	order := []settingsField{
		fieldSelfDriving,
		fieldProvider,
	}
	for _, provider := range domain.Providers() {
		order = append(order,
			providerField(provider, providerFieldKindEndpoint),
			providerField(provider, providerFieldKindModel),
			providerField(provider, providerFieldKindAPIKey),
			providerField(provider, providerFieldKindReasoning),
		)
	}
	order = append(order,
		fieldReactRalphIter,
		fieldPlanRalphIter,
		fieldCompactThreshold,
	)
	return order
}

func buildSettingsFieldSpecs() map[settingsField]settingsFieldSpec {
	specs := map[settingsField]settingsFieldSpec{
		fieldProvider:         {label: "Provider", kind: settingsFieldKindProvider, group: settingsFieldGroupProvider},
		fieldSelfDriving:      {label: "Self-Driving Mode", kind: settingsFieldKindToggle, group: settingsFieldGroupGeneral},
		fieldReactRalphIter:   {label: "ReAct Ralph Iterations", kind: settingsFieldKindText, group: settingsFieldGroupRalph},
		fieldPlanRalphIter:    {label: "Plan Ralph Iterations", kind: settingsFieldKindText, group: settingsFieldGroupRalph},
		fieldCompactThreshold: {label: "Compact Threshold (k)", kind: settingsFieldKindText, group: settingsFieldGroupGeneral},
	}

	for _, provider := range domain.Providers() {
		for _, kind := range providerFieldKinds() {
			specs[providerField(provider, kind)] = settingsFieldSpec{
				label:             providerFieldLabel(provider, kind),
				kind:              settingsFieldKindText,
				group:             settingsFieldGroupProvider,
				provider:          provider,
				providerFieldKind: kind,
			}
		}
	}

	return specs
}

func providerFieldKinds() []providerSettingFieldKind {
	return []providerSettingFieldKind{
		providerFieldKindEndpoint,
		providerFieldKindModel,
		providerFieldKindAPIKey,
		providerFieldKindReasoning,
	}
}

func providerField(provider domain.Provider, kind providerSettingFieldKind) settingsField {
	return settingsField("provider:" + provider.String() + ":" + string(kind))
}

func providerFieldLabel(provider domain.Provider, kind providerSettingFieldKind) string {
	switch kind {
	case providerFieldKindEndpoint:
		return provider.DisplayName() + " Endpoint"
	case providerFieldKindModel:
		return provider.DisplayName() + " Model"
	case providerFieldKindAPIKey:
		return provider.DisplayName() + " API Key"
	case providerFieldKindReasoning:
		return provider.DisplayName() + " Reasoning"
	default:
		return provider.DisplayName()
	}
}

func primaryProviderFieldKind(provider domain.Provider) providerSettingFieldKind {
	switch provider {
	case domain.ProviderGemini, domain.ProviderVertex, domain.ProviderClaude, domain.ProviderChatGPT:
		return providerFieldKindModel
	case domain.ProviderOllama, domain.ProviderVLLM, domain.ProviderBedrock, domain.ProviderAzure:
		return providerFieldKindEndpoint
	default:
		return providerFieldKindEndpoint
	}
}

func newSettingsModal(settings domain.Settings) settingsModalState {
	state := config.ConfigState{
		Document: config.DocumentFromSettings(settings),
		Settings: settings,
	}
	return newSettingsModalFromState(state)
}

func newSettingsModalFromState(state config.ConfigState) settingsModalState {
	return settingsModalState{
		mode:        settingsModeForm,
		configState: state,
		form:        newSettingsFormStateFromConfig(state),
		setup:       newSettingsSetupState(state.Settings),
	}
}

func newSetupSettingsModal(settings domain.Settings) settingsModalState {
	state := config.ConfigState{
		Document: config.DocumentFromSettings(settings),
		Settings: settings,
	}
	return newSetupSettingsModalFromState(state)
}

func newSetupSettingsModalFromState(state config.ConfigState) settingsModalState {
	modal := newSettingsModalFromState(state)
	modal.visible = true
	modal.mode = settingsModeSetup
	return modal
}

func newSettingsFormState(settings domain.Settings) settingsFormState {
	return newSettingsFormStateFromConfig(config.ConfigState{
		Document: config.DocumentFromSettings(settings),
		Settings: settings,
	})
}

func newSettingsSetupState(settings domain.Settings) settingsSetupState {
	settings.Normalize()
	urlInput := newSettingsInput("Ollama Endpoint", settings.Providers.Ollama.Endpoint)
	if strings.TrimSpace(urlInput.Value()) == "" {
		urlInput.SetValue("http://localhost:11434/v1")
	}

	return settingsSetupState{
		step:     settingsSetupStepProvider,
		urlMode:  ollamaURLDefault,
		urlInput: urlInput,
		baseURL:  settings.Providers.Ollama.Endpoint,
	}
}

func newSettingsInput(placeholder string, value string) textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = placeholder
	input.SetValue(value)
	input.CharLimit = 0
	return input
}

func (s *settingsModalState) resize(width int) {
	s.form.resizeInputs(width)
	s.setup.urlInput.Width = width
}

func (s *settingsModalState) useFormMode() {
	s.mode = settingsModeForm
	s.form.ensureVisibleFocus()
}

func (s *settingsModalState) useSetupStep(step settingsSetupStep) {
	s.mode = settingsModeSetup
	s.setup.step = step
}

func (s settingsModalState) isSetup() bool {
	return s.mode == settingsModeSetup
}

func (s settingsFormState) spec(field settingsField) settingsFieldSpec {
	return settingsFieldSpecs[field]
}

func (s settingsFormState) isTextField(field settingsField) bool {
	return s.spec(field).kind == settingsFieldKindText
}

func (s settingsFormState) visibleFields() []settingsField {
	return append([]settingsField(nil), settingsFieldOrder...)
}

func (s *settingsFormState) resizeInputs(width int) {
	for field, input := range s.inputs {
		input.Width = width
		s.inputs[field] = input
	}
}

func (s *settingsFormState) blurInputs() {
	for field, input := range s.inputs {
		input.Blur()
		s.inputs[field] = input
	}
}

func (s *settingsFormState) focusNext() {
	order := s.visibleFields()
	if len(order) == 0 {
		return
	}
	for index, field := range order {
		if field != s.focus {
			continue
		}
		s.focusField(order[(index+1)%len(order)])
		return
	}
	s.focusField(order[0])
}

func (s *settingsFormState) focusPrev() {
	order := s.visibleFields()
	if len(order) == 0 {
		return
	}
	for index, field := range order {
		if field != s.focus {
			continue
		}
		prev := index - 1
		if prev < 0 {
			prev = len(order) - 1
		}
		s.focusField(order[prev])
		return
	}
	s.focusField(order[0])
}

func (s *settingsFormState) focusField(field settingsField) {
	s.blurInputs()
	if s.isTextField(field) {
		input := s.inputs[field]
		input.Focus()
		s.inputs[field] = input
	}
	s.focus = field
}

func (s *settingsFormState) ensureVisibleFocus() {
	for _, field := range s.visibleFields() {
		if field == s.focus {
			s.focusField(field)
			return
		}
	}
	order := s.visibleFields()
	if len(order) > 0 {
		s.focusField(order[0])
	}
}

func (s *settingsFormState) beginProviderConfirmation(target domain.Provider) {
	s.confirmation = &providerChangeConfirmation{pendingProvider: target}
}

func (s *settingsFormState) cancelProviderConfirmation() {
	s.confirmation = nil
}

func (s settingsFormState) hasProviderConfirmation() bool {
	return s.confirmation != nil
}

func (s settingsFormState) pendingProvider() domain.Provider {
	if s.confirmation == nil {
		return ""
	}
	return s.confirmation.pendingProvider
}

func (s *settingsFormState) setProvider(provider domain.Provider) {
	s.provider = provider
	s.ensureVisibleFocus()
}

func (s settingsFormState) providerPrimaryField(provider domain.Provider) settingsField {
	return providerField(provider, primaryProviderFieldKind(provider))
}

func (s settingsFormState) missingProviderFields(base domain.Settings, provider domain.Provider) []string {
	return s.buildSettings(base).MissingProviderFields(provider)
}

func (s settingsFormState) fieldLocked(settingsField) bool {
	return false
}

func (s settingsFormState) displayProvider() domain.Provider {
	return s.provider
}

func (s settingsFormState) displaySelfDriving() bool {
	return s.selfDriving
}

func describeMissingProviderFields(provider domain.Provider, missing []string) string {
	labels := make([]string, 0, len(missing))
	for _, field := range missing {
		switch field {
		case "Endpoint":
			labels = append(labels, provider.DisplayName()+" Endpoint")
		case "Model":
			labels = append(labels, provider.DisplayName()+" Model")
		case "API Key":
			labels = append(labels, provider.DisplayName()+" API Key")
		default:
			labels = append(labels, field)
		}
	}
	return strings.Join(labels, ", ")
}

func describeMissingProviderConfiguration(provider domain.Provider, missing []string) string {
	if len(missing) == 1 {
		switch missing[0] {
		case "Endpoint":
			return "the " + provider.DisplayName() + " Endpoint"
		case "Model":
			return "the " + provider.DisplayName() + " Model"
		case "API Key":
			return "the " + provider.DisplayName() + " API Key"
		}
	}
	return describeMissingProviderFields(provider, missing)
}

func nextProvider(current domain.Provider, step int) domain.Provider {
	providers := domain.Providers()
	if len(providers) == 0 {
		return current
	}
	index := 0
	for providerIndex, provider := range providers {
		if provider == current {
			index = providerIndex
			break
		}
	}
	index = (index + step) % len(providers)
	if index < 0 {
		index += len(providers)
	}
	return providers[index]
}

func settingsFieldsForGroup(group settingsFieldGroup) []settingsField {
	fields := make([]settingsField, 0, len(settingsFieldOrder))
	for _, field := range settingsFieldOrder {
		if settingsFieldSpecs[field].group != group {
			continue
		}
		fields = append(fields, field)
	}
	return fields
}

func settingsFieldLabel(field settingsField) string {
	return settingsFieldSpecs[field].label
}

func parsePositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

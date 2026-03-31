package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/keonho-kim/orch/domain"
)

type settingsField int
type settingsMode int
type settingsSetupStep int
type settingsFieldKind int
type settingsFieldGroup int
type ollamaURLMode int

const (
	fieldProvider settingsField = iota
	fieldSelfDriving
	fieldOllamaBaseURL
	fieldOllamaModel
	fieldVLLMBaseURL
	fieldVLLMModel
	fieldVLLMAPIKeyEnv
	fieldReactRalphIter
	fieldPlanRalphIter
	fieldCompactThreshold
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

type settingsFieldSpec struct {
	label   string
	kind    settingsFieldKind
	group   settingsFieldGroup
	visible func(provider domain.Provider) bool
}

type providerChangeConfirmation struct {
	pendingProvider domain.Provider
}

type settingsFormState struct {
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
	visible bool
	mode    settingsMode
	form    settingsFormState
	setup   settingsSetupState
}

var settingsFieldOrder = []settingsField{
	fieldSelfDriving,
	fieldProvider,
	fieldOllamaBaseURL,
	fieldOllamaModel,
	fieldVLLMBaseURL,
	fieldVLLMModel,
	fieldVLLMAPIKeyEnv,
	fieldReactRalphIter,
	fieldPlanRalphIter,
	fieldCompactThreshold,
}

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

var settingsFieldSpecs = map[settingsField]settingsFieldSpec{
	fieldProvider:         {label: "Provider", kind: settingsFieldKindProvider, group: settingsFieldGroupProvider, visible: alwaysVisibleSettingsField},
	fieldSelfDriving:      {label: "Self-Driving Mode", kind: settingsFieldKindToggle, group: settingsFieldGroupGeneral, visible: alwaysVisibleSettingsField},
	fieldOllamaBaseURL:    {label: "Ollama Base URL", kind: settingsFieldKindText, group: settingsFieldGroupProvider, visible: providerSettingsField(domain.ProviderOllama)},
	fieldOllamaModel:      {label: "Ollama Model", kind: settingsFieldKindText, group: settingsFieldGroupProvider, visible: providerSettingsField(domain.ProviderOllama)},
	fieldVLLMBaseURL:      {label: "vLLM Base URL", kind: settingsFieldKindText, group: settingsFieldGroupProvider, visible: providerSettingsField(domain.ProviderVLLM)},
	fieldVLLMModel:        {label: "vLLM Model", kind: settingsFieldKindText, group: settingsFieldGroupProvider, visible: providerSettingsField(domain.ProviderVLLM)},
	fieldVLLMAPIKeyEnv:    {label: "vLLM API Key Env", kind: settingsFieldKindText, group: settingsFieldGroupProvider, visible: providerSettingsField(domain.ProviderVLLM)},
	fieldReactRalphIter:   {label: "ReAct Ralph Iterations", kind: settingsFieldKindText, group: settingsFieldGroupRalph, visible: alwaysVisibleSettingsField},
	fieldPlanRalphIter:    {label: "Plan Ralph Iterations", kind: settingsFieldKindText, group: settingsFieldGroupRalph, visible: alwaysVisibleSettingsField},
	fieldCompactThreshold: {label: "Compact Threshold (k)", kind: settingsFieldKindText, group: settingsFieldGroupGeneral, visible: alwaysVisibleSettingsField},
}

func alwaysVisibleSettingsField(domain.Provider) bool {
	return true
}

func providerSettingsField(provider domain.Provider) func(domain.Provider) bool {
	return func(current domain.Provider) bool {
		return current == provider
	}
}

func newSettingsModal(settings domain.Settings) settingsModalState {
	return settingsModalState{
		mode:  settingsModeForm,
		form:  newSettingsFormState(settings),
		setup: newSettingsSetupState(settings),
	}
}

func newSetupSettingsModal(settings domain.Settings) settingsModalState {
	modal := newSettingsModal(settings)
	modal.visible = true
	modal.mode = settingsModeSetup
	return modal
}

func newSettingsFormState(settings domain.Settings) settingsFormState {
	settings.Normalize()
	state := settingsFormState{
		provider:    settings.DefaultProvider,
		selfDriving: settings.SelfDrivingMode,
		focus:       fieldProvider,
		inputs:      make(map[settingsField]textinput.Model),
	}
	if state.provider == "" {
		state.provider = domain.ProviderOllama
	}

	state.inputs[fieldOllamaBaseURL] = newSettingsInput("Ollama Base URL", settings.Providers.Ollama.BaseURL)
	state.inputs[fieldOllamaModel] = newSettingsInput("Ollama Model", settings.Providers.Ollama.Model)
	state.inputs[fieldVLLMBaseURL] = newSettingsInput("vLLM Base URL", settings.Providers.VLLM.BaseURL)
	state.inputs[fieldVLLMModel] = newSettingsInput("vLLM Model", settings.Providers.VLLM.Model)
	state.inputs[fieldVLLMAPIKeyEnv] = newSettingsInput("vLLM API Key Env", settings.Providers.VLLM.APIKeyEnv)
	state.inputs[fieldReactRalphIter] = newSettingsInput("ReAct Ralph Iterations", strconv.Itoa(settings.ReactRalphIter))
	state.inputs[fieldPlanRalphIter] = newSettingsInput("Plan Ralph Iterations", strconv.Itoa(settings.PlanRalphIter))
	state.inputs[fieldCompactThreshold] = newSettingsInput("Compact Threshold (k)", strconv.Itoa(settings.CompactThresholdK))
	state.focusField(state.focus)

	return state
}

func newSettingsSetupState(settings domain.Settings) settingsSetupState {
	settings.Normalize()
	urlInput := newSettingsInput("Ollama Base URL", settings.Providers.Ollama.BaseURL)
	if strings.TrimSpace(urlInput.Value()) == "" {
		urlInput.SetValue("http://localhost:11434/v1")
	}

	return settingsSetupState{
		step:     settingsSetupStepProvider,
		urlMode:  ollamaURLDefault,
		urlInput: urlInput,
		baseURL:  settings.Providers.Ollama.BaseURL,
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
	fields := make([]settingsField, 0, len(settingsFieldOrder))
	for _, field := range settingsFieldOrder {
		spec := s.spec(field)
		if spec.visible != nil && !spec.visible(s.provider) {
			continue
		}
		fields = append(fields, field)
	}
	return fields
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

func (s settingsFormState) providerModelField(provider domain.Provider) settingsField {
	switch provider {
	case domain.ProviderVLLM:
		return fieldVLLMModel
	default:
		return fieldOllamaModel
	}
}

func (s settingsFormState) providerHasModel(provider domain.Provider) bool {
	field := s.providerModelField(provider)
	return strings.TrimSpace(s.inputs[field].Value()) != ""
}

func (s settingsFormState) buildSettings(base domain.Settings) domain.Settings {
	settings := base
	settings.DefaultProvider = s.provider
	settings.ApprovalPolicy = domain.ApprovalConfirmMutations
	settings.Providers.Ollama.BaseURL = strings.TrimSpace(s.inputs[fieldOllamaBaseURL].Value())
	settings.Providers.Ollama.Model = strings.TrimSpace(s.inputs[fieldOllamaModel].Value())
	settings.Providers.VLLM.BaseURL = strings.TrimSpace(s.inputs[fieldVLLMBaseURL].Value())
	settings.Providers.VLLM.Model = strings.TrimSpace(s.inputs[fieldVLLMModel].Value())
	settings.Providers.VLLM.APIKeyEnv = strings.TrimSpace(s.inputs[fieldVLLMAPIKeyEnv].Value())
	settings.SelfDrivingMode = s.selfDriving
	settings.ReactRalphIter = parsePositiveInt(s.inputs[fieldReactRalphIter].Value(), settings.ReactRalphIter)
	settings.PlanRalphIter = parsePositiveInt(s.inputs[fieldPlanRalphIter].Value(), settings.PlanRalphIter)
	settings.CompactThresholdK = parsePositiveInt(s.inputs[fieldCompactThreshold].Value(), settings.CompactThresholdK)
	settings.Normalize()
	return settings
}

func settingsFieldsForGroup(provider domain.Provider, group settingsFieldGroup) []settingsField {
	fields := make([]settingsField, 0, len(settingsFieldOrder))
	for _, field := range settingsFieldOrder {
		spec := settingsFieldSpecs[field]
		if spec.group != group {
			continue
		}
		if spec.visible != nil && !spec.visible(provider) {
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

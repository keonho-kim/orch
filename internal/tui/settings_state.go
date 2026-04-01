package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
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
	fieldGeminiBaseURL
	fieldGeminiModel
	fieldGeminiAPIKeyEnv
	fieldVertexBaseURL
	fieldVertexModel
	fieldVertexAPIKeyEnv
	fieldBedrockBaseURL
	fieldBedrockModel
	fieldBedrockAPIKeyEnv
	fieldClaudeBaseURL
	fieldClaudeModel
	fieldClaudeAPIKeyEnv
	fieldAzureBaseURL
	fieldAzureModel
	fieldAzureAPIKeyEnv
	fieldChatGPTBaseURL
	fieldChatGPTModel
	fieldChatGPTAPIKeyEnv
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
	label string
	kind  settingsFieldKind
	group settingsFieldGroup
}

type providerChangeConfirmation struct {
	pendingProvider domain.Provider
}

type settingsFormState struct {
	scope          config.Scope
	resolved       config.ResolvedSettings
	provider       domain.Provider
	providerSet    bool
	selfDriving    bool
	selfDrivingSet bool
	focus          settingsField
	inputs         map[settingsField]textinput.Model
	confirmation   *providerChangeConfirmation
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
	visible  bool
	mode     settingsMode
	scope    config.Scope
	resolved config.ResolvedSettings
	form     settingsFormState
	setup    settingsSetupState
}

var settingsFieldOrder = []settingsField{
	fieldSelfDriving,
	fieldProvider,
	fieldOllamaBaseURL,
	fieldOllamaModel,
	fieldVLLMBaseURL,
	fieldVLLMModel,
	fieldVLLMAPIKeyEnv,
	fieldGeminiBaseURL,
	fieldGeminiModel,
	fieldGeminiAPIKeyEnv,
	fieldVertexBaseURL,
	fieldVertexModel,
	fieldVertexAPIKeyEnv,
	fieldBedrockBaseURL,
	fieldBedrockModel,
	fieldBedrockAPIKeyEnv,
	fieldClaudeBaseURL,
	fieldClaudeModel,
	fieldClaudeAPIKeyEnv,
	fieldAzureBaseURL,
	fieldAzureModel,
	fieldAzureAPIKeyEnv,
	fieldChatGPTBaseURL,
	fieldChatGPTModel,
	fieldChatGPTAPIKeyEnv,
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
	fieldProvider:         {label: "Provider", kind: settingsFieldKindProvider, group: settingsFieldGroupProvider},
	fieldSelfDriving:      {label: "Self-Driving Mode", kind: settingsFieldKindToggle, group: settingsFieldGroupGeneral},
	fieldOllamaBaseURL:    {label: "Ollama Base URL", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldOllamaModel:      {label: "Ollama Model", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldVLLMBaseURL:      {label: "vLLM Base URL", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldVLLMModel:        {label: "vLLM Model", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldVLLMAPIKeyEnv:    {label: "vLLM API Key Env", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldGeminiBaseURL:    {label: "Gemini Base URL", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldGeminiModel:      {label: "Gemini Model", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldGeminiAPIKeyEnv:  {label: "Gemini API Key Env", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldVertexBaseURL:    {label: "Vertex Base URL", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldVertexModel:      {label: "Vertex Model", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldVertexAPIKeyEnv:  {label: "Vertex API Key Env", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldBedrockBaseURL:   {label: "Bedrock Base URL", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldBedrockModel:     {label: "Bedrock Model", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldBedrockAPIKeyEnv: {label: "Bedrock API Key Env", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldClaudeBaseURL:    {label: "Claude Base URL", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldClaudeModel:      {label: "Claude Model", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldClaudeAPIKeyEnv:  {label: "Claude API Key Env", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldAzureBaseURL:     {label: "Azure Base URL", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldAzureModel:       {label: "Azure Model", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldAzureAPIKeyEnv:   {label: "Azure API Key Env", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldChatGPTBaseURL:   {label: "ChatGPT Base URL", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldChatGPTModel:     {label: "ChatGPT Model", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldChatGPTAPIKeyEnv: {label: "ChatGPT API Key Env", kind: settingsFieldKindText, group: settingsFieldGroupProvider},
	fieldReactRalphIter:   {label: "ReAct Ralph Iterations", kind: settingsFieldKindText, group: settingsFieldGroupRalph},
	fieldPlanRalphIter:    {label: "Plan Ralph Iterations", kind: settingsFieldKindText, group: settingsFieldGroupRalph},
	fieldCompactThreshold: {label: "Compact Threshold (k)", kind: settingsFieldKindText, group: settingsFieldGroupGeneral},
}

var settingsScopeOrder = []config.Scope{
	config.ScopeEffective,
	config.ScopeManaged,
	config.ScopeUser,
	config.ScopeProject,
	config.ScopeLocal,
}

func newSettingsModal(settings domain.Settings) settingsModalState {
	return newSettingsModalFromResolved(resolvedSettingsForModal(settings, config.ScopeProject))
}

func newSettingsModalFromResolved(resolved config.ResolvedSettings) settingsModalState {
	return settingsModalState{
		mode:     settingsModeForm,
		scope:    config.ScopeProject,
		resolved: resolved,
		form:     newSettingsFormStateForScope(resolved, config.ScopeProject),
		setup:    newSettingsSetupState(resolved.Effective),
	}
}

func newSetupSettingsModal(settings domain.Settings) settingsModalState {
	return newSetupSettingsModalFromResolved(resolvedSettingsForModal(settings, config.ScopeUser))
}

func newSetupSettingsModalFromResolved(resolved config.ResolvedSettings) settingsModalState {
	modal := newSettingsModalFromResolved(resolved)
	modal.visible = true
	modal.mode = settingsModeSetup
	modal.scope = config.ScopeUser
	modal.form = newSettingsFormStateForScope(resolved, config.ScopeUser)
	return modal
}

func newSettingsFormState(settings domain.Settings) settingsFormState {
	return newSettingsFormStateForScope(resolvedSettingsForModal(settings, config.ScopeProject), config.ScopeProject)
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
	s.providerSet = true
	s.ensureVisibleFocus()
}

func (s settingsFormState) providerModelField(provider domain.Provider) settingsField {
	switch provider {
	case domain.ProviderGemini:
		return fieldGeminiModel
	case domain.ProviderVertex:
		return fieldVertexModel
	case domain.ProviderBedrock:
		return fieldBedrockModel
	case domain.ProviderClaude:
		return fieldClaudeModel
	case domain.ProviderAzure:
		return fieldAzureModel
	case domain.ProviderChatGPT:
		return fieldChatGPTModel
	case domain.ProviderVLLM:
		return fieldVLLMModel
	default:
		return fieldOllamaModel
	}
}

func (s settingsFormState) providerPrimaryField(provider domain.Provider) settingsField {
	switch provider {
	case domain.ProviderVLLM:
		return fieldVLLMBaseURL
	case domain.ProviderGemini:
		return fieldGeminiModel
	case domain.ProviderVertex:
		return fieldVertexModel
	case domain.ProviderBedrock:
		return fieldBedrockBaseURL
	case domain.ProviderClaude:
		return fieldClaudeModel
	case domain.ProviderAzure:
		return fieldAzureBaseURL
	case domain.ProviderChatGPT:
		return fieldChatGPTModel
	default:
		return fieldOllamaBaseURL
	}
}

func (s settingsFormState) missingProviderFields(base domain.Settings, provider domain.Provider) []string {
	_ = base
	return s.previewResolved().Effective.MissingProviderFields(provider)
}

func describeMissingProviderFields(provider domain.Provider, missing []string) string {
	labels := make([]string, 0, len(missing))
	for _, field := range missing {
		switch field {
		case "Base URL":
			labels = append(labels, provider.DisplayName()+" Base URL")
		case "Model":
			labels = append(labels, provider.DisplayName()+" Model")
		case "API Key Env":
			labels = append(labels, provider.DisplayName()+" API Key Env")
		default:
			labels = append(labels, field)
		}
	}
	return strings.Join(labels, ", ")
}

func describeMissingProviderConfiguration(provider domain.Provider, missing []string) string {
	if len(missing) == 1 {
		switch missing[0] {
		case "Base URL":
			return "the " + provider.DisplayName() + " Base URL"
		case "Model":
			return "the " + provider.DisplayName() + " Model"
		case "API Key Env":
			return "the " + provider.DisplayName() + " API Key Env"
		}
	}
	return describeMissingProviderFields(provider, missing)
}

func (s settingsFormState) buildSettings(base domain.Settings) domain.Settings {
	_ = base
	return s.previewResolved().Effective
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
		spec := settingsFieldSpecs[field]
		if spec.group != group {
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

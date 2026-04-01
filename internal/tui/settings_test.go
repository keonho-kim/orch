package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
)

func TestSettingsNavigationUsesUpDown(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings.visible = true

	updatedModel, _ := model.updateSettings(tea.KeyMsg{Type: tea.KeyDown})
	updated := updatedModel.(Model)
	if updated.settings.form.focus != fieldOllamaBaseURL {
		t.Fatalf("expected focus to move to the next provider field, got %v", updated.settings.form.focus)
	}

	updatedModel, _ = updated.updateSettings(tea.KeyMsg{Type: tea.KeyUp})
	updated = updatedModel.(Model)
	if updated.settings.form.focus != fieldProvider {
		t.Fatalf("expected focus to return to provider, got %v", updated.settings.form.focus)
	}
}

func TestSettingsTabChangesScopeAndResetsFocus(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings.visible = true
	model.settings.form.focusField(fieldOllamaBaseURL)

	updatedModel, _ := model.updateSettings(tea.KeyMsg{Type: tea.KeyTab})
	updated := updatedModel.(Model)
	if updated.settings.scope != config.ScopeLocal {
		t.Fatalf("expected tab to move to Local scope, got %s", updated.settings.scope)
	}
	if updated.settings.form.focus != fieldProvider {
		t.Fatalf("expected tab to reset focus to provider, got %v", updated.settings.form.focus)
	}

	updatedModel, _ = updated.updateSettings(tea.KeyMsg{Type: tea.KeyShiftTab})
	updated = updatedModel.(Model)
	if updated.settings.scope != config.ScopeProject {
		t.Fatalf("expected shift+tab to move back to Project scope, got %s", updated.settings.scope)
	}
	if updated.settings.form.focus != fieldProvider {
		t.Fatalf("expected shift+tab to reset focus to provider, got %v", updated.settings.form.focus)
	}
}

func TestSettingsFocusDoesNotFocusToggleFields(t *testing.T) {
	t.Parallel()

	state := newSettingsFormState(testModel(80, 24).snapshot.Settings)
	state.focusField(fieldSelfDriving)
	if state.inputs[fieldOllamaBaseURL].Focused() {
		t.Fatal("did not expect a text input to remain focused on a toggle field")
	}

	state.focusField(fieldOllamaBaseURL)
	if !state.inputs[fieldOllamaBaseURL].Focused() {
		t.Fatal("expected text-backed fields to receive focus")
	}
}

func TestProviderChangeRequiresConfirmation(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings.visible = true

	updatedModel, _ := model.updateSettings(tea.KeyMsg{Type: tea.KeyRight})
	updated := updatedModel.(Model)

	if updated.settings.form.provider != model.settings.form.provider {
		t.Fatalf("provider changed before confirmation: %s", updated.settings.form.provider)
	}
	if !updated.settings.form.hasProviderConfirmation() {
		t.Fatalf("expected provider confirmation state")
	}
	if updated.settings.form.pendingProvider() == "" || updated.settings.form.pendingProvider() == updated.settings.form.provider {
		t.Fatalf("expected a pending provider change, got %+v", updated.settings.form.confirmation)
	}

	updatedModel, _ = updated.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	updated = updatedModel.(Model)
	if updated.settings.form.provider != domain.ProviderVLLM {
		t.Fatalf("expected provider to change after confirmation, got %s", updated.settings.form.provider)
	}
	if updated.settings.form.hasProviderConfirmation() {
		t.Fatalf("expected confirmation state to be cleared")
	}
	if updated.settings.form.focus != fieldProvider {
		t.Fatalf("expected focus to remain on provider after confirmation, got %v", updated.settings.form.focus)
	}
	if updated.statusMessage != "Provider changed to vLLM. Configure the vLLM Model before saving settings." {
		t.Fatalf("unexpected status message: %q", updated.statusMessage)
	}
}

func TestProviderDoesNotChangeWhenFieldIsNotFocused(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings.visible = true
	model.settings.form.focusField(fieldOllamaBaseURL)

	updatedModel, _ := model.updateSettings(tea.KeyMsg{Type: tea.KeyRight})
	updated := updatedModel.(Model)

	if updated.settings.form.provider != model.settings.form.provider {
		t.Fatalf("provider changed while another field was focused")
	}
	if updated.settings.form.hasProviderConfirmation() {
		t.Fatalf("unexpected provider confirmation while another field was focused")
	}
}

func TestSettingsLeftRightTogglesSelfDriving(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings.visible = true
	model.settings.form.focusField(fieldSelfDriving)

	updatedModel, _ := model.updateSettings(tea.KeyMsg{Type: tea.KeyRight})
	updated := updatedModel.(Model)
	if !updated.settings.form.selfDriving {
		t.Fatal("expected self-driving mode to be enabled")
	}
}

func TestSettingsNavigationIncludesAllProviderFields(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings.visible = true

	updatedModel, _ := model.updateSettings(tea.KeyMsg{Type: tea.KeyUp})
	updated := updatedModel.(Model)
	if updated.settings.form.focus != fieldSelfDriving {
		t.Fatalf("expected focus to move to Self-Driving Mode, got %v", updated.settings.form.focus)
	}

	updatedModel, _ = updated.updateSettings(tea.KeyMsg{Type: tea.KeyDown})
	updated = updatedModel.(Model)
	if updated.settings.form.focus != fieldProvider {
		t.Fatalf("expected focus to return to Provider, got %v", updated.settings.form.focus)
	}

	updated.settings.form.setProvider(domain.ProviderVLLM)
	updated.settings.form.focusField(fieldProvider)
	updatedModel, _ = updated.updateSettings(tea.KeyMsg{Type: tea.KeyDown})
	updated = updatedModel.(Model)
	if updated.settings.form.focus != fieldOllamaBaseURL {
		t.Fatalf("expected focus to move to the next field in the shared form, got %v", updated.settings.form.focus)
	}

	updatedModel, _ = updated.updateSettings(tea.KeyMsg{Type: tea.KeyDown})
	updated = updatedModel.(Model)
	if updated.settings.form.focus != fieldOllamaModel {
		t.Fatalf("expected focus to continue through provider fields, got %v", updated.settings.form.focus)
	}
}

func TestSettingsSetupSelectsVLLMAndEntersManualForm(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings = newSetupSettingsModal(model.snapshot.Settings)

	updatedModel, _ := model.updateSettings(tea.KeyMsg{Type: tea.KeyRight})
	updated := updatedModel.(Model)
	if updated.settings.form.provider != domain.ProviderVLLM {
		t.Fatalf("expected provider selection to switch to vLLM, got %s", updated.settings.form.provider)
	}

	updatedModel, _ = updated.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	updated = updatedModel.(Model)
	if updated.settings.isSetup() {
		t.Fatal("expected setup mode to switch to manual form for vLLM")
	}
	if updated.settings.form.focus != fieldVLLMBaseURL {
		t.Fatalf("expected vLLM setup to focus base URL, got %v", updated.settings.form.focus)
	}
}

func TestSettingsSetupOllamaDefaultURLStartsDiscovery(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings = newSetupSettingsModal(model.snapshot.Settings)

	updatedModel, _ := model.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	updated := updatedModel.(Model)
	if updated.settings.setup.step != settingsSetupStepOllamaURL {
		t.Fatalf("expected provider step to advance to Ollama URL, got %v", updated.settings.setup.step)
	}

	updatedModel, cmd := updated.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	updated = updatedModel.(Model)
	if !updated.settings.setup.checking {
		t.Fatal("expected Ollama discovery to enter checking state")
	}
	if cmd == nil {
		t.Fatal("expected discovery command to be returned")
	}
}

func TestSettingsSetupCustomOllamaURLRequiresValue(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings = newSetupSettingsModal(model.snapshot.Settings)
	model.settings.useSetupStep(settingsSetupStepOllamaURL)
	model.settings.setup.urlMode = ollamaURLCustom
	model.settings.setup.urlInput.SetValue("")

	updatedModel, cmd := model.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	updated := updatedModel.(Model)
	if cmd != nil {
		t.Fatal("did not expect discovery command without a custom URL")
	}
	if updated.statusMessage != "Custom Ollama URL is required." {
		t.Fatalf("unexpected status message: %q", updated.statusMessage)
	}
}

func TestOllamaDiscoverySuccessTransitionsToModelSelection(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings = newSetupSettingsModal(model.snapshot.Settings)
	model.settings.useSetupStep(settingsSetupStepOllamaURL)
	model.settings.setup.checking = true

	updatedModel, _ := model.Update(ollamaDiscoveryMsg{
		baseURL: "http://localhost:11434/v1",
		models:  []string{"qwen2.5-coder", "llama3.2"},
	})
	updated := updatedModel.(Model)
	if updated.settings.setup.step != settingsSetupStepOllamaModel {
		t.Fatalf("expected model selection step, got %v", updated.settings.setup.step)
	}
	if len(updated.settings.setup.models) != 2 {
		t.Fatalf("expected discovered models to be stored, got %v", updated.settings.setup.models)
	}
}

func TestOllamaDiscoveryEmptyModelsLeavesURLStep(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings = newSetupSettingsModal(model.snapshot.Settings)
	model.settings.useSetupStep(settingsSetupStepOllamaURL)
	model.settings.setup.checking = true

	updatedModel, _ := model.Update(ollamaDiscoveryMsg{
		baseURL: "http://localhost:11434/v1",
		models:  nil,
	})
	updated := updatedModel.(Model)
	if updated.settings.setup.step != settingsSetupStepOllamaURL {
		t.Fatalf("expected to remain on URL step, got %v", updated.settings.setup.step)
	}
	if updated.statusMessage != "Connected to Ollama, but no local models were found." {
		t.Fatalf("unexpected status message: %q", updated.statusMessage)
	}
}

func TestSettingsSetupSaveSelectedOllamaModel(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings = newSetupSettingsModal(model.snapshot.Settings)
	model.settings.useSetupStep(settingsSetupStepOllamaModel)
	model.settings.setup.baseURL = "http://localhost:11434/v1"
	model.settings.setup.models = []string{"qwen2.5-coder", "llama3.2"}
	model.settings.setup.modelIndex = 1

	updatedModel, cmd := model.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	updated := updatedModel.(Model)
	if updated.settings.visible {
		t.Fatal("expected settings modal to close after saving setup")
	}
	if updated.snapshot.Settings.DefaultProvider != domain.ProviderOllama {
		t.Fatalf("expected Ollama to be saved as default provider, got %s", updated.snapshot.Settings.DefaultProvider)
	}
	if updated.snapshot.Settings.Providers.Ollama.Model != "llama3.2" {
		t.Fatalf("expected selected Ollama model to be saved, got %q", updated.snapshot.Settings.Providers.Ollama.Model)
	}
	if cmd == nil {
		t.Fatal("expected save command to be returned")
	}
}

func TestSettingsFormSaveBuildsSettingsFromFormState(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings.visible = true
	model.settings.form.focusField(fieldPlanRalphIter)
	planInput := model.settings.form.inputs[fieldPlanRalphIter]
	planInput.SetValue("7")
	model.settings.form.inputs[fieldPlanRalphIter] = planInput
	reactInput := model.settings.form.inputs[fieldReactRalphIter]
	reactInput.SetValue("5")
	model.settings.form.inputs[fieldReactRalphIter] = reactInput
	ollamaModelInput := model.settings.form.inputs[fieldOllamaModel]
	ollamaModelInput.SetValue("deepseek-r1")
	model.settings.form.inputs[fieldOllamaModel] = ollamaModelInput

	updatedModel, cmd := model.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	updated := updatedModel.(Model)
	if updated.snapshot.Settings.PlanRalphIter != 7 {
		t.Fatalf("expected plan Ralph iterations to save, got %d", updated.snapshot.Settings.PlanRalphIter)
	}
	if updated.snapshot.Settings.ReactRalphIter != 5 {
		t.Fatalf("expected react Ralph iterations to save, got %d", updated.snapshot.Settings.ReactRalphIter)
	}
	if updated.snapshot.Settings.Providers.Ollama.Model != "deepseek-r1" {
		t.Fatalf("expected Ollama model to save, got %q", updated.snapshot.Settings.Providers.Ollama.Model)
	}
	if cmd == nil {
		t.Fatal("expected save command to be returned")
	}
}

func TestProviderChangeKeepsFocusWhenTargetProviderIsConfigured(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings.visible = true
	vllmModelInput := model.settings.form.inputs[fieldVLLMModel]
	vllmModelInput.SetValue("qwen-vllm")
	model.settings.form.inputs[fieldVLLMModel] = vllmModelInput
	model.settings.form.focusField(fieldProvider)

	updatedModel, _ := model.updateSettings(tea.KeyMsg{Type: tea.KeyRight})
	updated := updatedModel.(Model)
	updatedModel, _ = updated.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	updated = updatedModel.(Model)

	if updated.settings.form.provider != domain.ProviderVLLM {
		t.Fatalf("expected provider to change to vLLM, got %s", updated.settings.form.provider)
	}
	if updated.settings.form.focus != fieldProvider {
		t.Fatalf("expected focus to remain on provider when target is configured, got %v", updated.settings.form.focus)
	}
	if updated.statusMessage != "Provider changed to vLLM." {
		t.Fatalf("unexpected status message: %q", updated.statusMessage)
	}
}

func TestSettingsSaveShowsExplicitProviderConfigurationMessage(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings.visible = true
	model.settings.form.setProvider(domain.ProviderVLLM)
	model.settings.form.focusField(fieldVLLMModel)

	updatedModel, cmd := model.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	updated := updatedModel.(Model)

	if cmd != nil {
		t.Fatal("did not expect save command when selected provider is incomplete")
	}
	want := "Provider changed to vLLM, but settings cannot be saved until the vLLM Model is configured."
	if updated.statusMessage != want {
		t.Fatalf("unexpected status message: %q", updated.statusMessage)
	}
	if !updated.settings.visible {
		t.Fatal("expected settings modal to remain open after rejected save")
	}
}

func TestSettingsSetupSelectsGeminiAndEntersManualForm(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings = newSetupSettingsModal(model.snapshot.Settings)
	model.settings.form.setProvider(domain.ProviderVLLM)

	updatedModel, _ := model.updateSettings(tea.KeyMsg{Type: tea.KeyRight})
	updated := updatedModel.(Model)
	if updated.settings.form.provider != domain.ProviderGemini {
		t.Fatalf("expected provider selection to switch to Gemini, got %s", updated.settings.form.provider)
	}

	updatedModel, _ = updated.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	updated = updatedModel.(Model)
	if updated.settings.isSetup() {
		t.Fatal("expected setup mode to switch to manual form for Gemini")
	}
	if updated.settings.form.focus != fieldGeminiModel {
		t.Fatalf("expected Gemini setup to focus model, got %v", updated.settings.form.focus)
	}
}

func TestSettingsSaveShowsAzureConfigurationMessage(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings.visible = true
	model.settings.form.setProvider(domain.ProviderAzure)
	model.settings.form.focusField(fieldAzureModel)

	updatedModel, cmd := model.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	updated := updatedModel.(Model)

	if cmd != nil {
		t.Fatal("did not expect save command when Azure is incomplete")
	}
	want := "Provider changed to Azure, but settings cannot be saved until Azure Base URL, Azure Model is configured."
	if updated.statusMessage != want {
		t.Fatalf("unexpected status message: %q", updated.statusMessage)
	}
}

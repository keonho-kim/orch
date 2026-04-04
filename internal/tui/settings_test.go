package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/keonho-kim/orch/domain"
)

func TestSettingsNavigationUsesUpDown(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings.visible = true

	updatedModel, _ := model.updateSettings(tea.KeyMsg{Type: tea.KeyDown})
	updated := updatedModel.(Model)
	if updated.settings.form.focus != providerField(domain.ProviderOllama, providerFieldKindEndpoint) {
		t.Fatalf("expected focus to move to the next provider field, got %v", updated.settings.form.focus)
	}

	updatedModel, _ = updated.updateSettings(tea.KeyMsg{Type: tea.KeyUp})
	updated = updatedModel.(Model)
	if updated.settings.form.focus != fieldProvider {
		t.Fatalf("expected focus to return to provider, got %v", updated.settings.form.focus)
	}
}

func TestSettingsFocusDoesNotFocusToggleFields(t *testing.T) {
	t.Parallel()

	state := newSettingsFormState(testModel(80, 24).snapshot.Settings)
	state.focusField(fieldSelfDriving)
	ollamaEndpoint := providerField(domain.ProviderOllama, providerFieldKindEndpoint)
	if state.inputs[ollamaEndpoint].Focused() {
		t.Fatal("did not expect a text input to remain focused on a toggle field")
	}

	state.focusField(ollamaEndpoint)
	if !state.inputs[ollamaEndpoint].Focused() {
		t.Fatal("expected text-backed fields to receive focus")
	}
}

func TestProviderChangeRequiresConfirmation(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.settings.visible = true

	updatedModel, _ := model.updateSettings(tea.KeyMsg{Type: tea.KeyRight})
	updated := updatedModel.(Model)
	if !updated.settings.form.hasProviderConfirmation() {
		t.Fatalf("expected provider confirmation state")
	}

	updatedModel, _ = updated.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	updated = updatedModel.(Model)
	if updated.settings.form.provider != domain.ProviderVLLM {
		t.Fatalf("expected provider to change after confirmation, got %s", updated.settings.form.provider)
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
	if updated.settings.form.focus != providerField(domain.ProviderVLLM, providerFieldKindEndpoint) {
		t.Fatalf("expected vLLM setup to focus endpoint, got %v", updated.settings.form.focus)
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
}

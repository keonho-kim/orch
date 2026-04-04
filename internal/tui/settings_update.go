package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/orchestrator"
)

func (m *Model) openSettings() {
	needsConfiguration := m.needsSettingsConfiguration()
	if m.service != nil {
		if needsConfiguration {
			m.settings = newSetupSettingsModalFromState(m.service.ConfigState())
		} else {
			m.settings = newSettingsModalFromState(m.service.ConfigState())
			m.settings.visible = true
		}
	} else {
		if needsConfiguration {
			m.settings = newSetupSettingsModal(m.snapshot.Settings)
		} else {
			m.settings = newSettingsModal(m.snapshot.Settings)
			m.settings.visible = true
		}
	}
	m.settings.resize(max(20, m.viewportWidth()-24))
}

func (m *Model) updateOllamaDiscovery(message ollamaDiscoveryMsg) {
	m.settings.setup.checking = false
	if message.err != nil {
		m.statusMessage = message.err.Error()
		return
	}
	if len(message.models) == 0 {
		m.statusMessage = "Connected to Ollama, but no local models were found."
		return
	}

	m.settings.setup.baseURL = message.baseURL
	m.settings.setup.models = message.models
	m.settings.setup.modelIndex = 0
	m.settings.useSetupStep(settingsSetupStepOllamaModel)
	m.statusMessage = fmt.Sprintf("Connected to Ollama. Found %d model(s).", len(message.models))
}

func (m Model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.settings.isSetup() {
		return m.updateSettingsSetup(msg)
	}
	return m.updateSettingsForm(msg)
}

func (m Model) updateSettingsForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	form := &m.settings.form
	if form.hasProviderConfirmation() {
		if handled, nextModel := m.handleProviderConfirmationKey(msg.String()); handled {
			return nextModel, nil
		}
		return m, nil
	}
	if handled, nextModel, cmd := m.handleSettingsFormNavigation(msg.String()); handled {
		return nextModel, cmd
	}
	return m.updateSettingsTextInput(msg)
}

func (m Model) handleProviderConfirmationKey(key string) (bool, tea.Model) {
	form := &m.settings.form
	switch key {
	case "esc":
		form.cancelProviderConfirmation()
		m.statusMessage = "Provider change cancelled."
		return true, m
	case "enter":
		pending := form.pendingProvider()
		form.cancelProviderConfirmation()
		if pending != "" && pending != form.provider {
			form.setProvider(pending)
			form.focusField(fieldProvider)
			if missing := form.missingProviderFields(m.snapshot.Settings, form.provider); len(missing) > 0 {
				m.statusMessage = fmt.Sprintf(
					"Provider changed to %s. Configure %s before saving settings.",
					form.provider.DisplayName(),
					describeMissingProviderConfiguration(form.provider, missing),
				)
			} else {
				m.statusMessage = fmt.Sprintf("Provider changed to %s.", form.provider.DisplayName())
			}
		}
		return true, m
	default:
		return false, m
	}
}

func (m Model) handleSettingsFormNavigation(key string) (bool, tea.Model, tea.Cmd) {
	form := &m.settings.form
	switch key {
	case "esc":
		if !m.needsSettingsConfiguration() {
			m.settings.visible = false
		}
		return true, m, nil
	case "ctrl+c":
		return true, m, tea.Quit
	case "down":
		form.focusNext()
		return true, m, nil
	case "up":
		form.focusPrev()
		return true, m, nil
	case "left", "right":
		return true, m.handleSettingsFormHorizontalKey(key), nil
	case "enter":
		nextModel, cmd := m.saveSettingsForm()
		return true, nextModel, cmd
	default:
		return false, m, nil
	}
}

func (m Model) handleSettingsFormHorizontalKey(key string) tea.Model {
	form := &m.settings.form
	switch form.focus {
	case fieldProvider:
		step := 1
		if key == "left" {
			step = -1
		}
		target := nextProvider(form.displayProvider(), step)
		if target != form.displayProvider() {
			form.beginProviderConfirmation(target)
			m.statusMessage = fmt.Sprintf(
				"Confirm provider change: %s -> %s.",
				form.displayProvider().DisplayName(),
				target.DisplayName(),
			)
		}
	case fieldSelfDriving:
		form.selfDriving = !form.selfDriving
		if form.selfDriving {
			m.statusMessage = "Self-driving mode enabled."
		} else {
			m.statusMessage = "Self-driving mode disabled."
		}
	}
	return m
}

func (m Model) saveSettingsForm() (tea.Model, tea.Cmd) {
	settings := m.settings.form.buildSettings(m.snapshot.Settings)
	if missing := settings.MissingProviderFields(settings.DefaultProvider); len(missing) > 0 {
		m.statusMessage = fmt.Sprintf(
			"Provider changed to %s, but settings cannot be saved until %s is configured.",
			settings.DefaultProvider.DisplayName(),
			describeMissingProviderConfiguration(settings.DefaultProvider, missing),
		)
		return m, nil
	}
	m.snapshot.Settings = settings
	m.settings.configState.Settings = settings
	m.settings.visible = false
	return m, saveSettingsCmd(m.service, settings)
}

func (m Model) updateSettingsTextInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	form := &m.settings.form
	if !form.isTextField(form.focus) {
		return m, nil
	}
	input := form.inputs[form.focus]
	updated, cmd := input.Update(msg)
	form.inputs[form.focus] = updated
	return m, cmd
}

func (m Model) updateSettingsSetup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	setup := &m.settings.setup

	switch setup.step {
	case settingsSetupStepProvider:
		return m.handleSettingsSetupProvider(msg)
	case settingsSetupStepOllamaURL:
		return m.handleSettingsSetupOllamaURL(msg)
	case settingsSetupStepOllamaModel:
		return m.handleSettingsSetupOllamaModel(msg)
	}

	return m, nil
}

func (m Model) handleSettingsSetupProvider(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	form := &m.settings.form
	switch msg.String() {
	case "esc":
		if !m.needsSettingsConfiguration() {
			m.settings.visible = false
		}
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "left", "up":
		form.setProvider(nextProvider(form.provider, -1))
		return m, nil
	case "right", "down":
		form.setProvider(nextProvider(form.provider, 1))
		return m, nil
	case "enter":
		if form.provider == domain.ProviderOllama {
			m.settings.useSetupStep(settingsSetupStepOllamaURL)
			m.statusMessage = "Choose the Ollama URL and press Enter to check connection."
			return m, nil
		}
		m.settings.useFormMode()
		form.focusField(form.providerPrimaryField(form.provider))
		m.statusMessage = fmt.Sprintf("Configure %s settings.", form.provider.DisplayName())
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) handleSettingsSetupOllamaURL(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	setup := &m.settings.setup
	switch msg.String() {
	case "esc":
		setup.urlInput.Blur()
		m.settings.useSetupStep(settingsSetupStepProvider)
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "left", "up":
		setup.urlMode = ollamaURLDefault
		setup.urlInput.Blur()
		return m, nil
	case "right", "down":
		setup.urlMode = ollamaURLCustom
		setup.urlInput.Focus()
		return m, nil
	case "enter":
		baseURL := strings.TrimSpace(setup.urlInput.Value())
		if setup.urlMode == ollamaURLCustom && baseURL == "" {
			m.statusMessage = "Custom Ollama URL is required."
			return m, nil
		}
		setup.checking = true
		if setup.urlMode != ollamaURLCustom {
			baseURL = ""
		}
		m.statusMessage = "Checking Ollama connection and loading model list..."
		return m, discoverOllamaCmd(m.service, baseURL)
	default:
		if setup.urlMode != ollamaURLCustom {
			return m, nil
		}
		var cmd tea.Cmd
		setup.urlInput, cmd = setup.urlInput.Update(msg)
		return m, cmd
	}
}

func (m Model) handleSettingsSetupOllamaModel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	setup := &m.settings.setup
	switch msg.String() {
	case "esc":
		m.settings.useSetupStep(settingsSetupStepOllamaURL)
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "up":
		if setup.modelIndex > 0 {
			setup.modelIndex--
		}
		return m, nil
	case "down":
		if setup.modelIndex+1 < len(setup.models) {
			setup.modelIndex++
		}
		return m, nil
	case "enter":
		if len(setup.models) == 0 {
			return m, nil
		}
		return m.saveDiscoveredOllamaSettings()
	default:
		return m, nil
	}
}

func (m Model) saveDiscoveredOllamaSettings() (tea.Model, tea.Cmd) {
	setup := m.settings.setup
	settings := m.settings.form.buildSettings(m.snapshot.Settings)
	settings.DefaultProvider = domain.ProviderOllama
	settings.Providers.Ollama.Endpoint = setup.baseURL
	settings.Providers.Ollama.Model = setup.models[setup.modelIndex]
	settings.Normalize()
	m.snapshot.Settings = settings
	m.settings.visible = false
	return m, saveSettingsCmd(m.service, settings)
}

func saveSettingsCmd(service *orchestrator.Service, settings domain.Settings) tea.Cmd {
	return func() tea.Msg {
		if service == nil {
			return nil
		}
		if err := service.SaveSettings(settings); err != nil {
			return operationErrMsg{err: err}
		}
		return nil
	}
}

func discoverOllamaCmd(service *orchestrator.Service, baseURL string) tea.Cmd {
	return func() tea.Msg {
		normalized, models, err := service.DiscoverOllama(context.Background(), baseURL)
		return ollamaDiscoveryMsg{
			baseURL: normalized,
			models:  models,
			err:     err,
		}
	}
}

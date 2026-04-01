package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
	"github.com/keonho-kim/orch/internal/orchestrator"
)

func (m *Model) openSettings() {
	needsConfiguration := m.needsSettingsConfiguration()
	var resolved config.ResolvedSettings
	if m.service != nil {
		resolved = m.service.ConfigState()
	} else {
		resolved = resolvedSettingsForModal(m.snapshot.Settings, config.ScopeProject)
	}
	if needsConfiguration {
		m.settings = newSetupSettingsModalFromResolved(resolved)
	} else {
		m.settings = newSettingsModalFromResolved(resolved)
		m.settings.visible = true
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
		switch msg.String() {
		case "esc":
			form.cancelProviderConfirmation()
			m.statusMessage = "Provider change cancelled."
			return m, nil
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
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		if !m.needsSettingsConfiguration() {
			m.settings.visible = false
		}
		return m, nil
	case "tab":
		m.settings.setScope(nextSettingsScope(m.settings.scope, 1))
		m.settings.resize(max(20, m.viewportWidth()-24))
		return m, nil
	case "shift+tab":
		m.settings.setScope(nextSettingsScope(m.settings.scope, -1))
		m.settings.resize(max(20, m.viewportWidth()-24))
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "ctrl+u":
		if !form.isEditable() {
			m.statusMessage = fmt.Sprintf("%s scope is read-only.", settingsScopeLabel(m.settings.scope))
			return m, nil
		}
		if form.fieldLocked(form.focus) {
			m.statusMessage = "Managed settings cannot be overridden in this scope."
			return m, nil
		}
		m.statusMessage = form.unsetFocusedField()
		return m, nil
	case "down":
		form.focusNext()
		return m, nil
	case "up":
		form.focusPrev()
		return m, nil
	case "left", "right":
		switch form.focus {
		case fieldProvider:
			if !form.isEditable() {
				return m, nil
			}
			if form.fieldLocked(fieldProvider) {
				m.statusMessage = "Managed settings cannot be overridden in this scope."
				return m, nil
			}
			step := 1
			if msg.String() == "left" {
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
			return m, nil
		case fieldSelfDriving:
			if !form.isEditable() {
				return m, nil
			}
			if form.fieldLocked(fieldSelfDriving) {
				m.statusMessage = "Managed settings cannot be overridden in this scope."
				return m, nil
			}
			form.selfDriving = !form.selfDriving
			form.selfDrivingSet = true
			if form.selfDriving {
				m.statusMessage = "Self-driving mode enabled."
			} else {
				m.statusMessage = "Self-driving mode disabled."
			}
			return m, nil
		}
	case "enter":
		if !form.isEditable() {
			m.statusMessage = fmt.Sprintf("%s scope is read-only.", settingsScopeLabel(m.settings.scope))
			return m, nil
		}
		preview := form.previewResolved()
		settings := preview.Effective
		if missing := settings.MissingProviderFields(settings.DefaultProvider); len(missing) > 0 {
			m.statusMessage = fmt.Sprintf(
				"Provider changed to %s, but settings cannot be saved until %s is configured.",
				settings.DefaultProvider.DisplayName(),
				describeMissingProviderConfiguration(settings.DefaultProvider, missing),
			)
			return m, nil
		}
		m.snapshot.Settings = settings
		m.settings.resolved = preview
		m.settings.visible = false
		return m, saveScopeSettingsCmd(m.service, m.settings.scope, form.buildScopeSettings())
	}

	if !form.isTextField(form.focus) {
		return m, nil
	}
	if !form.isEditable() {
		return m, nil
	}
	if form.fieldLocked(form.focus) {
		return m, nil
	}

	input := form.inputs[form.focus]
	updated, cmd := input.Update(msg)
	form.inputs[form.focus] = updated
	return m, cmd
}

func (m Model) updateSettingsSetup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	setup := &m.settings.setup
	form := &m.settings.form

	switch setup.step {
	case settingsSetupStepProvider:
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
		}
	case settingsSetupStepOllamaURL:
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
			if setup.urlMode == ollamaURLCustom && strings.TrimSpace(setup.urlInput.Value()) == "" {
				m.statusMessage = "Custom Ollama URL is required."
				return m, nil
			}
			setup.checking = true
			baseURL := ""
			if setup.urlMode == ollamaURLCustom {
				baseURL = strings.TrimSpace(setup.urlInput.Value())
			}
			m.statusMessage = "Checking Ollama connection and loading model list..."
			return m, discoverOllamaCmd(m.service, baseURL)
		}
		if setup.urlMode == ollamaURLCustom {
			var cmd tea.Cmd
			setup.urlInput, cmd = setup.urlInput.Update(msg)
			return m, cmd
		}
		return m, nil
	case settingsSetupStepOllamaModel:
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
			settings := form.buildSettings(m.snapshot.Settings)
			settings.DefaultProvider = domain.ProviderOllama
			settings.Providers.Ollama.BaseURL = setup.baseURL
			settings.Providers.Ollama.Model = setup.models[setup.modelIndex]
			settings.Normalize()
			m.snapshot.Settings = settings
			m.settings.visible = false
			userScope := config.ScopeSettingsFromDomainSettings(settings)
			return m, saveScopeSettingsCmd(m.service, config.ScopeUser, userScope)
		}
	}

	return m, nil
}

func saveScopeSettingsCmd(service *orchestrator.Service, scope config.Scope, settings config.ScopeSettings) tea.Cmd {
	return func() tea.Msg {
		if service == nil {
			return nil
		}
		if err := service.SaveScopeSettings(scope, settings); err != nil {
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

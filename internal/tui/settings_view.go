package tui

import (
	"fmt"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

func (m Model) renderSettingsLines(width int) []string {
	if m.settings.isSetup() {
		return m.renderSettingsSetupLines(width)
	}

	maxWidth := width
	lines := []string{
		sectionHeader("SETTINGS", maxWidth),
		"",
		fitLine("Up/Down: Move Between Fields", maxWidth),
		fitLine("Left/Right: Change Focused Value", maxWidth),
		fitLine("Enter: Save  Esc: Cancel", maxWidth),
		"",
	}

	for index, group := range settingsFieldGroups {
		fields := settingsFieldsForGroup(group)
		if len(fields) == 0 {
			continue
		}
		if index > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, settingsGroupHeader(settingsFieldGroupTitles[group], maxWidth))
		for _, field := range fields {
			lines = append(lines, fitLine(m.renderSettingsField(field), maxWidth))
		}
	}

	if missing := m.settings.form.missingProviderFields(m.snapshot.Settings, m.settings.form.provider); len(missing) > 0 {
		lines = append(lines,
			"",
			fitLine(fmt.Sprintf(
				"Selected provider %s is incomplete. Configure %s before saving.",
				m.settings.form.provider.DisplayName(),
				describeMissingProviderConfiguration(m.settings.form.provider, missing),
			), maxWidth),
		)
	}

	if m.settings.form.hasProviderConfirmation() {
		lines = append(lines,
			"",
			fitLine(fmt.Sprintf(
				"Change provider from %s to %s?",
				m.settings.form.provider.DisplayName(),
				m.settings.form.pendingProvider().DisplayName(),
			), maxWidth),
			fitLine("Enter: Confirm  Esc: Cancel", maxWidth),
		)
	}

	return lines
}

func (m Model) renderSettingsField(field settingsField) string {
	label := settingsFieldLabel(field)
	prefix := "  "
	if m.settings.form.focus == field {
		prefix = "> "
	}
	suffix := ""
	if m.settings.form.fieldLocked(field) {
		suffix = " [LOCKED]"
	}

	switch settingsFieldSpecs[field].kind {
	case settingsFieldKindProvider:
		selectedProvider := m.settings.form.displayProvider()
		if m.settings.form.hasProviderConfirmation() && m.settings.form.pendingProvider() != "" {
			selectedProvider = m.settings.form.pendingProvider()
		}
		value := renderSettingsProviderSelector(selectedProvider)
		return prefix + label + ": " + value + suffix
	case settingsFieldKindToggle:
		value := onOffLabel(m.settings.form.displaySelfDriving())
		return prefix + label + ": " + value + suffix
	default:
		return prefix + label + ": " + m.settings.form.inputs[field].View() + suffix
	}
}

func renderSettingsProviderSelector(provider domain.Provider) string {
	options := make([]string, 0, len(domain.Providers()))
	for _, candidate := range domain.Providers() {
		options = append(options, renderProviderOption(candidate.DisplayName(), provider == candidate))
	}
	return strings.Join(options, "  ")
}

func onOffLabel(enabled bool) string {
	if enabled {
		return "On"
	}
	return "Off"
}

func settingsGroupHeader(title string, width int) string {
	label := "[ " + title + " ]"
	line := strings.Repeat("-", max(0, width-len(label)-1))
	return fitLine(label+" "+line, width)
}

func (m Model) renderSettingsSetupLines(width int) []string {
	maxWidth := width
	setup := m.settings.setup

	switch setup.step {
	case settingsSetupStepProvider:
		lines := []string{
			sectionHeader("SETUP", maxWidth),
			"",
			fitLine("Choose the provider for the first launch.", maxWidth),
			fitLine("Left/Right or Up/Down: switch provider", maxWidth),
			fitLine("Enter: continue", maxWidth),
			"",
		}
		for _, provider := range domain.Providers() {
			lines = append(lines, fitLine(renderProviderOption(provider.DisplayName(), m.settings.form.provider == provider), maxWidth))
		}
		return lines
	case settingsSetupStepOllamaURL:
		defaultSelected := setup.urlMode == ollamaURLDefault
		customSelected := setup.urlMode == ollamaURLCustom
		lines := []string{
			sectionHeader("OLLAMA SETUP", maxWidth),
			"",
			fitLine("Choose the Ollama URL, then press Enter to check the connection and load models.", maxWidth),
			fitLine("Left/Right or Up/Down: switch URL mode", maxWidth),
			fitLine("Enter: connect  Esc: back", maxWidth),
			"",
			fitLine(renderProviderOption("Use default URL (http://localhost:11434/v1)", defaultSelected), maxWidth),
			fitLine(renderProviderOption("Use custom URL", customSelected), maxWidth),
		}
		if customSelected {
			lines = append(lines, "")
			lines = append(lines, fitLine("> URL: "+setup.urlInput.View(), maxWidth))
		}
		if setup.checking {
			lines = append(lines, "", fitLine("Connecting to Ollama...", maxWidth))
		}
		return lines
	case settingsSetupStepOllamaModel:
		lines := []string{
			sectionHeader("OLLAMA MODELS", maxWidth),
			"",
			fitLine("Connected successfully. Choose a model and press Enter to save.", maxWidth),
			fitLine("Up/Down: move  Enter: save  Esc: back", maxWidth),
			"",
			fitLine("Base URL: "+setup.baseURL, maxWidth),
			"",
		}
		for index, model := range setup.models {
			prefix := "  "
			if index == setup.modelIndex {
				prefix = "> "
			}
			lines = append(lines, fitLine(prefix+model, maxWidth))
		}
		return lines
	default:
		return []string{sectionHeader("SETTINGS", maxWidth)}
	}
}

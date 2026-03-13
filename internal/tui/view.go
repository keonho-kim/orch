package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"orch/domain"
	"orch/internal/branding"
)

var (
	titleStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	subtleStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	sectionStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	keyStyle            = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117"))
	footerStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	statusLineStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("62"))
	commandLabelStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	commandMetaStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("24")).Padding(0, 1)
	providerChipOn      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("33")).Padding(0, 1)
	providerChipOff     = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("236")).Padding(0, 1)
	userLabelStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("31")).Padding(0, 1)
	assistantLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("232")).Background(lipgloss.Color("150")).Padding(0, 1)
	thinkLabelStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245"))
	userBoxStyle        = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("31")).Padding(0, 1)
	assistantBoxStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("150")).Padding(0, 1)
	thinkBoxStyle       = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
	thinkContentStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	thinkingInlineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
)

func (m Model) View() string {
	width := m.viewportWidth()
	height := m.viewportHeight()

	if m.snapshot.PendingApproval != nil {
		return renderViewport(m.renderCompactPage(width, m.renderApprovalLines(width)), width, height)
	}
	if m.showExitConfirm {
		return renderViewport(m.renderCompactPage(width, m.renderExitConfirmLines(width)), width, height)
	}
	if m.settings.visible {
		return renderViewport(m.renderPageWithHeader(width, m.renderSettingsLines(width)), width, height)
	}

	return m.renderDashboardView(width, height)
}

func (m Model) renderDashboardView(width int, height int) string {
	help := m.dashboardHelpLines(width, height)
	bodyHeight := m.chatViewportHeight(width, height)
	m.body.Width = width
	m.body.Height = bodyHeight

	lines := strings.Split(m.body.View(), "\n")
	lines = append(lines, m.renderCommandLine(width), statusLineStyle.Render(fitLine(m.statusLine(), width)))
	for _, line := range help {
		lines = append(lines, footerStyle.Render(fitLine(line, width)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderPageWithHeader(width int, body []string) []string {
	lines := append([]string{}, m.renderHeaderLines(width)...)
	if len(body) > 0 {
		lines = append(lines, "")
		lines = append(lines, body...)
	}
	return lines
}

func (m Model) renderCompactPage(width int, body []string) []string {
	lines := []string{
		fitLine(titleStyle.Render("ORCH"), width),
		fitLine(subtleStyle.Render("Version")+" "+branding.Version, width),
	}
	if len(body) > 0 {
		lines = append(lines, "")
		lines = append(lines, body...)
	}
	return lines
}

func (m Model) renderHeaderLines(width int) []string {
	metaLine := subtleStyle.Render("Version") + " " + branding.Version
	lines := make([]string, 0, len(branding.Wordmark)+1)
	for _, line := range branding.Wordmark {
		lines = append(lines, fitLine(titleStyle.Render(line), width))
	}
	lines = append(lines, fitLine(metaLine, width))
	return lines
}

func (m Model) renderCommandLine(width int) string {
	prefix := commandLabelStyle.Render("COMMAND") + " > "
	meta := commandMetaStyle.Render(m.commandMeta())
	input := m.input
	metaWidth := lipgloss.Width(meta)
	available := width - lipgloss.Width(prefix)
	if metaWidth > 0 {
		available -= metaWidth + 1
	}
	input.Width = max(1, available)
	line := prefix + input.View()
	padding := width - lipgloss.Width(line) - metaWidth
	if padding < 1 {
		padding = 1
	}
	if metaWidth > 0 {
		line += strings.Repeat(" ", padding) + meta
	}
	return fitLine(line, width)
}

func (m Model) commandMeta() string {
	modeName := "ACTION"
	if m.composerMode == domain.RunModePlan {
		modeName = "PLAN"
	}

	providerName := "UNSET"
	modelName := "UNSET"
	if m.snapshot.Settings.DefaultProvider != "" {
		providerName = strings.ToUpper(m.snapshot.Settings.DefaultProvider.DisplayName())
		model := strings.TrimSpace(m.snapshot.Settings.ConfigFor(m.snapshot.Settings.DefaultProvider).Model)
		if model != "" {
			modelName = strings.ToUpper(model)
		}
	}

	return "[" + modeName + "] " + providerName + " " + modelName
}

func (m Model) renderExitConfirmLines(width int) []string {
	maxWidth := width
	return []string{
		sectionHeader("EXIT", maxWidth),
		"",
		fitLine("A run is still active.", maxWidth),
		fitLine("Enter: cancel the run and quit", maxWidth),
		fitLine("Esc: stay in orch", maxWidth),
	}
}

func (m Model) renderApprovalLines(width int) []string {
	request := m.snapshot.PendingApproval
	maxWidth := width
	if request == nil {
		return []string{sectionHeader("APPROVAL", maxWidth), "No approval pending."}
	}

	return []string{
		sectionHeader("APPROVAL", maxWidth),
		"",
		fitLine("Run: "+request.RunID, maxWidth),
		fitLine("Tool: "+request.Call.Name, maxWidth),
		fitLine("Reason: "+request.Reason, maxWidth),
		"",
		fitLine("Arguments:", maxWidth),
		fitLine(domain.ClipTask(request.Call.Arguments, maxWidth), maxWidth),
		"",
		fitLine("Enter/Y: approve", maxWidth),
		fitLine("Esc/N: deny", maxWidth),
	}
}

func (m Model) statusLine() string {
	if strings.TrimSpace(m.statusMessage) != "" {
		return m.statusMessage
	}
	if m.snapshot.PendingApproval != nil {
		return "Approval required for the next local tool."
	}
	if strings.TrimSpace(m.snapshot.ActivePlan.Content) != "" {
		return "Ready. Cached plan is active for ReAct runs."
	}
	if m.needsSettingsConfiguration() {
		return "Open Settings and configure a provider model."
	}
	return "Ready."
}

func renderStyledLines(output string, width int, emptyPlaceholder string) []string {
	if width <= 0 {
		return nil
	}
	if strings.TrimSpace(output) == "" {
		return []string{fitLine(emptyPlaceholder, width)}
	}

	sourceLines := strings.Split(output, "\n")
	lines := make([]string, 0, len(sourceLines))
	for _, line := range sourceLines {
		styled := renderInlineMarkdown(line)
		lines = append(lines, wrapLine(styled, width)...)
	}
	if len(lines) == 0 {
		return []string{fitLine(emptyPlaceholder, width)}
	}
	return lines
}

func renderWrappedLines(output string, width int, emptyPlaceholder string) []string {
	if width <= 0 {
		return nil
	}
	if strings.TrimSpace(output) == "" {
		return []string{fitLine(emptyPlaceholder, width)}
	}

	sourceLines := strings.Split(output, "\n")
	lines := make([]string, 0, len(sourceLines))
	for _, line := range sourceLines {
		lines = append(lines, wrapLine(line, width)...)
	}
	if len(lines) == 0 {
		return []string{fitLine(emptyPlaceholder, width)}
	}
	return lines
}

func (m Model) viewportWidth() int {
	if m.width > 0 {
		return m.width
	}
	return 80
}

func (m Model) viewportHeight() int {
	if m.height > 0 {
		return m.height
	}
	return 24
}

func (m Model) chatViewportHeight(width int, height int) int {
	bodyHeight := height - 2 - len(m.dashboardHelpLines(width, height))
	if bodyHeight < 1 {
		return 1
	}
	return bodyHeight
}

func (m Model) dashboardHelpLines(width int, height int) []string {
	candidates := m.helpLines(width)
	maxLines := max(0, height-3)
	if maxLines <= 0 {
		return nil
	}
	if len(candidates) > maxLines {
		return candidates[:maxLines]
	}
	return candidates
}

func (m Model) helpLines(width int) []string {
	switch {
	case width >= 84:
		return []string{
			renderHelpLine("Enter", "Send", "Shift+Tab", "Mode", "Up/Down", "History", "PgUp/PgDn", "Scroll", "Home/End", "Jump", "Ctrl+T", "Show Think", "Ctrl+S", "Settings"),
			renderHelpLine("/exit", "Quit", "Mouse", "Scroll", "Enter", "Approve", "Esc", "Cancel"),
		}
	default:
		return []string{
			renderHelpLine("Enter", "Send", "Shift+Tab", "Mode", "Up/Down", "History", "PgUp/PgDn", "Scroll", "Ctrl+T", "Show Think"),
			renderHelpLine("Home/End", "Jump", "Ctrl+S", "Settings", "/exit", "Quit"),
		}
	}
}

func sectionHeader(title string, width int) string {
	line := strings.Repeat("-", max(0, width-lipgloss.Width(title)-1))
	return sectionStyle.Render(title + " " + line)
}

func renderHelpLine(parts ...string) string {
	items := make([]string, 0, len(parts)/2)
	for index := 0; index+1 < len(parts); index += 2 {
		items = append(items, keyStyle.Render(parts[index])+" "+parts[index+1])
	}
	return strings.Join(items, "  |  ")
}

func renderProviderOption(label string, selected bool) string {
	if selected {
		return providerChipOn.Render(label)
	}
	return providerChipOff.Render(label)
}

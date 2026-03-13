package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"orch/domain"
	"orch/internal/orchestrator"
)

type Model struct {
	service *orchestrator.Service
	input   textinput.Model
	body    viewport.Model

	width  int
	height int

	snapshot        orchestrator.Snapshot
	historyIndex    int
	historyDraft    string
	composerMode    domain.RunMode
	statusMessage   string
	showThinking    bool
	followOutput    bool
	showExitConfirm bool
	settings        settingsModalState
}

type serviceUpdateMsg struct {
	event orchestrator.UIEvent
}

type operationErrMsg struct {
	err error
}

type exitCompletedMsg struct{}
type ollamaDiscoveryMsg struct {
	baseURL string
	models  []string
	err     error
}

func New(service *orchestrator.Service) Model {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = "Describe the next request..."
	input.Focus()
	input.CharLimit = 0
	input.Width = 64

	body := viewport.New(0, 0)
	body.MouseWheelEnabled = true
	body.MouseWheelDelta = 3

	model := Model{
		service:      service,
		input:        input,
		body:         body,
		historyIndex: -1,
		composerMode: domain.RunModeReact,
		snapshot:     service.Snapshot(),
		showThinking: true,
		followOutput: true,
		settings:     newSettingsModal(service.Snapshot().Settings),
	}
	if model.needsSettingsConfiguration() {
		model.settings = newSetupSettingsModal(model.snapshot.Settings)
		model.statusMessage = "Configure a provider and model to start the first run."
	}
	model.syncChatViewport(true)

	return model
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, waitForServiceUpdate(m.service))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch message := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
		m.input.Width = max(8, message.Width-18)
		m.settings.resize(max(20, m.viewportWidth()-24))
		m.syncChatViewport(m.followOutput || m.body.AtBottom())
		return m, nil
	case serviceUpdateMsg:
		stickToBottom := m.followOutput || m.body.AtBottom()
		m.snapshot = m.service.Snapshot()
		if strings.TrimSpace(message.event.Message) != "" {
			m.statusMessage = message.event.Message
		}
		m.syncChatViewport(stickToBottom)
		return m, waitForServiceUpdate(m.service)
	case operationErrMsg:
		if message.err != nil {
			m.statusMessage = message.err.Error()
		}
		return m, nil
	case exitCompletedMsg:
		return m, tea.Quit
	case ollamaDiscoveryMsg:
		m.updateOllamaDiscovery(message)
		return m, nil
	case tea.KeyMsg:
		if m.snapshot.PendingApproval != nil {
			return m.updateApproval(message)
		}
		if m.showExitConfirm {
			return m.updateExitConfirm(message)
		}
		if m.settings.visible {
			return m.updateSettings(message)
		}
		return m.updateDashboard(message)
	case tea.MouseMsg:
		if m.snapshot.PendingApproval != nil || m.showExitConfirm || m.settings.visible {
			return m, nil
		}
		return m.updateDashboard(message)
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m Model) updateDashboard(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch message := msg.(type) {
	case tea.KeyMsg:
		switch message.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+s":
			m.openSettings()
			return m, nil
		case "ctrl+t":
			stickToBottom := m.followOutput || m.body.AtBottom()
			m.showThinking = !m.showThinking
			if m.showThinking {
				m.statusMessage = "Thinking expanded."
			} else {
				m.statusMessage = "Thinking collapsed."
			}
			m.syncChatViewport(stickToBottom)
			return m, nil
		case "pgup":
			m.body.PageUp()
			m.followOutput = m.body.AtBottom()
			return m, nil
		case "pgdown":
			m.body.PageDown()
			m.followOutput = m.body.AtBottom()
			return m, nil
		case "home":
			m.body.GotoTop()
			m.followOutput = false
			return m, nil
		case "end":
			m.body.GotoBottom()
			m.followOutput = true
			return m, nil
		case "up":
			m.applyHistory(-1)
			return m, nil
		case "down":
			m.applyHistory(1)
			return m, nil
		case "shift+tab":
			if m.composerMode == domain.RunModeReact {
				m.composerMode = domain.RunModePlan
				m.statusMessage = "Plan mode enabled."
			} else {
				m.composerMode = domain.RunModeReact
				m.statusMessage = "Plan mode disabled."
			}
			return m, nil
		case "enter":
			value := strings.TrimSpace(m.input.Value())
			command := parseDashboardCommand(value)
			if command.kind == commandExit {
				if m.service != nil && m.service.ActiveRunCount() > 0 {
					m.showExitConfirm = true
					m.statusMessage = "An active run is still in progress. Enter will cancel it and quit."
					return m, nil
				}
				return m, tea.Quit
			}
			if value == "" {
				return m, nil
			}
			m.input.SetValue("")
			m.historyIndex = -1
			m.historyDraft = ""
			m.followOutput = true
			return m, submitPromptCmd(m.service, value, m.composerMode)
		}

		var cmd tea.Cmd
		m.input, cmd = m.input.Update(message)
		return m, cmd
	case tea.MouseMsg:
		previousOffset := m.body.YOffset
		var cmd tea.Cmd
		m.body, cmd = m.body.Update(message)
		if m.body.YOffset != previousOffset {
			m.followOutput = m.body.AtBottom()
		}
		return m, cmd
	default:
		return m, nil
	}
}

func (m Model) updateExitConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showExitConfirm = false
		m.statusMessage = "Exit cancelled."
		return m, nil
	case "enter", "ctrl+c":
		m.showExitConfirm = false
		return m, shutdownAllCmd(m.service)
	}

	return m, nil
}

func (m Model) updateApproval(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	request := m.snapshot.PendingApproval
	if request == nil {
		return m, nil
	}

	switch msg.String() {
	case "enter", "y":
		return m, resolveApprovalCmd(m.service, request.RunID, true)
	case "esc", "n":
		return m, resolveApprovalCmd(m.service, request.RunID, false)
	}

	return m, nil
}

func (m *Model) applyHistory(delta int) {
	history := m.snapshot.History
	if len(history) == 0 {
		return
	}

	if m.historyIndex == -1 {
		m.historyDraft = m.input.Value()
	}

	nextIndex := m.historyIndex + delta
	if m.historyIndex == -1 && delta < 0 {
		nextIndex = 0
	}
	if nextIndex < -1 {
		nextIndex = -1
	}
	if nextIndex >= len(history) {
		nextIndex = len(history) - 1
	}

	m.historyIndex = nextIndex
	if m.historyIndex == -1 {
		m.input.SetValue(m.historyDraft)
		return
	}

	m.input.SetValue(history[m.historyIndex])
}

func waitForServiceUpdate(service *orchestrator.Service) tea.Cmd {
	return func() tea.Msg {
		event := <-service.Events()
		return serviceUpdateMsg{event: event}
	}
}

func submitPromptCmd(service *orchestrator.Service, prompt string, mode domain.RunMode) tea.Cmd {
	return func() tea.Msg {
		if _, err := service.SubmitPromptMode(prompt, mode); err != nil {
			return operationErrMsg{err: err}
		}
		return nil
	}
}

func resolveApprovalCmd(service *orchestrator.Service, runID string, approved bool) tea.Cmd {
	return func() tea.Msg {
		if err := service.ResolveApproval(runID, approved); err != nil {
			return operationErrMsg{err: err}
		}
		return nil
	}
}

func shutdownAllCmd(service *orchestrator.Service) tea.Cmd {
	return func() tea.Msg {
		if err := service.ShutdownAll(); err != nil {
			return operationErrMsg{err: err}
		}
		return exitCompletedMsg{}
	}
}

func (m *Model) syncChatViewport(stickToBottom bool) {
	width := m.viewportWidth()
	height := m.chatViewportHeight(width, m.viewportHeight())
	if width <= 0 || height <= 0 {
		return
	}

	m.body.Width = width
	m.body.Height = height
	m.body.SetContent(m.renderChatTimeline(width))
	if stickToBottom || m.body.PastBottom() {
		m.body.GotoBottom()
		m.followOutput = true
	}
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m Model) needsSettingsConfiguration() bool {
	if m.service != nil {
		return m.service.NeedsSettingsConfiguration()
	}

	settings := m.snapshot.Settings
	if settings.DefaultProvider == "" {
		return true
	}
	return !settings.HasProviderModel(settings.DefaultProvider)
}

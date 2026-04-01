package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/orchestrator"
)

type Model struct {
	service *orchestrator.Service
	input   textinput.Model
	body    viewport.Model

	width  int
	height int

	snapshot            orchestrator.Snapshot
	messageHistoryIndex int
	messageHistoryDraft string
	composerMode        domain.RunMode
	statusMessage       string
	showThinking        bool
	followOutput        bool
	showExitConfirm     bool
	showHistoryPicker   bool
	historySessions     []domain.SessionMetadata
	historySessionIndex int
	showInfoModal       bool
	infoTitle           string
	infoLines           []string
	slashMenuIndex      int
	settings            settingsModalState
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
		service:             service,
		input:               input,
		body:                body,
		messageHistoryIndex: -1,
		slashMenuIndex:      0,
		composerMode:        domain.RunModeReact,
		snapshot:            service.Snapshot(),
		showThinking:        true,
		followOutput:        true,
		settings:            newSettingsModalFromResolved(service.ConfigState()),
	}
	if model.needsSettingsConfiguration() {
		model.settings = newSetupSettingsModalFromResolved(service.ConfigState())
		model.statusMessage = "Configure a provider and model to start the first run."
	}
	model.syncChatViewport(true)

	return model
}

func (m *Model) OpenHistoryPicker() {
	m.showHistoryPicker = true
	m.historySessionIndex = 0
	m.statusMessage = "Browse saved sessions."
	if m.service == nil {
		return
	}
	sessions, err := m.service.ListSessions(200)
	if err != nil {
		m.statusMessage = err.Error()
		return
	}
	m.historySessions = sessions
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
		if m.showHistoryPicker {
			return m.updateHistoryPicker(message)
		}
		if m.showInfoModal {
			return m.updateInfoModal(message)
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
			if m.slashMenuVisible() {
				m.moveSlashMenu(-1)
				return m, nil
			}
			m.applyHistory(-1)
			return m, nil
		case "down":
			if m.slashMenuVisible() {
				m.moveSlashMenu(1)
				return m, nil
			}
			m.applyHistory(1)
			return m, nil
		case "tab":
			if m.slashMenuVisible() {
				m.applySlashMenuSelection()
				return m, nil
			}
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
			if m.slashMenuVisible() {
				selected := m.selectedSlashCommand()
				if selected.value != "" && value != selected.value {
					m.applySlashMenuSelection()
					return m, nil
				}
			}
			command := parseDashboardCommand(value)
			if command.kind == commandExit {
				if m.service != nil && m.service.ActiveRunCount() > 0 {
					m.showExitConfirm = true
					m.statusMessage = "An active run is still in progress. Enter will cancel it and quit."
					return m, nil
				}
				return m, tea.Quit
			}
			if command.kind == commandClear {
				m.input.SetValue("")
				m.messageHistoryIndex = -1
				m.messageHistoryDraft = ""
				return m, clearSessionCmd(m.service)
			}
			if command.kind == commandCompact {
				m.input.SetValue("")
				m.messageHistoryIndex = -1
				m.messageHistoryDraft = ""
				return m, compactSessionCmd(m.service)
			}
			if command.kind == commandStatus {
				m.input.SetValue("")
				m.messageHistoryIndex = -1
				m.messageHistoryDraft = ""
				m.openStatusInfo()
				return m, nil
			}
			if command.kind == commandContext {
				m.input.SetValue("")
				m.messageHistoryIndex = -1
				m.messageHistoryDraft = ""
				m.openContextInfo()
				return m, nil
			}
			if command.kind == commandTasks {
				m.input.SetValue("")
				m.messageHistoryIndex = -1
				m.messageHistoryDraft = ""
				m.openTasksInfo(command.arg)
				return m, nil
			}
			if value == "" {
				return m, nil
			}
			m.input.SetValue("")
			m.messageHistoryIndex = -1
			m.messageHistoryDraft = ""
			m.followOutput = true
			return m, submitPromptCmd(m.service, value, m.composerMode)
		}

		var cmd tea.Cmd
		m.input, cmd = m.input.Update(message)
		m.refreshSlashMenu()
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

func (m *Model) refreshSlashMenu() {
	options := filteredSlashCommands(m.input.Value())
	if len(options) == 0 {
		m.slashMenuIndex = 0
		return
	}
	if m.slashMenuIndex < 0 {
		m.slashMenuIndex = 0
	}
	if m.slashMenuIndex >= len(options) {
		m.slashMenuIndex = len(options) - 1
	}
}

func (m Model) slashMenuVisible() bool {
	return len(filteredSlashCommands(m.input.Value())) > 0
}

func (m *Model) moveSlashMenu(delta int) {
	options := filteredSlashCommands(m.input.Value())
	if len(options) == 0 {
		m.slashMenuIndex = 0
		return
	}
	m.slashMenuIndex += delta
	if m.slashMenuIndex < 0 {
		m.slashMenuIndex = len(options) - 1
	}
	if m.slashMenuIndex >= len(options) {
		m.slashMenuIndex = 0
	}
}

func (m Model) selectedSlashCommand() slashCommandOption {
	options := filteredSlashCommands(m.input.Value())
	if len(options) == 0 {
		return slashCommandOption{}
	}
	index := m.slashMenuIndex
	if index < 0 || index >= len(options) {
		index = 0
	}
	return options[index]
}

func (m *Model) applySlashMenuSelection() {
	selected := m.selectedSlashCommand()
	if selected.value == "" {
		return
	}
	m.input.SetValue(selected.value)
	m.refreshSlashMenu()
}

func (m Model) updateHistoryPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showHistoryPicker = false
		m.statusMessage = "Session history closed."
		return m, nil
	case "up":
		if m.historySessionIndex > 0 {
			m.historySessionIndex--
		}
		return m, nil
	case "down":
		if m.historySessionIndex+1 < len(m.historySessions) {
			m.historySessionIndex++
		}
		return m, nil
	case "enter":
		if len(m.historySessions) == 0 {
			return m, nil
		}
		sessionID := m.historySessions[m.historySessionIndex].SessionID
		m.showHistoryPicker = false
		return m, restoreSessionCmd(m.service, sessionID)
	}
	return m, nil
}

func (m Model) updateInfoModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.showInfoModal = false
		m.statusMessage = m.infoTitle + " closed."
		return m, nil
	}
	return m, nil
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
	messageHistory := m.snapshot.MessageHistory
	if len(messageHistory) == 0 {
		return
	}

	if m.messageHistoryIndex == -1 {
		m.messageHistoryDraft = m.input.Value()
	}

	nextIndex := m.messageHistoryIndex + delta
	if m.messageHistoryIndex == -1 && delta < 0 {
		nextIndex = 0
	}
	if nextIndex < -1 {
		nextIndex = -1
	}
	if nextIndex >= len(messageHistory) {
		nextIndex = len(messageHistory) - 1
	}

	m.messageHistoryIndex = nextIndex
	if m.messageHistoryIndex == -1 {
		m.input.SetValue(m.messageHistoryDraft)
		return
	}

	m.input.SetValue(messageHistory[m.messageHistoryIndex])
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

func compactSessionCmd(service *orchestrator.Service) tea.Cmd {
	return func() tea.Msg {
		if err := service.ForceCompact(); err != nil {
			return operationErrMsg{err: err}
		}
		return nil
	}
}

func clearSessionCmd(service *orchestrator.Service) tea.Cmd {
	return func() tea.Msg {
		if err := service.OpenNewSession(); err != nil {
			return operationErrMsg{err: err}
		}
		return nil
	}
}

func (m *Model) openStatusInfo() {
	lines := []string{
		"Session ID: " + fallbackText(strings.TrimSpace(m.snapshot.CurrentSession.SessionID), "none"),
		"Session Title: " + fallbackText(strings.TrimSpace(m.snapshot.CurrentSession.Title), "Untitled session"),
		"Current Run: " + fallbackText(strings.TrimSpace(m.snapshot.CurrentRunID), "none"),
		"Mode: " + m.composerMode.DisplayName(),
		"Provider: " + fallbackText(strings.TrimSpace(m.snapshot.Settings.DefaultProvider.DisplayName()), "UNSET"),
		"Model: " + fallbackText(strings.TrimSpace(m.snapshot.Settings.ConfigFor(m.snapshot.Settings.DefaultProvider).Model), "UNSET"),
		"Pending Approval: " + yesNo(m.snapshot.PendingApproval != nil),
		fmt.Sprintf("Compacted Through Seq: %d", m.snapshot.CurrentSession.LastCompactedSeq),
		fmt.Sprintf("Tokens Since Compact: %d", m.snapshot.CurrentSession.TokensSinceCompact),
		fmt.Sprintf("Total Tokens: %d", m.snapshot.CurrentSession.TotalTokens),
	}

	taskCounts := map[string]int{}
	if m.service != nil {
		tasks, err := m.service.ListCurrentTasks("")
		if err != nil {
			m.statusMessage = err.Error()
			return
		}
		for _, task := range tasks {
			taskCounts[task.Status]++
		}
	}
	lines = append(lines, "Direct Child Tasks:")
	for _, status := range []string{
		domain.TaskStatusQueued,
		domain.TaskStatusRunning,
		domain.TaskStatusCompleted,
		domain.TaskStatusFailed,
		domain.TaskStatusCancelled,
	} {
		lines = append(lines, fmt.Sprintf("- %s: %d", status, taskCounts[status]))
	}
	m.openInfoModal("STATUS", lines)
}

func (m *Model) openContextInfo() {
	if m.service == nil {
		m.statusMessage = "Context view is unavailable without a service."
		return
	}
	snapshot, err := m.service.CurrentContextSnapshot()
	if err != nil {
		m.statusMessage = err.Error()
		return
	}
	m.openInfoModal("CONTEXT", formatContextSnapshotLines(snapshot))
}

func (m *Model) openTasksInfo(arg string) {
	if m.service == nil {
		m.statusMessage = "Task view is unavailable without a service."
		return
	}
	taskID := strings.TrimSpace(arg)
	if taskID == "" {
		tasks, err := m.service.ListCurrentTasks("")
		if err != nil {
			m.statusMessage = err.Error()
			return
		}
		m.openInfoModal("TASKS", formatTaskListLines(tasks))
		return
	}
	task, err := m.service.GetCurrentTask(taskID)
	if err != nil {
		m.statusMessage = err.Error()
		return
	}
	m.openInfoModal("TASK "+taskID, formatTaskDetailLines(task))
}

func (m *Model) openInfoModal(title string, lines []string) {
	m.showInfoModal = true
	m.infoTitle = strings.TrimSpace(title)
	m.infoLines = append([]string(nil), lines...)
	m.statusMessage = strings.TrimSpace(title) + " opened."
}

func formatContextSnapshotLines(snapshot domain.ContextSnapshot) []string {
	return []string{
		"Session ID: " + snapshot.SessionID,
		"Run ID: " + snapshot.RunID,
		"Provider: " + snapshot.Provider,
		"Model: " + snapshot.Model,
		"Workspace: " + snapshot.WorkspacePath,
		"CWD: " + snapshot.CurrentCwd,
		fmt.Sprintf("Compact Summary Present: %s", yesNo(snapshot.CompactSummaryPresent)),
		fmt.Sprintf("Post-Compact Record Count: %d", snapshot.PostCompactRecordCount),
		fmt.Sprintf("Inherited Summary Present: %s", yesNo(snapshot.InheritedSummaryPresent)),
		fmt.Sprintf("Inherited Record Count: %d", snapshot.InheritedRecordCount),
		"Selected Skills: " + fallbackText(strings.Join(snapshot.SelectedSkills, ", "), "(none)"),
		fmt.Sprintf("Resolved References: %d", snapshot.ResolvedReferenceCount),
		fmt.Sprintf("User Memory Present: %s", yesNo(snapshot.UserMemoryPresent)),
		fmt.Sprintf("Chat History Excerpt Bytes: %d", snapshot.ChatHistoryExcerptBytes),
		fmt.Sprintf("Plan Cache Present: %s", yesNo(snapshot.PlanCachePresent)),
	}
}

func formatTaskListLines(tasks []domain.TaskView) []string {
	if len(tasks) == 0 {
		return []string{"No direct child tasks for the current session."}
	}
	sort.Slice(tasks, func(i int, j int) bool {
		return tasks[i].StartedAt.Before(tasks[j].StartedAt)
	})
	lines := make([]string, 0, len(tasks)+1)
	lines = append(lines, "Direct child tasks:")
	for _, task := range tasks {
		lines = append(lines, fmt.Sprintf(
			"- %s | %s | %s | %s/%s",
			fallbackText(task.TaskID, "(no task id)"),
			fallbackText(task.Status, domain.TaskStatusQueued),
			fallbackText(task.Title, "(untitled task)"),
			fallbackText(task.Provider, "unknown"),
			fallbackText(task.Model, "unknown"),
		))
	}
	return lines
}

func formatTaskDetailLines(task domain.TaskView) []string {
	lines := []string{
		"Task ID: " + fallbackText(task.TaskID, "(none)"),
		"Title: " + fallbackText(task.Title, "(untitled task)"),
		"Status: " + fallbackText(task.Status, domain.TaskStatusQueued),
		"Parent Session: " + fallbackText(task.ParentSessionID, "(none)"),
		"Parent Run: " + fallbackText(task.ParentRunID, "(none)"),
		"Child Session: " + fallbackText(task.ChildSessionID, "(none)"),
		"Child Run: " + fallbackText(task.ChildRunID, "(none)"),
		"Worker Role: " + fallbackText(task.WorkerRole, "(none)"),
		"Provider: " + fallbackText(task.Provider, "unknown"),
		"Model: " + fallbackText(task.Model, "unknown"),
		"Summary: " + fallbackText(task.TaskSummary, "(none)"),
		"Changed Paths: " + fallbackText(strings.Join(task.TaskChangedPaths, ", "), "(none)"),
		"Checks Run: " + fallbackText(strings.Join(task.TaskChecksRun, ", "), "(none)"),
		"Evidence Pointers: " + fallbackText(strings.Join(task.TaskEvidencePointers, ", "), "(none)"),
		"Followups: " + fallbackText(strings.Join(task.TaskFollowups, " | "), "(none)"),
		"Error Kind: " + fallbackText(task.TaskErrorKind, "(none)"),
		"Final Output Excerpt: " + fallbackText(task.FinalOutputExcerpt, "(none)"),
	}
	return lines
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func fallbackText(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func restoreSessionCmd(service *orchestrator.Service, sessionID string) tea.Cmd {
	return func() tea.Msg {
		if err := service.RestoreSession(sessionID); err != nil {
			return operationErrMsg{err: err}
		}
		return nil
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
	return !settings.IsProviderReady(settings.DefaultProvider)
}

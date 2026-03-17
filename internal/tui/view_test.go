package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/branding"
	"github.com/keonho-kim/orch/internal/orchestrator"
)

func TestViewShowsApprovalModalWithoutDashboard(t *testing.T) {
	t.Parallel()

	model := testModel(64, 20)
	model.snapshot.PendingApproval = &domain.ApprovalRequest{
		RunID: "R1",
		Call: domain.ToolCall{
			Name:      "exec",
			Arguments: "{\"command\":\"ot\",\"args\":[\"write\",\"--path\",\"README.md\",\"--from-stdin\"]}",
		},
		Reason: "Workspace mutation requires approval.",
	}

	view := model.View()
	if !strings.Contains(view, "APPROVAL") {
		t.Fatalf("expected approval modal, got %q", view)
	}
	if strings.Contains(view, "COMMAND >") {
		t.Fatalf("did not expect dashboard command row behind approval modal, got %q", view)
	}
	assertViewportBounds(t, view, 64, 20)
}

func TestViewDoesNotRenderDuplicateCommandPrompt(t *testing.T) {
	t.Parallel()

	view := testModel(80, 24).View()
	if strings.Contains(view, "COMMAND > >") {
		t.Fatalf("expected a single command prompt, got %q", view)
	}
}

func TestViewShowsSlashCommandDropdown(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.input.SetValue("/")
	model.refreshSlashMenu()

	view := stripANSI(model.View())
	if !strings.Contains(view, "/clear  Open a new session") {
		t.Fatalf("expected slash menu item in view, got %q", view)
	}
	if !strings.Contains(view, "/compact  Compact current session") {
		t.Fatalf("expected compact slash menu item in view, got %q", view)
	}
}

func TestDashboardViewFitsViewport(t *testing.T) {
	t.Parallel()

	assertViewportBounds(t, testModel(80, 24).View(), 80, 24)
	assertViewportBounds(t, testModel(42, 16).View(), 42, 16)
}

func TestDashboardViewShowsConsoleChatLayout(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	timeline := stripANSI(model.renderChatTimeline(80))
	if !strings.Contains(timeline, branding.Wordmark[0]) {
		t.Fatalf("expected brand wordmark in scrollable timeline, got %q", timeline)
	}
	if !strings.Contains(timeline, "Version") {
		t.Fatalf("expected version line in timeline content, got %q", timeline)
	}
	view := stripANSI(model.View())
	if strings.Contains(view, "RECENT RUNS") {
		t.Fatalf("did not expect legacy recent runs panel, got %q", view)
	}
	if !strings.Contains(view, "USER") || !strings.Contains(view, "ORCH") {
		t.Fatalf("expected user and assistant labels, got %q", view)
	}
	if strings.Contains(view, "Ctrl+N/P") {
		t.Fatalf("did not expect legacy paging help, got %q", view)
	}
	if !strings.Contains(view, "PgUp/PgDn") {
		t.Fatalf("expected scroll help in view, got %q", view)
	}
	if !strings.Contains(view, "Up/Down Messages") {
		t.Fatalf("expected message history help in view, got %q", view)
	}
	if !strings.Contains(view, "/clear New Session") {
		t.Fatalf("expected new-session help for /clear, got %q", view)
	}
}

func TestCommandLineShowsRightAlignedCommandMeta(t *testing.T) {
	t.Parallel()

	model := testModel(100, 24)
	view := stripANSI(model.View())
	if !strings.Contains(view, "[ACTION] OLLAMA QWEN2.5-CODER") {
		t.Fatalf("expected action/provider/model meta on command row, got %q", view)
	}

	model.composerMode = domain.RunModePlan
	view = stripANSI(model.View())
	if !strings.Contains(view, "[PLAN] OLLAMA QWEN2.5-CODER") {
		t.Fatalf("expected plan/provider/model meta on command row, got %q", view)
	}
}

func TestCurrentRunShowsThinkingBox(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.snapshot.CurrentThinking = "thinking step 1"
	model.syncChatViewport(true)

	view := stripANSI(model.View())
	if !strings.Contains(view, "THINK") {
		t.Fatalf("expected think label, got %q", view)
	}
	if !strings.Contains(view, "thinking step 1") {
		t.Fatalf("expected think content, got %q", view)
	}
}

func TestCollapsedThinkingShowsPlaceholderOnly(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.snapshot.CurrentThinking = "thinking step 1"
	model.showThinking = false
	model.syncChatViewport(true)

	view := stripANSI(model.View())
	if !strings.Contains(view, "THINKING ...") {
		t.Fatalf("expected collapsed thinking placeholder, got %q", view)
	}
	if strings.Contains(view, "thinking step 1") {
		t.Fatalf("did not expect full thinking content when collapsed, got %q", view)
	}
}

func TestCollapsedThinkingAddsVerticalSpacing(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.snapshot.CurrentThinking = "thinking step 1"
	model.showThinking = false

	block := stripANSI(model.renderThinkingBlock(80))
	if !strings.HasPrefix(block, "\nTHINKING ...") {
		t.Fatalf("expected top spacing before collapsed thinking, got %q", block)
	}
	if !strings.HasSuffix(block, "THINKING ...\n") {
		t.Fatalf("expected bottom spacing after collapsed thinking, got %q", block)
	}
}

func TestRunSeparatorIsCenteredAtQuarterWidth(t *testing.T) {
	t.Parallel()

	line := stripANSI(renderRunSeparator(80))
	if strings.Count(line, "-") != 20 {
		t.Fatalf("expected 20 dash separator, got %q", line)
	}
	if got := strings.Index(line, "-"); got != 30 {
		t.Fatalf("expected centered separator to start at column 30, got %d in %q", got, line)
	}
}

func TestChatViewWrapsWithinNarrowPane(t *testing.T) {
	t.Parallel()

	model := testModel(42, 16)
	model.snapshot.CurrentThinking = strings.Repeat("thinking ", 8)
	model.snapshot.CurrentOutput = strings.Repeat("response ", 12)
	model.snapshot.Runs[0].Prompt = strings.Repeat("prompt ", 10)
	model.syncChatViewport(true)

	assertViewportBounds(t, model.View(), 42, 16)
}

func testModel(width int, height int) Model {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = "Describe the next request..."

	settings := domain.Settings{
		DefaultProvider: domain.ProviderOllama,
		Providers: domain.ProviderCatalog{
			Ollama: domain.ProviderSettings{
				BaseURL: "http://localhost:11434/v1",
				Model:   "qwen2.5-coder",
			},
		},
	}
	settings.Normalize()

	model := Model{
		input:         input,
		width:         width,
		height:        height,
		composerMode:  domain.RunModeReact,
		statusMessage: "ready",
		showThinking:  true,
		followOutput:  true,
		settings:      newSettingsModal(settings),
		snapshot: orchestrator.Snapshot{
			Settings:      settings,
			CurrentRunID:  "R1",
			CurrentOutput: "**done**\n[tool exec]\nREADME",
			Runs: []domain.RunRecord{
				{
					RunID:          "R1",
					Mode:           domain.RunModeReact,
					Provider:       domain.ProviderOllama,
					Model:          "qwen2.5-coder",
					Prompt:         "Refactor the runtime.",
					CurrentTask:    "Thinking",
					Status:         domain.StatusRunning,
					WorkspacePath:  "/tmp/test-workspace",
					CurrentCwd:     "/tmp/test-workspace",
					RalphIteration: 1,
				},
				{
					RunID:          "R0",
					Mode:           domain.RunModePlan,
					Provider:       domain.ProviderOllama,
					Model:          "qwen2.5-coder",
					Prompt:         "Document the provider settings.",
					CurrentTask:    "Completed",
					Status:         domain.StatusCompleted,
					WorkspacePath:  "/tmp/test-workspace",
					CurrentCwd:     "/tmp/test-workspace",
					RalphIteration: 2,
					FinalOutput:    "Plan drafted.",
				},
			},
			LastPrompt: "Refactor the runtime.",
		},
	}
	model.syncChatViewport(true)
	return model
}

func assertViewportBounds(t *testing.T, view string, width int, height int) {
	t.Helper()

	lines := strings.Split(view, "\n")
	if len(lines) != height {
		t.Fatalf("expected %d rendered lines, got %d: %q", height, len(lines), view)
	}
	for index, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line %d exceeds width %d: got %d in %q", index, width, got, line)
		}
	}
}

func stripANSI(value string) string {
	return ansi.Strip(value)
}

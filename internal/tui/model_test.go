package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCtrlTTogglesThinkingVisibility(t *testing.T) {
	t.Parallel()

	model := testModel(80, 24)
	model.snapshot.CurrentThinking = "thinking step 1"
	model.syncChatViewport(true)

	updatedModel, _ := model.updateDashboard(tea.KeyMsg{Type: tea.KeyCtrlT})
	updated := updatedModel.(Model)
	if updated.showThinking {
		t.Fatal("expected thinking to be hidden after ctrl+t")
	}
	if !strings.Contains(stripANSI(updated.body.View()), "THINKING ...") {
		t.Fatalf("expected collapsed thinking placeholder, got %q", stripANSI(updated.body.View()))
	}

	updatedModel, _ = updated.updateDashboard(tea.KeyMsg{Type: tea.KeyCtrlT})
	updated = updatedModel.(Model)
	if !updated.showThinking {
		t.Fatal("expected thinking to be shown after second ctrl+t")
	}
	if !strings.Contains(stripANSI(updated.body.View()), "thinking step 1") {
		t.Fatalf("expected expanded thinking content, got %q", stripANSI(updated.body.View()))
	}
}

func TestScrollKeysUpdateViewportOffset(t *testing.T) {
	t.Parallel()

	model := tallModel()
	if !model.body.AtBottom() {
		t.Fatal("expected tall model to start at bottom")
	}

	updatedModel, _ := model.updateDashboard(tea.KeyMsg{Type: tea.KeyHome})
	updated := updatedModel.(Model)
	if updated.body.YOffset != 0 {
		t.Fatalf("expected home to jump to top, got offset %d", updated.body.YOffset)
	}
	if updated.followOutput {
		t.Fatal("expected followOutput to be disabled after scrolling to top")
	}

	updatedModel, _ = updated.updateDashboard(tea.KeyMsg{Type: tea.KeyPgDown})
	updated = updatedModel.(Model)
	if updated.body.YOffset == 0 {
		t.Fatal("expected pgdown to move viewport down")
	}

	updatedModel, _ = updated.updateDashboard(tea.KeyMsg{Type: tea.KeyEnd})
	updated = updatedModel.(Model)
	if !updated.body.AtBottom() {
		t.Fatal("expected end to jump to bottom")
	}
	if !updated.followOutput {
		t.Fatal("expected followOutput to be re-enabled at bottom")
	}
}

func TestSyncChatViewportPreservesManualScroll(t *testing.T) {
	t.Parallel()

	model := tallModel()
	model.body.GotoTop()
	model.followOutput = false

	model.snapshot.CurrentOutput += "\nnew line one\nnew line two\nnew line three"
	model.syncChatViewport(false)

	if model.body.YOffset != 0 {
		t.Fatalf("expected manual scroll position to stay at top, got %d", model.body.YOffset)
	}
	if model.body.AtBottom() {
		t.Fatal("did not expect viewport to snap to bottom while followOutput is disabled")
	}
}

func TestSyncChatViewportFollowsBottomOnStreamUpdates(t *testing.T) {
	t.Parallel()

	model := tallModel()
	model.snapshot.CurrentOutput += "\nnew line one\nnew line two\nnew line three"
	model.syncChatViewport(true)

	if !model.body.AtBottom() {
		t.Fatal("expected viewport to stay pinned to bottom")
	}
}

func tallModel() Model {
	model := testModel(80, 18)
	model.snapshot.CurrentThinking = strings.Repeat("thinking line\n", 8)
	model.snapshot.CurrentOutput = strings.Repeat("response line\n", 24)
	model.snapshot.Runs[0].Prompt = strings.Repeat("prompt line\n", 12)
	model.syncChatViewport(true)
	return model
}

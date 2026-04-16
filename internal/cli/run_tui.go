package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/keonho-kim/orch/internal/apiserver"
	"github.com/keonho-kim/orch/internal/orchestrator"
	"github.com/keonho-kim/orch/internal/tui"
)

func runTUI(repoRoot string, restoreSessionID string, showHistory bool, restoreLatest bool) error {
	app, err := newApp(repoRoot, orchestrator.BootOptions{RestoreSessionID: restoreSessionID})
	if err != nil {
		return err
	}
	defer app.close()

	if restoreLatest {
		sessionID, err := app.service.LatestSessionID()
		if err != nil {
			return err
		}
		if err := app.service.RestoreSession(sessionID); err != nil {
			return err
		}
	}

	if isTTY(os.Stdout) {
		if _, err := io.WriteString(os.Stdout, "\r\n"); err != nil {
			return fmt.Errorf("prepare tui output: %w", err)
		}
	}

	model := tui.New(app.service)
	apiStatus, apiEvent := startAttachedAPIServer(app, os.Stderr)
	if strings.TrimSpace(apiStatus) != "" {
		model.SetStatusMessage(apiStatus)
	}
	if apiEvent.Type != "" {
		app.service.EmitEvent(apiEvent)
	}
	if showHistory {
		model.OpenHistoryPicker()
	}

	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithContext(app.ctx),
	)

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}

	return nil
}

func startAttachedAPIServer(app *app, stderr io.Writer) (string, orchestrator.ServiceEvent) {
	server, err := apiserver.Start(app.ctx, app.service, app.paths)
	if err != nil {
		message := "Attached API server failed to start."
		_, _ = fmt.Fprintf(stderr, "orch api: %v\n", err)
		return message + " See stderr for details.", orchestrator.ServiceEvent{}
	}
	app.api = server

	discovery := server.Discovery()
	currentPath := filepath.Join(app.paths.APIDir, "current.json")
	_, _ = fmt.Fprintf(stderr, "orch api: ready at %s (details: %s)\n", discovery.BaseURL, currentPath)

	message := fmt.Sprintf("Local API ready at %s. See %s for connection details.", discovery.BaseURL, currentPath)
	return message, orchestrator.ServiceEvent{
		Type:      "api_server_ready",
		SessionID: discovery.SessionID,
		Message:   "API server ready.",
		Payload: map[string]any{
			"base_url":   discovery.BaseURL,
			"session_id": discovery.SessionID,
			"repo_root":  discovery.RepoRoot,
		},
	}
}

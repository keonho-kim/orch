package cli

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/keonho-kim/orch/internal/apiserver"
	"github.com/keonho-kim/orch/internal/config"
	"github.com/keonho-kim/orch/internal/orchestrator"
	sqlitestore "github.com/keonho-kim/orch/internal/store/sqlite"
	"github.com/keonho-kim/orch/internal/tooling"
)

type app struct {
	ctx          context.Context
	cancel       context.CancelFunc
	store        *sqlitestore.Store
	service      *orchestrator.Service
	paths        config.Paths
	api          *apiserver.Server
	skipFinalize bool
}

func newApp(repoRoot string, configFile string, options orchestrator.BootOptions) (*app, error) {
	paths, err := resolveAppPaths(repoRoot, configFile)
	if err != nil {
		return nil, err
	}
	if err := config.EnsureRuntimePaths(paths); err != nil {
		return nil, err
	}

	store, err := sqlitestore.Open(paths.DBPath)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	service, err := orchestrator.NewService(ctx, store, tooling.NewExecutor(), paths, options)
	if err != nil {
		cancel()
		_ = store.Close()
		return nil, err
	}

	return &app{
		ctx:     ctx,
		cancel:  cancel,
		store:   store,
		service: service,
		paths:   paths,
	}, nil
}

func (a *app) close() {
	if a.api != nil {
		_ = a.api.Close()
	}
	a.cancel()
	if a.service != nil && !a.skipFinalize {
		_ = spawnFinalizeProcess(a.service.Snapshot().CurrentSession.SessionID, a.paths.RepoRoot)
	}
	if a.store != nil {
		_ = a.store.Close()
	}
}

func runFinalizeSession(repoRoot string, sessionID string) error {
	app, err := newApp(repoRoot, "", orchestrator.BootOptions{RestoreSessionID: sessionID})
	if err != nil {
		return err
	}
	app.skipFinalize = true
	defer app.close()
	return app.service.FinalizeCurrentSession()
}

func spawnFinalizeProcess(sessionID string, repoRoot string) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}

	command := exec.Command(executable, "__finalize-session", sessionID, repoRoot)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	command.Stdin = nil
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

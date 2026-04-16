package tooling

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
)

func (r *OTRunner) runSubagent(
	ctx context.Context,
	workspaceRoot string,
	record domain.RunRecord,
	env []string,
	inspection otInspection,
) (string, error) {
	title := strings.TrimSpace(inspection.Prompt)
	if len(title) > 72 {
		title = title[:72]
	}
	return r.RunDelegateTask(ctx, workspaceRoot, record, env, domain.SubagentTask{
		ID:       fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Title:    title,
		Contract: inspection.Prompt,
	}, true)
}

func (r *OTRunner) RunDelegateTask(
	ctx context.Context,
	workspaceRoot string,
	record domain.RunRecord,
	env []string,
	task domain.SubagentTask,
	wait bool,
) (string, error) {
	if subagentDepth(env) > 0 {
		return "", fmt.Errorf("nested ot subagent runs are not allowed")
	}
	if strings.TrimSpace(task.Contract) == "" {
		return "", fmt.Errorf("subagent task contract is required")
	}

	repoRoot, err := resolveSubagentRepoRoot(workspaceRoot, env)
	if err != nil {
		return "", err
	}

	executable, err := resolveOrchExecutable()
	if err != nil {
		return "", err
	}

	parentSessionID := strings.TrimSpace(record.SessionID)
	if parentSessionID == "" {
		parentSessionID = subagentPlaceholder
	}
	parentRunID := strings.TrimSpace(record.RunID)
	if parentRunID == "" {
		parentRunID = subagentPlaceholder
	}
	encodedTask, err := json.Marshal(task)
	if err != nil {
		return "", fmt.Errorf("marshal subagent task: %w", err)
	}

	request := domain.ExecRequest{
		Command: executable,
		Args: []string{
			"__subagent-run",
			repoRoot,
			parentSessionID,
			parentRunID,
			string(encodedTask),
		},
		Cwd: requestCwdOrWorkspace(record),
	}

	nextEnv := append([]string(nil), env...)
	nextEnv = append(nextEnv, fmt.Sprintf("%s=%d", subagentDepthEnv, subagentDepth(env)+1))
	if wait {
		return runExternal(ctx, workspaceRoot, record, nextEnv, request)
	}

	startFile, err := os.CreateTemp("", "orch-subagent-start-*.json")
	if err != nil {
		return "", fmt.Errorf("create async start file: %w", err)
	}
	startPath := startFile.Name()
	if err := startFile.Close(); err != nil {
		return "", fmt.Errorf("close async start file: %w", err)
	}
	if err := os.Remove(startPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("prepare async start file: %w", err)
	}
	defer os.Remove(startPath)

	task.StartFilePath = startPath
	encodedTask, err = json.Marshal(task)
	if err != nil {
		return "", fmt.Errorf("marshal async subagent task: %w", err)
	}

	command := exec.Command(executable,
		"__subagent-run",
		repoRoot,
		parentSessionID,
		parentRunID,
		string(encodedTask),
	)
	command.Dir = request.Cwd
	command.Env = append([]string(nil), nextEnv...)
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return "", fmt.Errorf("open devnull: %w", err)
	}
	defer devNull.Close()
	command.Stdout = devNull
	command.Stderr = devNull
	if err := command.Start(); err != nil {
		return "", fmt.Errorf("start async subagent: %w", err)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- command.Wait()
	}()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	for {
		data, err := os.ReadFile(startPath)
		if err == nil && len(data) > 0 {
			return strings.TrimSpace(string(data)), nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("read async start file: %w", err)
		}

		select {
		case err := <-waitDone:
			if err != nil {
				return "", fmt.Errorf("async subagent exited before start handshake: %w", err)
			}
			return "", fmt.Errorf("async subagent exited before start handshake")
		case <-ctx.Done():
			_ = command.Process.Kill()
			return "", ctx.Err()
		case <-timeout.C:
			_ = command.Process.Kill()
			return "", fmt.Errorf("timed out waiting for async subagent start handshake")
		case <-ticker.C:
		}
	}
}

func subagentDepth(env []string) int {
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || key != subagentDepthEnv {
			continue
		}

		depth := 0
		if _, err := fmt.Sscanf(value, "%d", &depth); err == nil && depth > 0 {
			return depth
		}
	}
	return 0
}

func resolveOrchExecutable() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}

	if filepath.Base(executable) != "ot" {
		return executable, nil
	}

	sibling := filepath.Join(filepath.Dir(executable), "orch")
	if _, err := os.Stat(sibling); err == nil {
		return sibling, nil
	}

	if lookedUp, err := exec.LookPath("orch"); err == nil {
		return lookedUp, nil
	}
	return "", fmt.Errorf("resolve orch executable from %s", executable)
}

func resolveSubagentRepoRoot(workspaceRoot string, env []string) (string, error) {
	for _, candidate := range []string{
		envValueByKey(env, "OT_REPO_ROOT"),
		envValueByKey(env, "PWD"),
		workspaceRoot,
	} {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		discovered, err := config.DiscoverRepoRoot(candidate)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(discovered) != "" {
			return discovered, nil
		}
		if config.LooksLikeRepoRoot(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("resolve repo root from workspace %s", workspaceRoot)
}

func requestCwdOrWorkspace(record domain.RunRecord) string {
	if strings.TrimSpace(record.CurrentCwd) != "" {
		return record.CurrentCwd
	}
	return record.WorkspacePath
}

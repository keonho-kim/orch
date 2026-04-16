package tooling

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
)

const maxCommandOutputBytes = 64000

func normalizeWorkspacePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return "."
	}
	return strings.TrimSpace(path)
}

func normalizeWorkspaceRelativePath(workspaceRoot string, record domain.RunRecord, path string) (string, error) {
	resolved, err := resolveCommandPath(workspaceRoot, baseCwd(record), normalizeWorkspacePath(path))
	if err != nil {
		return "", err
	}
	return displayRelativePath(workspaceRoot, resolved), nil
}

func runExternal(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.ExecRequest) (string, error) {
	cwd, err := resolveExecutionCwd(workspaceRoot, record, request)
	if err != nil {
		return "", err
	}

	runCtx := ctx
	cancel := func() {}
	if request.TimeoutSec > 0 {
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(request.TimeoutSec)*time.Second)
	}
	defer cancel()

	command := exec.CommandContext(runCtx, request.Command, request.Args...)
	command.Dir = cwd
	command.Env = baseEnv(workspaceRoot, env)
	if request.Stdin != "" {
		command.Stdin = strings.NewReader(request.Stdin)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		combined := truncateOutput(stdout.String() + stderr.String())
		if combined == "" {
			return "", fmt.Errorf("run %s: %w", request.Command, err)
		}
		return "", fmt.Errorf("run %s: %w: %s", request.Command, err, combined)
	}

	return truncateOutput(stdout.String() + stderr.String()), nil
}

func resolveExecutionCwd(workspaceRoot string, record domain.RunRecord, request domain.ExecRequest) (string, error) {
	raw := strings.TrimSpace(request.Cwd)
	if raw == "" {
		return baseCwd(record), nil
	}
	return resolveCommandPath(workspaceRoot, baseCwd(record), raw)
}

func resolveCommandPath(workspaceRoot string, base string, raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" || cleaned == "." {
		return filepath.Clean(base), nil
	}

	var candidate string
	if filepath.IsAbs(cleaned) {
		candidate = filepath.Clean(cleaned)
	} else {
		candidate = filepath.Clean(filepath.Join(base, cleaned))
	}

	rel, err := filepath.Rel(workspaceRoot, candidate)
	if err != nil {
		return "", fmt.Errorf("compute relative path for %s: %w", candidate, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes workspace root", raw)
	}
	return candidate, nil
}

func baseCwd(record domain.RunRecord) string {
	if strings.TrimSpace(record.CurrentCwd) != "" {
		return record.CurrentCwd
	}
	return record.WorkspacePath
}

func displayRelativePath(workspaceRoot string, path string) string {
	rel, err := filepath.Rel(workspaceRoot, path)
	if err != nil || rel == "." {
		return "."
	}
	return filepath.ToSlash(rel)
}

func truncateOutput(value string) string {
	if len(value) <= maxCommandOutputBytes {
		return value
	}
	return value[len(value)-maxCommandOutputBytes:]
}

func runOTBinary(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.ExecRequest) (string, error) {
	executable, err := resolveOTExecutable(env)
	if err != nil {
		return "", err
	}

	cwd, err := resolveExecutionCwd(workspaceRoot, record, request)
	if err != nil {
		return "", err
	}

	runCtx := ctx
	cancel := func() {}
	if request.TimeoutSec > 0 {
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(request.TimeoutSec)*time.Second)
	}
	defer cancel()

	command := exec.CommandContext(runCtx, executable, request.Args...)
	command.Dir = cwd
	command.Env = baseEnv(workspaceRoot, env)
	if request.Stdin != "" {
		command.Stdin = strings.NewReader(request.Stdin)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		combined := truncateOutput(stdout.String() + stderr.String())
		if combined == "" {
			return "", fmt.Errorf("run ot: %w", err)
		}
		return "", fmt.Errorf("run ot: %w: %s", err, combined)
	}

	return truncateOutput(stdout.String() + stderr.String()), nil
}

func resolveOTExecutable(env []string) (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}

	return resolveOTExecutableFrom(executable, func(name string) (string, error) {
		return lookPathInEnv(name, env)
	})
}

func resolveOTExecutableFrom(executable string, lookPath func(string) (string, error)) (string, error) {
	if filepath.Base(executable) == "ot" {
		return executable, nil
	}

	sibling := filepath.Join(filepath.Dir(executable), "ot")
	if _, err := os.Stat(sibling); err == nil {
		return sibling, nil
	}

	if lookedUp, err := lookPath("ot"); err == nil {
		return lookedUp, nil
	}
	return "", fmt.Errorf("resolve ot executable from %s", executable)
}

func lookPathInEnv(name string, env []string) (string, error) {
	if strings.Contains(name, string(filepath.Separator)) {
		if isExecutableFile(name) {
			return name, nil
		}
		return "", fmt.Errorf("%s is not executable", name)
	}

	pathValue := envValueByKey(env, "PATH")
	if strings.TrimSpace(pathValue) == "" {
		pathValue = os.Getenv("PATH")
	}

	for _, dir := range filepath.SplitList(pathValue) {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		if isExecutableFile(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("look up %s in PATH", name)
}

func envValueByKey(env []string, key string) string {
	for _, entry := range env {
		currentKey, value, ok := strings.Cut(entry, "=")
		if ok && currentKey == key {
			return value
		}
	}
	return ""
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}

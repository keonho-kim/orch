package tooling

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"orch/domain"
)

type OTRunner struct{}

var supportedOTSubcommands = map[string]struct{}{
	"exec":  {},
	"patch": {},
	"read":  {},
	"write": {},
}

func NewOTRunner() *OTRunner {
	return &OTRunner{}
}

func (r *OTRunner) Run(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.ExecRequest) (string, error) {
	if len(request.Args) == 0 {
		return "", fmt.Errorf("ot requires a subcommand")
	}

	subcommand := strings.TrimSpace(request.Args[0])
	if subcommand == "" {
		return "", fmt.Errorf("ot subcommand is required")
	}
	if _, ok := supportedOTSubcommands[subcommand]; !ok {
		return "", fmt.Errorf("ot %s is not supported", subcommand)
	}

	normalizedArgs, err := normalizeOTArgs(workspaceRoot, record, subcommand, request.Args[1:])
	if err != nil {
		return "", err
	}

	scriptPath := filepath.Join(workspaceRoot, "tools", "ot", subcommand+".sh")
	if _, err := os.Stat(scriptPath); err != nil {
		return "", fmt.Errorf("resolve ot subcommand %s: %w", subcommand, err)
	}

	scriptRequest := domain.ExecRequest{
		Command:    "bash",
		Args:       append([]string{filepath.ToSlash(filepath.Join("tools", "ot", subcommand+".sh"))}, normalizedArgs...),
		Cwd:        request.Cwd,
		TimeoutSec: request.TimeoutSec,
		Stdin:      request.Stdin,
	}

	return runExternal(ctx, workspaceRoot, record, env, scriptRequest)
}

func normalizeOTArgs(workspaceRoot string, record domain.RunRecord, subcommand string, args []string) ([]string, error) {
	switch subcommand {
	case "read":
		return normalizeOTReadArgs(workspaceRoot, record, args)
	case "write":
		return normalizeOTWriteArgs(workspaceRoot, record, args)
	default:
		return append([]string(nil), args...), nil
	}
}

func normalizeOTReadArgs(workspaceRoot string, record domain.RunRecord, args []string) ([]string, error) {
	path, start, end, err := parseOTReadArgs(args)
	if err != nil {
		return nil, err
	}

	resolved, err := resolveCommandPath(workspaceRoot, baseCwd(record), path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("stat read path %q: %w", path, err)
	}
	if info.IsDir() && (start != "" || end != "") {
		return nil, fmt.Errorf("ot read line ranges are only supported for files")
	}

	normalized := []string{"--path", displayRelativePath(workspaceRoot, resolved)}
	if start != "" {
		normalized = append(normalized, "--start", start)
	}
	if end != "" {
		normalized = append(normalized, "--end", end)
	}
	return normalized, nil
}

func parseOTReadArgs(args []string) (string, string, string, error) {
	path := ""
	start := ""
	end := ""

	for index := 0; index < len(args); {
		switch args[index] {
		case "--path":
			if index+1 >= len(args) {
				return "", "", "", fmt.Errorf("--path is required")
			}
			path = strings.TrimSpace(args[index+1])
			index += 2
		case "--start":
			if index+1 >= len(args) {
				return "", "", "", fmt.Errorf("--start requires a value")
			}
			start = strings.TrimSpace(args[index+1])
			index += 2
		case "--end":
			if index+1 >= len(args) {
				return "", "", "", fmt.Errorf("--end requires a value")
			}
			end = strings.TrimSpace(args[index+1])
			index += 2
		default:
			return "", "", "", fmt.Errorf("unknown ot read arg: %s", args[index])
		}
	}

	if path == "" {
		return "", "", "", fmt.Errorf("--path is required")
	}
	return path, start, end, nil
}

func normalizeOTWriteArgs(workspaceRoot string, record domain.RunRecord, args []string) ([]string, error) {
	path, fromStdin, err := parseOTWriteArgs(args)
	if err != nil {
		return nil, err
	}

	resolved, err := resolveCommandPath(workspaceRoot, baseCwd(record), path)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(resolved); err == nil && info.IsDir() {
		return nil, fmt.Errorf("ot write requires a file path, not a directory")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat write path %q: %w", path, err)
	}

	normalized := []string{"--path", displayRelativePath(workspaceRoot, resolved)}
	if fromStdin {
		normalized = append(normalized, "--from-stdin")
	}
	return normalized, nil
}

func parseOTWriteArgs(args []string) (string, bool, error) {
	path := ""
	fromStdin := false

	for index := 0; index < len(args); {
		switch args[index] {
		case "--path":
			if index+1 >= len(args) {
				return "", false, fmt.Errorf("--path is required")
			}
			path = strings.TrimSpace(args[index+1])
			index += 2
		case "--from-stdin":
			fromStdin = true
			index++
		default:
			return "", false, fmt.Errorf("unknown ot write arg: %s", args[index])
		}
	}

	if path == "" {
		return "", false, fmt.Errorf("--path is required")
	}
	if !fromStdin {
		return "", false, fmt.Errorf("--from-stdin is required")
	}
	return path, fromStdin, nil
}

func baseEnv(workspaceRoot string, env []string) []string {
	base := append([]string(nil), env...)
	repoRoot := workspaceRoot
	if cwd, err := os.Getwd(); err == nil {
		repoRoot = cwd
	}
	base = append(base, "OT_WORKSPACE_ROOT="+workspaceRoot)
	base = append(base, "OT_REPO_ROOT="+repoRoot)
	base = append(base, "PATH="+prefixedPath(env))
	return base
}

func prefixedPath(env []string) string {
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok && key == "PATH" {
			return value
		}
	}

	path, err := exec.LookPath("bash")
	if err != nil {
		return ""
	}
	return filepath.Dir(path)
}

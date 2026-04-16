package cli

import (
	"fmt"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

type command struct {
	name             string
	prompt           string
	mode             domain.RunMode
	repoRoot         string
	configCommand    configCommandState
	restoreSessionID string
	showHistory      bool
	restoreLatest    bool
	finalizeSession  string
	subagentTask     string
	parentSessionID  string
	parentRunID      string
}

func parseCommand(args []string) (command, error) {
	if len(args) == 0 {
		return command{name: "interactive", repoRoot: "."}, nil
	}
	if args[0] == "--workspace" {
		repoRoot, rest, err := parseWorkspaceFlag(args, ".")
		if err != nil {
			return command{}, err
		}
		if len(rest) != 0 {
			return command{}, fmt.Errorf("unsupported command %q", rest[0])
		}
		return command{name: "interactive", repoRoot: repoRoot}, nil
	}

	switch args[0] {
	case "exec":
		repoRoot, rest, err := parseWorkspaceFlag(args[1:], ".")
		if err != nil {
			return command{}, err
		}
		mode := domain.RunModeReact
		if len(rest) >= 2 && rest[0] == "--mode" {
			parsedMode, err := domain.ParseRunMode(rest[1])
			if err != nil {
				return command{}, err
			}
			mode = parsedMode
			rest = rest[2:]
		}
		if len(rest) == 0 || strings.TrimSpace(strings.Join(rest, " ")) == "" {
			return command{}, fmt.Errorf("usage: orch exec [--workspace <path>] [--mode react|plan] \"<request>\"")
		}
		return command{name: "exec", prompt: strings.Join(rest, " "), mode: mode, repoRoot: repoRoot}, nil
	case "history":
		repoRoot, rest, err := parseWorkspaceFlag(args[1:], ".")
		if err != nil {
			return command{}, err
		}
		switch {
		case len(rest) == 0:
			return command{name: "interactive", repoRoot: repoRoot, showHistory: true}, nil
		case rest[0] == "--latest":
			return command{name: "interactive", repoRoot: repoRoot, restoreLatest: true}, nil
		default:
			return command{name: "interactive", repoRoot: repoRoot, restoreSessionID: rest[0]}, nil
		}
	case "config":
		return parseConfigCommand(args[1:])
	case "__finalize-session":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return command{}, fmt.Errorf("usage: orch __finalize-session <session-id> [workspace]")
		}
		repoRoot := "."
		if len(args) > 2 {
			repoRoot = args[2]
		}
		return command{name: "__finalize-session", finalizeSession: args[1], repoRoot: repoRoot}, nil
	case "__subagent-run":
		if len(args) != 5 || strings.TrimSpace(args[1]) == "" || strings.TrimSpace(args[4]) == "" {
			return command{}, fmt.Errorf("usage: orch __subagent-run <repo-root> <parent-session-id|-> <parent-run-id|-> <task-json>")
		}
		return command{
			name:            "__subagent-run",
			repoRoot:        args[1],
			parentSessionID: hiddenValue(args[2]),
			parentRunID:     hiddenValue(args[3]),
			subagentTask:    args[4],
		}, nil
	default:
		repoRoot, rest, err := parseWorkspaceFlag(args, ".")
		if err != nil {
			return command{}, err
		}
		prompt := strings.TrimSpace(strings.Join(rest, " "))
		if prompt == "" {
			return command{}, fmt.Errorf("usage: orch [--workspace <path>] <request>")
		}
		return command{name: "exec", prompt: prompt, mode: domain.RunModeReact, repoRoot: repoRoot}, nil
	}
}

func parseWorkspaceFlag(args []string, defaultRepoRoot string) (string, []string, error) {
	repoRoot := defaultRepoRoot
	rest := make([]string, 0, len(args))
	seen := false

	for index := 0; index < len(args); index++ {
		if args[index] != "--workspace" {
			rest = append(rest, args[index])
			continue
		}
		if seen {
			return "", nil, fmt.Errorf("--workspace may only be provided once")
		}
		if index+1 >= len(args) || strings.TrimSpace(args[index+1]) == "" {
			return "", nil, fmt.Errorf("--workspace requires a path")
		}
		repoRoot = args[index+1]
		seen = true
		index++
	}

	return repoRoot, rest, nil
}

func hiddenValue(value string) string {
	if strings.TrimSpace(value) == "-" {
		return ""
	}
	return value
}

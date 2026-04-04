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
	configFile       string
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
	if strings.HasPrefix(args[0], "--") {
		return parseInteractiveCommand(args)
	}

	switch args[0] {
	case "exec":
		return parseExecCommand(args[1:])
	case "history":
		return parseHistoryCommand(args[1:])
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
		return command{}, fmt.Errorf("unsupported command %q", args[0])
	}
}

func parseInteractiveCommand(args []string) (command, error) {
	repoRoot, configFile, rest, err := parseGlobalFlags(args)
	if err != nil {
		return command{}, err
	}
	if len(rest) != 0 {
		return command{}, fmt.Errorf("unsupported command %q", rest[0])
	}
	return command{name: "interactive", repoRoot: repoRoot, configFile: configFile}, nil
}

func parseExecCommand(args []string) (command, error) {
	repoRoot, configFile, rest, err := parseGlobalFlags(args)
	if err != nil {
		return command{}, err
	}
	mode, rest, err := parseExecMode(rest)
	if err != nil {
		return command{}, err
	}
	prompt := strings.TrimSpace(strings.Join(rest, " "))
	if prompt == "" {
		return command{}, fmt.Errorf("usage: orch exec [--workspace <path>] [--env-file <path>] [--mode react|plan] \"<request>\"")
	}
	return command{name: "exec", prompt: prompt, mode: mode, repoRoot: repoRoot, configFile: configFile}, nil
}

func parseExecMode(args []string) (domain.RunMode, []string, error) {
	mode := domain.RunModeReact
	if len(args) >= 2 && args[0] == "--mode" {
		parsedMode, err := domain.ParseRunMode(args[1])
		if err != nil {
			return "", nil, err
		}
		mode = parsedMode
		args = args[2:]
	}
	return mode, args, nil
}

func parseHistoryCommand(args []string) (command, error) {
	repoRoot, configFile, rest, err := parseGlobalFlags(args)
	if err != nil {
		return command{}, err
	}
	switch {
	case len(rest) == 0:
		return command{name: "interactive", repoRoot: repoRoot, configFile: configFile, showHistory: true}, nil
	case rest[0] == "--latest":
		return command{name: "interactive", repoRoot: repoRoot, configFile: configFile, restoreLatest: true}, nil
	default:
		return command{name: "interactive", repoRoot: repoRoot, configFile: configFile, restoreSessionID: rest[0]}, nil
	}
}

func parseGlobalFlags(args []string) (string, string, []string, error) {
	repoRoot := "."
	configFile := ""
	rest := make([]string, 0, len(args))
	seenWorkspace := false
	seenConfigFile := false

	for index := 0; index < len(args); index++ {
		if args[index] != "--workspace" && args[index] != "--env-file" {
			rest = append(rest, args[index])
			continue
		}
		if index+1 >= len(args) || strings.TrimSpace(args[index+1]) == "" {
			if args[index] == "--workspace" {
				return "", "", nil, fmt.Errorf("--workspace requires a path")
			}
			return "", "", nil, fmt.Errorf("--env-file requires a path")
		}
		switch args[index] {
		case "--workspace":
			if seenWorkspace {
				return "", "", nil, fmt.Errorf("--workspace may only be provided once")
			}
			repoRoot = args[index+1]
			seenWorkspace = true
		case "--env-file":
			if seenConfigFile {
				return "", "", nil, fmt.Errorf("--env-file may only be provided once")
			}
			configFile = args[index+1]
			seenConfigFile = true
		}
		index++
	}

	return repoRoot, configFile, rest, nil
}

func hiddenValue(value string) string {
	if strings.TrimSpace(value) == "-" {
		return ""
	}
	return value
}

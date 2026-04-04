package tooling

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

type otInspection struct {
	Subcommand      string
	NormalizedArgs  []string
	WithinWorkspace bool
	Prompt          string
}

var supportedOTSubcommands = map[string]struct{}{
	"exec":     {},
	"list":     {},
	"patch":    {},
	"pointer":  {},
	"read":     {},
	"search":   {},
	"subagent": {},
	"write":    {},
}

func inspectOTRequest(workspaceRoot string, record domain.RunRecord, request domain.ExecRequest) (otInspection, error) {
	if len(request.Args) == 0 {
		return otInspection{}, fmt.Errorf("ot requires a subcommand")
	}

	subcommand := strings.TrimSpace(request.Args[0])
	if subcommand == "" {
		return otInspection{}, fmt.Errorf("ot subcommand is required")
	}
	if _, ok := supportedOTSubcommands[subcommand]; !ok {
		return otInspection{}, fmt.Errorf("ot %s is not supported", subcommand)
	}

	args := request.Args[1:]
	switch subcommand {
	case "read":
		return inspectOTRead(workspaceRoot, record, args)
	case "pointer":
		return inspectOTPointer(args)
	case "list":
		return inspectOTList(workspaceRoot, record, args)
	case "search":
		return inspectOTSearch(workspaceRoot, record, args)
	case "subagent":
		return inspectOTSubagent(args)
	case "write":
		return inspectOTWrite(workspaceRoot, record, args)
	default:
		return otInspection{
			Subcommand:      subcommand,
			NormalizedArgs:  append([]string(nil), args...),
			WithinWorkspace: true,
		}, nil
	}
}

func inspectOTSubagent(args []string) (otInspection, error) {
	prompt, err := parseOTSubagentArgs(args)
	if err != nil {
		return otInspection{}, err
	}

	return otInspection{
		Subcommand:      "subagent",
		NormalizedArgs:  []string{"--prompt", prompt},
		WithinWorkspace: true,
		Prompt:          prompt,
	}, nil
}

func inspectOTPointer(args []string) (otInspection, error) {
	value, err := parseOTPointerArgs(args)
	if err != nil {
		return otInspection{}, err
	}
	return otInspection{
		Subcommand:      "pointer",
		NormalizedArgs:  []string{"--value", value},
		WithinWorkspace: true,
		Prompt:          value,
	}, nil
}

func inspectOTRead(workspaceRoot string, record domain.RunRecord, args []string) (otInspection, error) {
	path, start, end, err := parseOTReadArgs(args)
	if err != nil {
		return otInspection{}, err
	}

	resolved, info, withinWorkspace, err := inspectReadOnlyTarget(workspaceRoot, baseCwd(record), path)
	if err != nil {
		return otInspection{}, err
	}
	if info.IsDir() && (start != "" || end != "") {
		return otInspection{}, fmt.Errorf("ot read line ranges are only supported for files")
	}

	normalized := normalizedReadOnlyArgs(workspaceRoot, resolved, withinWorkspace)
	if start != "" {
		normalized = append(normalized, "--start", start)
	}
	if end != "" {
		normalized = append(normalized, "--end", end)
	}
	return otInspection{
		Subcommand:      "read",
		NormalizedArgs:  normalized,
		WithinWorkspace: withinWorkspace,
	}, nil
}

func inspectOTList(workspaceRoot string, record domain.RunRecord, args []string) (otInspection, error) {
	path, err := parseOTListArgs(args)
	if err != nil {
		return otInspection{}, err
	}

	resolved, _, withinWorkspace, err := inspectReadOnlyTarget(workspaceRoot, baseCwd(record), path)
	if err != nil {
		return otInspection{}, err
	}

	return otInspection{
		Subcommand:      "list",
		NormalizedArgs:  normalizedReadOnlyArgs(workspaceRoot, resolved, withinWorkspace),
		WithinWorkspace: withinWorkspace,
	}, nil
}

func inspectOTSearch(workspaceRoot string, record domain.RunRecord, args []string) (otInspection, error) {
	path, name, content, err := parseOTSearchArgs(args)
	if err != nil {
		return otInspection{}, err
	}

	resolved, _, withinWorkspace, err := inspectReadOnlyTarget(workspaceRoot, baseCwd(record), path)
	if err != nil {
		return otInspection{}, err
	}

	normalized := normalizedReadOnlyArgs(workspaceRoot, resolved, withinWorkspace)
	if name != "" {
		normalized = append(normalized, "--name", name)
	}
	if content != "" {
		normalized = append(normalized, "--content", content)
	}

	return otInspection{
		Subcommand:      "search",
		NormalizedArgs:  normalized,
		WithinWorkspace: withinWorkspace,
	}, nil
}

func inspectOTWrite(workspaceRoot string, record domain.RunRecord, args []string) (otInspection, error) {
	path, fromStdin, err := parseOTWriteArgs(args)
	if err != nil {
		return otInspection{}, err
	}

	resolved, err := resolveCommandPath(workspaceRoot, baseCwd(record), path)
	if err != nil {
		return otInspection{}, err
	}
	if info, err := os.Stat(resolved); err == nil && info.IsDir() {
		return otInspection{}, fmt.Errorf("ot write requires a file path, not a directory")
	} else if err != nil && !os.IsNotExist(err) {
		return otInspection{}, fmt.Errorf("stat write path %q: %w", path, err)
	}

	normalized := []string{"--path", displayRelativePath(workspaceRoot, resolved)}
	if fromStdin {
		normalized = append(normalized, "--from-stdin")
	}
	return otInspection{
		Subcommand:      "write",
		NormalizedArgs:  normalized,
		WithinWorkspace: true,
	}, nil
}

func inspectReadOnlyTarget(workspaceRoot string, base string, rawPath string) (string, os.FileInfo, bool, error) {
	resolved := resolveInspectablePath(base, rawPath)

	info, err := os.Stat(resolved)
	if err != nil {
		return "", nil, false, fmt.Errorf("stat path %q: %w", rawPath, err)
	}

	withinWorkspace := isPathInsideWorkspace(workspaceRoot, resolved)
	if !withinWorkspace && pathContainsHiddenSegment(resolved) {
		return "", nil, false, fmt.Errorf("path %q contains hidden segments outside the workspace", rawPath)
	}
	return resolved, info, withinWorkspace, nil
}

func normalizedReadOnlyArgs(workspaceRoot string, resolved string, withinWorkspace bool) []string {
	scope := otScopeOutside
	display := otDisplayAbsolute
	if withinWorkspace {
		scope = otScopeInside
		display = otDisplayRelative
	}

	return []string{
		"--target", filepath.Clean(resolved),
		"--scope", scope,
		"--display", display,
		"--workspace-root", filepath.Clean(workspaceRoot),
	}
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

func parseOTListArgs(args []string) (string, error) {
	path := "."

	for index := 0; index < len(args); {
		switch args[index] {
		case "--path":
			if index+1 >= len(args) {
				return "", fmt.Errorf("--path requires a value")
			}
			path = strings.TrimSpace(args[index+1])
			index += 2
		default:
			return "", fmt.Errorf("unknown ot list arg: %s", args[index])
		}
	}

	if path == "" {
		path = "."
	}
	return path, nil
}

func parseOTSearchArgs(args []string) (string, string, string, error) {
	path := "."
	name := ""
	content := ""

	for index := 0; index < len(args); {
		switch args[index] {
		case "--path":
			if index+1 >= len(args) {
				return "", "", "", fmt.Errorf("--path requires a value")
			}
			path = strings.TrimSpace(args[index+1])
			index += 2
		case "--name":
			if index+1 >= len(args) {
				return "", "", "", fmt.Errorf("--name requires a value")
			}
			name = strings.TrimSpace(args[index+1])
			index += 2
		case "--content":
			if index+1 >= len(args) {
				return "", "", "", fmt.Errorf("--content requires a value")
			}
			content = strings.TrimSpace(args[index+1])
			index += 2
		default:
			return "", "", "", fmt.Errorf("unknown ot search arg: %s", args[index])
		}
	}

	if path == "" {
		path = "."
	}
	if name == "" && content == "" {
		return "", "", "", fmt.Errorf("ot search requires --name or --content")
	}
	return path, name, content, nil
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

func parseOTSubagentArgs(args []string) (string, error) {
	prompt := ""

	for index := 0; index < len(args); {
		switch args[index] {
		case "--prompt":
			if index+1 >= len(args) {
				return "", fmt.Errorf("--prompt requires a value")
			}
			prompt = strings.TrimSpace(args[index+1])
			index += 2
		default:
			return "", fmt.Errorf("unknown ot subagent arg: %s", args[index])
		}
	}

	if prompt == "" {
		return "", fmt.Errorf("ot subagent requires --prompt")
	}
	return prompt, nil
}

func parseOTPointerArgs(args []string) (string, error) {
	value := ""
	for index := 0; index < len(args); {
		switch args[index] {
		case "--value":
			if index+1 >= len(args) {
				return "", fmt.Errorf("--value requires a value")
			}
			value = strings.TrimSpace(args[index+1])
			index += 2
		default:
			return "", fmt.Errorf("unknown ot pointer arg: %s", args[index])
		}
	}
	if value == "" {
		return "", fmt.Errorf("ot pointer requires --value")
	}
	return value, nil
}

func resolveInspectablePath(base string, raw string) string {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" || cleaned == "." {
		return filepath.Clean(base)
	}

	if filepath.IsAbs(cleaned) {
		return filepath.Clean(cleaned)
	}
	return filepath.Clean(filepath.Join(base, cleaned))
}

func isPathInsideWorkspace(workspaceRoot string, path string) bool {
	rel, err := filepath.Rel(workspaceRoot, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func pathContainsHiddenSegment(path string) bool {
	cleaned := filepath.Clean(path)
	for _, segment := range strings.Split(cleaned, string(filepath.Separator)) {
		if segment == "" || segment == "." || segment == ".." {
			continue
		}
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}

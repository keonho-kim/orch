package tooling

import (
	"fmt"
	"strings"
)

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

package tui

import "strings"

type dashboardCommand struct {
	kind string
}

type slashCommandOption struct {
	value       string
	description string
}

const (
	commandNone    = ""
	commandExit    = "exit"
	commandClear   = "clear"
	commandCompact = "compact"
)

var slashCommandOptions = []slashCommandOption{
	{value: "/clear", description: "Open a new session"},
	{value: "/compact", description: "Compact current session"},
	{value: "/exit", description: "Quit orch"},
}

func parseDashboardCommand(value string) dashboardCommand {
	switch strings.TrimSpace(value) {
	case "/exit":
		return dashboardCommand{kind: commandExit}
	case "/clear":
		return dashboardCommand{kind: commandClear}
	case "/compact":
		return dashboardCommand{kind: commandCompact}
	}
	return dashboardCommand{}
}

func filteredSlashCommands(value string) []slashCommandOption {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "/") {
		return nil
	}

	filtered := make([]slashCommandOption, 0, len(slashCommandOptions))
	for _, option := range slashCommandOptions {
		if strings.HasPrefix(option.value, trimmed) {
			filtered = append(filtered, option)
		}
	}
	return filtered
}

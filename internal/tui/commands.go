package tui

import "strings"

type dashboardCommand struct {
	kind string
	arg  string
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
	commandStatus  = "status"
	commandContext = "context"
	commandTasks   = "tasks"
)

var slashCommandOptions = []slashCommandOption{
	{value: "/clear", description: "Open a new session"},
	{value: "/compact", description: "Compact current session"},
	{value: "/context", description: "Show current run context"},
	{value: "/exit", description: "Quit orch"},
	{value: "/status", description: "Show current session status"},
	{value: "/tasks", description: "Show delegated tasks"},
}

func parseDashboardCommand(value string) dashboardCommand {
	trimmed := strings.TrimSpace(value)
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return dashboardCommand{}
	}
	arg := strings.TrimSpace(strings.TrimPrefix(trimmed, fields[0]))
	switch fields[0] {
	case "/exit":
		return dashboardCommand{kind: commandExit}
	case "/clear":
		return dashboardCommand{kind: commandClear}
	case "/compact":
		return dashboardCommand{kind: commandCompact}
	case "/status":
		return dashboardCommand{kind: commandStatus}
	case "/context":
		return dashboardCommand{kind: commandContext}
	case "/tasks":
		return dashboardCommand{kind: commandTasks, arg: arg}
	}
	return dashboardCommand{}
}

func filteredSlashCommands(value string) []slashCommandOption {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "/") {
		return nil
	}
	if strings.Contains(trimmed, " ") {
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

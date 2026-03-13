package tui

import "strings"

type dashboardCommand struct {
	kind string
}

const (
	commandNone = ""
	commandExit = "exit"
)

func parseDashboardCommand(value string) dashboardCommand {
	if strings.TrimSpace(value) == "/exit" {
		return dashboardCommand{kind: commandExit}
	}
	return dashboardCommand{}
}

package cli

import (
	"fmt"
	"os"
)

func Run(args []string) error {
	command, err := parseCommand(args)
	if err != nil {
		return err
	}

	switch command.name {
	case "interactive":
		return runTUI(command.repoRoot, command.configFile, command.restoreSessionID, command.showHistory, command.restoreLatest)
	case "exec":
		return runExec(command.repoRoot, command.configFile, command.prompt, command.mode, os.Stdin, os.Stdout, os.Stderr)
	case "config-list":
		return runConfigList(command.repoRoot, command.configFile, os.Stdout)
	case "config-set":
<<<<<<< HEAD
		return runConfigUpdate(command.repoRoot, command.configFile, command.configCommand)
=======
		return runConfigUpdate(command.repoRoot, command.configCommand)
	case "config-migrate":
		return runConfigMigrate(command.repoRoot)
>>>>>>> cef7a8c (update)
	case "__finalize-session":
		return runFinalizeSession(command.repoRoot, command.finalizeSession)
	case "__subagent-run":
		return runSubagent(command.repoRoot, command.parentSessionID, command.parentRunID, command.subagentTask, os.Stdout)
	default:
		return fmt.Errorf("unsupported command %q", command.name)
	}
}

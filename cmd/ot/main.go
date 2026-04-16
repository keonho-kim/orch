package main

import (
	"context"
	"fmt"
	"os"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/buildinfo"
	"github.com/keonho-kim/orch/internal/tooling"
	helperbin "github.com/keonho-kim/orch/runtime-asset/helper-bin"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
	builtBy = ""
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ot: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	buildinfo.Set(version, commit, date, builtBy)

	if len(args) == 0 {
		return fmt.Errorf("usage: ot <subcommand> [args...]")
	}

	workspaceRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	output, err := tooling.NewOTRunnerWithScriptEnvPreparer(func(env []string) ([]string, error) {
		return helperbin.PrepareOTEnv(env, buildinfo.Version())
	}).Run(
		context.Background(),
		workspaceRoot,
		domain.RunRecord{WorkspacePath: workspaceRoot, CurrentCwd: workspaceRoot},
		os.Environ(),
		domain.ExecRequest{
			Command: "ot",
			Args:    args,
		},
	)
	if err != nil {
		return err
	}

	if output != "" {
		if _, err := fmt.Fprint(os.Stdout, output); err != nil {
			return err
		}
	}
	return nil
}

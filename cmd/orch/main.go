package main

import (
	"fmt"
	"os"

	"github.com/keonho-kim/orch/internal/buildinfo"
	"github.com/keonho-kim/orch/internal/cli"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
	builtBy = ""
)

func main() {
	buildinfo.Set(version, commit, date, builtBy)
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "orch: %v\n", err)
		os.Exit(1)
	}
}

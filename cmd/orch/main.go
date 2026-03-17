package main

import (
	"fmt"
	"os"

	"github.com/keonho-kim/orch/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "orch: %v\n", err)
		os.Exit(1)
	}
}

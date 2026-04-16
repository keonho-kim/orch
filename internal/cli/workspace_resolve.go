package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/keonho-kim/orch/internal/config"
)

func resolveAppPaths(repoRoot string) (config.Paths, error) {
	start := repoRoot
	if strings.TrimSpace(start) == "" || strings.TrimSpace(start) == "." {
		cwd, err := os.Getwd()
		if err != nil {
			return config.Paths{}, fmt.Errorf("resolve working directory: %w", err)
		}
		start = cwd
	}

	var resolvedRepoRoot string
	if strings.TrimSpace(repoRoot) != "" && strings.TrimSpace(repoRoot) != "." {
		absoluteRepoRoot, err := filepath.Abs(repoRoot)
		if err != nil {
			return config.Paths{}, fmt.Errorf("resolve working directory: %w", err)
		}
		resolvedRepoRoot = absoluteRepoRoot
	} else {
		discovered, err := config.DiscoverRepoRoot(start)
		if err != nil {
			return config.Paths{}, err
		}
		resolvedRepoRoot = discovered
	}

	paths, err := config.ResolvePaths(resolvedRepoRoot)
	if err != nil {
		return config.Paths{}, err
	}
	return paths, nil
}

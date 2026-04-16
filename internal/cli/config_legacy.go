package cli

import "github.com/keonho-kim/orch/internal/config"

func runConfigMigrate(repoRoot string) error {
	paths, err := resolveAppPaths(repoRoot)
	if err != nil {
		return err
	}
	if err := config.EnsureRuntimePaths(paths); err != nil {
		return err
	}
	return config.MigrateLegacyJSON(paths)
}

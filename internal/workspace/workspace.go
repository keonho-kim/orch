package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

type ProvisionedWorkspace struct {
	Root string
	Env  []string
}

func Provision(root string, bootstrapAssets string, toolsSource string, baseEnv []string, allowedSecretEnv []string) (ProvisionedWorkspace, error) {
	bootstrapDir := filepath.Join(root, "bootstrap")
	toolsDir := filepath.Join(root, "tools")
	for _, path := range []string{root, bootstrapDir, toolsDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return ProvisionedWorkspace{}, fmt.Errorf("create workspace path %s: %w", path, err)
		}
	}

	if err := syncBootstrapFiles(root, bootstrapDir, toolsDir, bootstrapAssets, toolsSource); err != nil {
		return ProvisionedWorkspace{}, err
	}

	return ProvisionedWorkspace{
		Root: root,
		Env:  sanitizeEnv(baseEnv, allowedSecretEnv),
	}, nil
}

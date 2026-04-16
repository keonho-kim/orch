package assets

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/keonho-kim/orch/internal/buildinfo"
	runtimeasset "github.com/keonho-kim/orch/runtime-asset"
	toolsasset "github.com/keonho-kim/orch/tools"
)

type Materialized struct {
	Root         string
	BootstrapDir string
	ToolsDir     string
}

func VersionedRoot(orchHome string) string {
	return filepath.Join(strings.TrimSpace(orchHome), "runtime", "assets", buildinfo.Version())
}

func EnsureMaterialized(orchHome string) (Materialized, error) {
	root := VersionedRoot(orchHome)
	bootstrapDir := filepath.Join(root, "bootstrap")
	toolsDir := filepath.Join(root, "tools", "ot")
	for _, path := range []string{root, bootstrapDir, toolsDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return Materialized{}, fmt.Errorf("create assets directory %s: %w", path, err)
		}
	}

	if err := writeEmbeddedTree(runtimeasset.BootstrapFS, "bootstrap", bootstrapDir); err != nil {
		return Materialized{}, err
	}
	if err := writeEmbeddedTree(toolsasset.OTFS, "ot", toolsDir); err != nil {
		return Materialized{}, err
	}

	return Materialized{
		Root:         root,
		BootstrapDir: bootstrapDir,
		ToolsDir:     toolsDir,
	}, nil
}

func writeEmbeddedTree(source fs.FS, sourceRoot string, targetRoot string) error {
	return fs.WalkDir(source, sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk embedded assets %s: %w", path, walkErr)
		}
		relPath, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return fmt.Errorf("relative path for %s: %w", path, err)
		}
		if relPath == "." {
			return nil
		}

		targetPath := filepath.Join(targetRoot, relPath)
		if entry.IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create asset directory %s: %w", targetPath, err)
			}
			return nil
		}

		data, err := fs.ReadFile(source, path)
		if err != nil {
			return fmt.Errorf("read embedded asset %s: %w", path, err)
		}
		mode := fs.FileMode(0o644)
		if strings.HasSuffix(path, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(targetPath, data, mode); err != nil {
			return fmt.Errorf("write asset file %s: %w", targetPath, err)
		}
		return nil
	})
}

package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func syncBootstrapFiles(
	workspaceRoot string,
	bootstrapDir string,
	toolsDir string,
	bootstrapAssets string,
	toolsSource string,
) error {
	files := []struct {
		source string
		target string
	}{
		{source: filepath.Join(bootstrapAssets, "AGENTS.md"), target: filepath.Join(workspaceRoot, "AGENTS.md")},
		{source: filepath.Join(bootstrapAssets, "SKILLS.md"), target: filepath.Join(bootstrapDir, "SKILLS.md")},
		{source: filepath.Join(bootstrapAssets, "TOOLS.md"), target: filepath.Join(bootstrapDir, "TOOLS.md")},
	}

	for _, item := range files {
		if err := copyFile(item.source, item.target); err != nil {
			return err
		}
	}

	if err := copyFileIfMissing(filepath.Join(bootstrapAssets, "USER.md"), filepath.Join(bootstrapDir, "USER.md")); err != nil {
		return err
	}

	if err := syncDirectory(filepath.Join(bootstrapAssets, "skills"), filepath.Join(bootstrapDir, "skills")); err != nil {
		return err
	}
	if err := syncDirectory(filepath.Join(bootstrapAssets, "system-prompt"), filepath.Join(bootstrapDir, "system-prompt")); err != nil {
		return err
	}

	return syncDirectory(toolsSource, toolsDir)
}

func copyFile(source string, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open source file %s: %w", source, err)
	}
	defer func() {
		_ = in.Close()
	}()

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create parent directory for %s: %w", target, err)
	}

	out, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("create target file %s: %w", target, err)
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s to %s: %w", source, target, err)
	}

	return nil
}

func copyFileIfMissing(source string, target string) error {
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat target file %s: %w", target, err)
	}

	return copyFile(source, target)
}

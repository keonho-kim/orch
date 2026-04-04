package workspace

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func syncDirectory(source string, target string) error {
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("remove directory %s: %w", target, err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", target, err)
	}

	info, err := os.Stat(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat source directory %s: %w", source, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("source %s is not a directory", source)
	}

	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %s: %w", path, walkErr)
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("compute relative path for %s: %w", path, err)
		}
		if relPath == "." {
			return nil
		}

		targetPath := filepath.Join(target, relPath)
		if entry.IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create directory %s: %w", targetPath, err)
			}
			return nil
		}

		return copyFile(path, targetPath)
	})
}

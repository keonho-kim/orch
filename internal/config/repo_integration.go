package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func LooksLikeRepoRoot(path string) bool {
	bootstrapPath := filepath.Join(path, runtimeAssetDirName, bootstrapDirName, "AGENTS.md")
	if !fileExists(bootstrapPath) {
		return false
	}
	markers := []string{
		filepath.Join(path, "go.mod"),
		filepath.Join(path, ".git"),
		filepath.Join(path, settingsFileName),
		filepath.Join(path, "orch.settings.json"),
	}
	for _, marker := range markers {
		if fileExists(marker) || dirExists(marker) {
			return true
		}
	}
	return true
}

func ensureConfigIgnored(paths Paths) error {
	relative, ok := pathRelativeToRepo(paths.RepoRoot, paths.ConfigFile)
	if !ok {
		return nil
	}

	gitDir := filepath.Join(paths.RepoRoot, ".git")
	if !dirExists(gitDir) {
		return nil
	}
	excludePath := filepath.Join(gitDir, "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return fmt.Errorf("create git exclude directory: %w", err)
	}

	data, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read git exclude: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if slices.Contains(lines, relative) {
		return nil
	}
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		data = append(data, '\n')
	}
	data = append(data, []byte(relative+"\n")...)
	if err := os.WriteFile(excludePath, data, 0o644); err != nil {
		return fmt.Errorf("write git exclude: %w", err)
	}
	return nil
}

func pathRelativeToRepo(repoRoot string, target string) (string, bool) {
	relative, err := filepath.Rel(repoRoot, target)
	if err != nil {
		return "", false
	}
	if relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

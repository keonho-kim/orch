package workspace

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type ProvisionedWorkspace struct {
	Root string
	Env  []string
}

func Provision(root string, bootstrapAssets string, baseEnv []string, allowedSecretEnv []string) (ProvisionedWorkspace, error) {
	bootstrapDir := filepath.Join(root, "bootstrap")
	toolsDir := filepath.Join(root, "tools")
	for _, path := range []string{root, bootstrapDir, toolsDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return ProvisionedWorkspace{}, fmt.Errorf("create workspace path %s: %w", path, err)
		}
	}

	repoRoot := filepath.Dir(filepath.Dir(bootstrapAssets))
	if err := syncBootstrapFiles(root, bootstrapDir, toolsDir, bootstrapAssets, filepath.Join(repoRoot, "tools")); err != nil {
		return ProvisionedWorkspace{}, err
	}

	return ProvisionedWorkspace{
		Root: root,
		Env:  sanitizeEnv(baseEnv, allowedSecretEnv),
	}, nil
}

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

func sanitizeEnv(baseEnv []string, allowedSecretEnv []string) []string {
	allowedPrefixes := []string{
		"GIT_",
		"GO",
		"HTTP_",
		"HTTPS_",
		"NO_PROXY",
		"OLLAMA_",
		"VLLM_",
	}
	allowedKeys := map[string]struct{}{
		"HOME":                {},
		"LANG":                {},
		"LC_ALL":              {},
		"LC_CTYPE":            {},
		"ORCH_SUBAGENT_DEPTH": {},
		"PATH":                {},
		"PWD":                 {},
		"SHELL":               {},
		"TERM":                {},
		"TMPDIR":              {},
		"USER":                {},
		"USERNAME":            {},
	}
	for _, key := range allowedSecretEnv {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		allowedKeys[key] = struct{}{}
	}

	filtered := make([]string, 0, len(baseEnv))
	for _, entry := range baseEnv {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			continue
		}

		if _, ok := allowedKeys[key]; ok {
			filtered = append(filtered, key+"="+value)
			continue
		}

		for _, prefix := range allowedPrefixes {
			if strings.HasPrefix(key, prefix) {
				filtered = append(filtered, key+"="+value)
				break
			}
		}
	}

	return filtered
}

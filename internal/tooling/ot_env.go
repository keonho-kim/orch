package tooling

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func resolveOTScriptPath(workspaceRoot string, env []string, subcommand string) string {
	if root := strings.TrimSpace(envValueByKey(env, "ORCH_OT_TOOLS_DIR")); root != "" {
		return filepath.Join(root, "ot", subcommand+".sh")
	}
	return filepath.Join(workspaceRoot, "tools", "ot", subcommand+".sh")
}

func baseEnv(workspaceRoot string, env []string) []string {
	base := append([]string(nil), env...)
	repoRoot := workspaceRoot
	if cwd, err := os.Getwd(); err == nil {
		repoRoot = cwd
	}
	base = append(base, "OT_WORKSPACE_ROOT="+workspaceRoot)
	base = append(base, "OT_REPO_ROOT="+repoRoot)
	base = append(base, "PATH="+prefixedPath(env))
	return base
}

func prefixedPath(env []string) string {
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok && key == "PATH" {
			return value
		}
	}

	path, err := exec.LookPath("bash")
	if err != nil {
		return ""
	}
	return filepath.Dir(path)
}

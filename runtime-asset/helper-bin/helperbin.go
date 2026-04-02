package helperbin

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	appDirName = "orch"
)

//go:embed linux/amd64/rg linux/amd64/patch linux/arm64/rg linux/arm64/patch
var embeddedHelpers embed.FS

type Prepared struct {
	Dir      string
	RGBin    string
	PatchBin string
}

func PrepareOTEnv(env []string, version string) ([]string, error) {
	next := append([]string(nil), env...)
	if runtime.GOOS != "linux" {
		return next, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user config dir: %w", err)
	}

	prepared, err := prepareForPlatform(version, runtime.GOOS, runtime.GOARCH, configDir)
	if err != nil {
		return nil, err
	}

	next = upsertEnv(next, "OT_RG_BIN", prepared.RGBin)
	next = upsertEnv(next, "OT_PATCH_BIN", prepared.PatchBin)
	next = upsertEnv(next, "ORCH_HELPER_BIN_DIR", prepared.Dir)
	next = upsertEnv(next, "PATH", prependPath(prepared.Dir, envValue(next, "PATH")))
	return next, nil
}

func prepareForPlatform(version string, goos string, goarch string, configDir string) (Prepared, error) {
	if goos != "linux" {
		return Prepared{}, nil
	}

	platform, err := platformDir(goos, goarch)
	if err != nil {
		return Prepared{}, err
	}

	root := helperRoot(configDir, version, platform)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Prepared{}, fmt.Errorf("create helper directory %s: %w", root, err)
	}

	rgPath, err := ensureHelper(root, goos, goarch, "rg")
	if err != nil {
		return Prepared{}, err
	}
	patchPath, err := ensureHelper(root, goos, goarch, "patch")
	if err != nil {
		return Prepared{}, err
	}

	return Prepared{
		Dir:      root,
		RGBin:    rgPath,
		PatchBin: patchPath,
	}, nil
}

func platformDir(goos string, goarch string) (string, error) {
	if goos != "linux" {
		return "", fmt.Errorf("unsupported helper platform %s/%s", goos, goarch)
	}

	switch goarch {
	case "amd64", "arm64":
		return goos + "-" + goarch, nil
	default:
		return "", fmt.Errorf("unsupported helper platform %s/%s", goos, goarch)
	}
}

func helperRoot(configDir string, version string, platform string) string {
	cleanVersion := strings.TrimSpace(version)
	if cleanVersion == "" {
		cleanVersion = "dev"
	}
	return filepath.Join(configDir, appDirName, "runtime", "bin", cleanVersion, platform)
}

func ensureHelper(root string, goos string, goarch string, name string) (string, error) {
	assetPath := filepath.ToSlash(filepath.Join(goos, goarch, name))
	data, err := embeddedHelpers.ReadFile(assetPath)
	if err != nil {
		target := filepath.Join(root, name)
		return "", fmt.Errorf("prepare %s helper at %s: %w", name, target, err)
	}

	target := filepath.Join(root, name)
	if err := ensureExecutableFile(target, data); err != nil {
		return "", fmt.Errorf("prepare %s helper at %s: %w", name, target, err)
	}
	return target, nil
}

func ensureExecutableFile(path string, data []byte) error {
	info, err := os.Stat(path)
	if err == nil {
		if info.Size() == int64(len(data)) && info.Mode().Perm()&0o111 == 0o111 {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat existing helper: %w", err)
	}

	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp helper: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temp helper: %w", err)
	}
	if err := temp.Chmod(0o755); err != nil {
		temp.Close()
		return fmt.Errorf("chmod temp helper: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temp helper: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("install helper: %w", err)
	}
	return nil
}

func envValue(env []string, key string) string {
	for _, entry := range env {
		currentKey, value, ok := strings.Cut(entry, "=")
		if ok && currentKey == key {
			return value
		}
	}
	return ""
}

func upsertEnv(env []string, key string, value string) []string {
	next := make([]string, 0, len(env)+1)
	replaced := false
	for _, entry := range env {
		currentKey, _, ok := strings.Cut(entry, "=")
		if ok && currentKey == key {
			if !replaced {
				next = append(next, key+"="+value)
				replaced = true
			}
			continue
		}
		next = append(next, entry)
	}
	if !replaced {
		next = append(next, key+"="+value)
	}
	return next
}

func prependPath(dir string, current string) string {
	if strings.TrimSpace(current) == "" {
		return dir
	}
	parts := strings.Split(current, string(os.PathListSeparator))
	for _, part := range parts {
		if part == dir {
			return current
		}
	}
	return dir + string(os.PathListSeparator) + current
}

package helperbin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlatformDirSupportsLinuxHelpers(t *testing.T) {
	t.Parallel()

	got, err := platformDir("linux", "amd64")
	if err != nil {
		t.Fatalf("platformDir: %v", err)
	}
	if got != "linux-amd64" {
		t.Fatalf("unexpected platform dir: %q", got)
	}
}

func TestPlatformDirRejectsUnsupportedTarget(t *testing.T) {
	t.Parallel()

	if _, err := platformDir("darwin", "arm64"); err == nil {
		t.Fatal("expected unsupported platform to fail")
	}
}

func TestPrepareForPlatformExtractsHelpers(t *testing.T) {
	t.Parallel()

	orchHome := t.TempDir()
	prepared, err := prepareForPlatform("test-version", "linux", "amd64", orchHome)
	if err != nil {
		t.Fatalf("prepareForPlatform: %v", err)
	}

	if prepared.Dir != filepath.Join(orchHome, "runtime", "bin", "test-version", "linux-amd64") {
		t.Fatalf("unexpected helper dir: %q", prepared.Dir)
	}
	assertExecutable(t, prepared.RGBin)
	assertExecutable(t, prepared.PatchBin)
}

func TestPrepareForPlatformUsesDevVersionFallback(t *testing.T) {
	t.Parallel()

	orchHome := t.TempDir()
	prepared, err := prepareForPlatform("", "linux", "arm64", orchHome)
	if err != nil {
		t.Fatalf("prepareForPlatform: %v", err)
	}

	if prepared.Dir != filepath.Join(orchHome, "runtime", "bin", "dev", "linux-arm64") {
		t.Fatalf("unexpected helper dir: %q", prepared.Dir)
	}
}

func TestPrepareForPlatformSkipsNonLinux(t *testing.T) {
	t.Parallel()

	prepared, err := prepareForPlatform("test-version", "darwin", "arm64", t.TempDir())
	if err != nil {
		t.Fatalf("prepareForPlatform: %v", err)
	}
	if prepared != (Prepared{}) {
		t.Fatalf("expected empty preparation result, got %+v", prepared)
	}
}

func TestPrepareOTEnvPrependsHelperDirectory(t *testing.T) {
	t.Parallel()

	env := []string{"PATH=/usr/bin"}
	next := upsertEnv(env, "PATH", prependPath("/tmp/helpers", envValue(env, "PATH")))
	if got := envValue(next, "PATH"); got != "/tmp/helpers:/usr/bin" {
		t.Fatalf("unexpected PATH: %q", got)
	}
}

func assertExecutable(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("expected file, got directory: %s", path)
	}
	if info.Mode().Perm()&0o111 != 0o111 {
		t.Fatalf("expected executable mode, got %o", info.Mode().Perm())
	}
	if info.Size() == 0 {
		t.Fatalf("expected helper %s to be non-empty", path)
	}
}

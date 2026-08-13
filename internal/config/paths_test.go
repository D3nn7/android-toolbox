package config

import (
	"os"
	"path/filepath"
	"testing"
)

// setUserConfigDir points os.UserConfigDir() at a temp directory regardless
// of the host OS, so Resolve's tests don't depend on (or pollute) the real
// per-user config location.
func setUserConfigDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("AppData", dir)         // windows
	t.Setenv("XDG_CONFIG_HOME", dir) // linux
	t.Setenv("HOME", dir)            // darwin fallback ($HOME/Library/Application Support)
}

func TestResolveCreatesDirectoriesAndFillsPaths(t *testing.T) {
	base := t.TempDir()
	setUserConfigDir(t, base)

	p, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve returned an error: %v", err)
	}

	if p.ConfigDir == "" {
		t.Fatal("expected a non-empty ConfigDir")
	}
	for _, dir := range []string{p.ConfigDir, p.BackupDir, p.ToolsDir, p.LogsDir, filepath.Dir(p.AIPromptFile)} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("expected %s to exist: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", dir)
		}
	}

	wantFiles := map[string]string{
		"actions.yaml":  p.ActionsFile,
		"settings.yaml": p.SettingsFile,
		"state.json":    p.StateFile,
	}
	for base, path := range wantFiles {
		if filepath.Base(path) != base {
			t.Errorf("expected %s to end in %q, got %q", path, base, path)
		}
		if filepath.Dir(path) != p.ConfigDir {
			t.Errorf("expected %s to live under ConfigDir, got %q", base, path)
		}
	}
}

func TestResolveIsIdempotent(t *testing.T) {
	base := t.TempDir()
	setUserConfigDir(t, base)

	first, err := Resolve()
	if err != nil {
		t.Fatalf("first Resolve returned an error: %v", err)
	}
	second, err := Resolve()
	if err != nil {
		t.Fatalf("second Resolve returned an error: %v", err)
	}
	if first != second {
		t.Fatalf("expected Resolve to be deterministic, got %+v then %+v", first, second)
	}
}

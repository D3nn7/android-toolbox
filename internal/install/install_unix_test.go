//go:build !windows

package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallCreatesSymlinksOnPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	installDir := filepath.Join(home, ".local", "bin")
	t.Setenv("PATH", installDir)

	exeDir := t.TempDir()
	exePath := filepath.Join(exeDir, "android-toolbox")
	if err := os.WriteFile(exePath, []byte("fake binary"), 0o755); err != nil {
		t.Fatalf("WriteFile returned an error: %v", err)
	}

	res, err := Install(exePath, "android-toolbox", "atbx")
	if err != nil {
		t.Fatalf("Install returned an error: %v", err)
	}

	if res.InstallDir != installDir {
		t.Errorf("InstallDir = %q, want %q", res.InstallDir, installDir)
	}
	if !res.OnPath {
		t.Errorf("expected OnPath to be true when installDir is already on PATH, got false (note: %q)", res.Note)
	}
	if res.Note != "" {
		t.Errorf("expected no follow-up note when already on PATH, got %q", res.Note)
	}

	for _, name := range []string{"android-toolbox", "atbx"} {
		link := filepath.Join(installDir, name)
		target, err := os.Readlink(link)
		if err != nil {
			t.Fatalf("expected %s to be a symlink: %v", link, err)
		}
		if target != exePath {
			t.Errorf("symlink %s points to %q, want %q", link, target, exePath)
		}
	}
}

func TestInstallReportsNotOnPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir()) // deliberately does not include installDir

	exeDir := t.TempDir()
	exePath := filepath.Join(exeDir, "android-toolbox")
	if err := os.WriteFile(exePath, []byte("fake binary"), 0o755); err != nil {
		t.Fatalf("WriteFile returned an error: %v", err)
	}

	res, err := Install(exePath, "android-toolbox", "atbx")
	if err != nil {
		t.Fatalf("Install returned an error: %v", err)
	}
	if res.OnPath {
		t.Error("expected OnPath to be false when installDir is not on PATH")
	}
	if res.Note == "" {
		t.Error("expected a follow-up note explaining how to add installDir to PATH")
	}
}

func TestInstallAppendsToShellRC(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir()) // deliberately does not include installDir
	t.Setenv("SHELL", "/bin/zsh")

	exeDir := t.TempDir()
	exePath := filepath.Join(exeDir, "android-toolbox")
	if err := os.WriteFile(exePath, []byte("fake binary"), 0o755); err != nil {
		t.Fatalf("WriteFile returned an error: %v", err)
	}

	installDir := filepath.Join(home, ".local", "bin")

	res, err := Install(exePath, "android-toolbox", "atbx")
	if err != nil {
		t.Fatalf("Install returned an error: %v", err)
	}
	if res.OnPath {
		t.Error("expected OnPath to be false - the change only lands in the rc file, not the current process' env")
	}

	rcFile := filepath.Join(home, ".zshrc")
	data, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatalf("expected %s to have been created: %v", rcFile, err)
	}
	if !strings.Contains(string(data), installDir) {
		t.Errorf("expected %s to reference %s, got:\n%s", rcFile, installDir, data)
	}
	if !strings.Contains(res.Note, rcFile) {
		t.Errorf("expected Note to mention %s, got %q", rcFile, res.Note)
	}
}

func TestInstallDoesNotDuplicateShellRCEntry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("SHELL", "/bin/zsh")

	exeDir := t.TempDir()
	exePath := filepath.Join(exeDir, "android-toolbox")
	if err := os.WriteFile(exePath, []byte("fake binary"), 0o755); err != nil {
		t.Fatalf("WriteFile returned an error: %v", err)
	}

	if _, err := Install(exePath, "android-toolbox", "atbx"); err != nil {
		t.Fatalf("first Install returned an error: %v", err)
	}
	if _, err := Install(exePath, "android-toolbox", "atbx"); err != nil {
		t.Fatalf("second Install returned an error: %v", err)
	}

	rcFile := filepath.Join(home, ".zshrc")
	data, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", rcFile, err)
	}
	installDir := filepath.Join(home, ".local", "bin")
	if count := strings.Count(string(data), installDir); count != 1 {
		t.Errorf("expected exactly one PATH entry for %s after two installs, found %d in:\n%s", installDir, count, data)
	}
}

func TestShellRCFilePicksVariantByShell(t *testing.T) {
	home := string(filepath.Separator) + "home-fixture"
	installDir := filepath.Join(home, ".local", "bin")

	wantBashFile := filepath.Join(home, ".bashrc")
	if runtime.GOOS == "darwin" {
		wantBashFile = filepath.Join(home, ".bash_profile")
	}

	cases := []struct {
		shell    string
		wantFile string
	}{
		{"/bin/zsh", filepath.Join(home, ".zshrc")},
		{"/bin/bash", wantBashFile},
		{"/usr/bin/fish", filepath.Join(home, ".config", "fish", "config.fish")},
	}
	for _, c := range cases {
		t.Setenv("SHELL", c.shell)
		gotFile, gotLine := shellRCFile(home, installDir)
		if gotFile != c.wantFile {
			t.Errorf("shellRCFile(SHELL=%q) file = %q, want %q", c.shell, gotFile, c.wantFile)
		}
		if !strings.Contains(gotLine, installDir) {
			t.Errorf("shellRCFile(SHELL=%q) export line %q does not reference %q", c.shell, gotLine, installDir)
		}
	}
}

func TestInstallOverwritesExistingSymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")

	exeDir := t.TempDir()
	firstExe := filepath.Join(exeDir, "v1")
	secondExe := filepath.Join(exeDir, "v2")
	for _, p := range []string{firstExe, secondExe} {
		if err := os.WriteFile(p, []byte("fake binary"), 0o755); err != nil {
			t.Fatalf("WriteFile returned an error: %v", err)
		}
	}

	if _, err := Install(firstExe, "android-toolbox", "atbx"); err != nil {
		t.Fatalf("first Install returned an error: %v", err)
	}
	if _, err := Install(secondExe, "android-toolbox", "atbx"); err != nil {
		t.Fatalf("second Install returned an error: %v", err)
	}

	link := filepath.Join(home, ".local", "bin", "android-toolbox")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("expected %s to be a symlink: %v", link, err)
	}
	if target != secondExe {
		t.Errorf("expected re-installing to repoint the symlink to %q, got %q", secondExe, target)
	}
}

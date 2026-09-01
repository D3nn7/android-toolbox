package toolsmanager

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEnsureEmulatorPlatformToolsCopiesAdbDir(t *testing.T) {
	toolsDir := t.TempDir()
	adbDir := t.TempDir()

	adbName := exeName("adb", runtime.GOOS)
	if err := os.WriteFile(filepath.Join(adbDir, adbName), []byte("fake adb"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adbDir, "AdbWinApi.dll"), []byte("fake dll"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(toolsDir)
	if err := m.EnsureEmulatorPlatformTools(filepath.Join(adbDir, adbName)); err != nil {
		t.Fatal(err)
	}

	destAdb := filepath.Join(m.SdkRoot(), "platform-tools", adbName)
	if _, err := os.Stat(destAdb); err != nil {
		t.Fatalf("expected adb to be copied to %s: %v", destAdb, err)
	}
	if _, err := os.Stat(filepath.Join(m.SdkRoot(), "platform-tools", "AdbWinApi.dll")); err != nil {
		t.Fatalf("expected sibling files to be copied too: %v", err)
	}
}

func TestEnsureEmulatorPlatformToolsIsIdempotent(t *testing.T) {
	toolsDir := t.TempDir()
	adbDir := t.TempDir()
	adbName := exeName("adb", runtime.GOOS)
	adbPath := filepath.Join(adbDir, adbName)
	if err := os.WriteFile(adbPath, []byte("fake adb"), 0o755); err != nil {
		t.Fatal(err)
	}

	m := New(toolsDir)
	if err := m.EnsureEmulatorPlatformTools(adbPath); err != nil {
		t.Fatal(err)
	}
	// Removing the source shouldn't matter the second time around - the
	// destination already has adb, so this should be a no-op rather than
	// failing trying to re-copy from a now-missing source.
	if err := os.Remove(adbPath); err != nil {
		t.Fatal(err)
	}
	if err := m.EnsureEmulatorPlatformTools(adbPath); err != nil {
		t.Fatalf("expected the second call to be a no-op, got: %v", err)
	}
}

func TestEnsureEmulatorPlatformToolsRequiresAdbPath(t *testing.T) {
	m := New(t.TempDir())
	if err := m.EnsureEmulatorPlatformTools(""); err == nil {
		t.Fatal("expected an error when adbPath is empty")
	}
}

//go:build windows

package install

import (
	"os"
	"path/filepath"
	"testing"
)

// Install and addToUserPath are deliberately not covered by an automated
// test here: addToUserPath writes to the real HKCU\Environment registry key
// of whichever account runs `go test`, which would permanently mutate that
// user's actual PATH - an unacceptable side effect for a unit test. copyFile
// is the one piece of Install with no such side effect, so it's what gets
// tested directly.
func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.exe")
	dst := filepath.Join(dir, "dst.exe")

	want := []byte("fake binary contents")
	if err := os.WriteFile(src, want, 0o644); err != nil {
		t.Fatalf("WriteFile returned an error: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile returned an error: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("expected the destination file to exist: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("copyFile contents = %q, want %q", got, want)
	}
}

func TestCopyFileMissingSource(t *testing.T) {
	dir := t.TempDir()
	if err := copyFile(filepath.Join(dir, "does-not-exist"), filepath.Join(dir, "dst")); err == nil {
		t.Fatal("expected an error when the source file does not exist")
	}
}

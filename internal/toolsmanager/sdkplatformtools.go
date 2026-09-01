package toolsmanager

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// EnsureEmulatorPlatformTools makes sure SdkRoot()/platform-tools exists and
// contains adb. The emulator binary validates whatever SDK root it's given
// (explicitly via ANDROID_SDK_ROOT/ANDROID_HOME, or guessed from its own
// path otherwise) by checking for a platform-tools subdirectory, and
// refuses to start at all if that check fails ("Cannot find AVD system
// path. Please define ANDROID_SDK_ROOT") - this app manages adb separately
// from SdkRoot() for the rest of the app (see adbDir), so without this,
// SdkRoot() never satisfies that check on its own. adbPath is the
// already-resolved adb binary (see ResolveADB) whose directory gets mirrored
// into place; a no-op once done, since it only checks for adb's presence
// under the destination first.
func (m *Manager) EnsureEmulatorPlatformTools(adbPath string) error {
	dest := filepath.Join(m.SdkRoot(), "platform-tools")
	if isExecutableFile(filepath.Join(dest, exeName("adb", runtime.GOOS))) {
		return nil
	}
	if adbPath == "" {
		return fmt.Errorf("adb is not available - cannot set up the emulator's SDK root")
	}
	return copyDirTree(filepath.Dir(adbPath), dest)
}

// copyDirTree recursively copies every file under src into dest, creating
// directories as needed.
func copyDirTree(src, dest string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		destPath := filepath.Join(dest, e.Name())
		if e.IsDir() {
			if err := copyDirTree(srcPath, destPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(srcPath, destPath); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

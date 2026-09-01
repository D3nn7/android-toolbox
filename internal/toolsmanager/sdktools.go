package toolsmanager

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// SdkRoot returns the base directory this app maintains its own bootstrapped
// Android SDK components (cmdline-tools, emulator, system images, ...)
// under - kept separate from the adb/scrcpy directories managed elsewhere in
// this package so the two never collide.
func (m *Manager) SdkRoot() string {
	return filepath.Join(m.ToolsDir, "sdk")
}

func (m *Manager) cmdlineToolsDir() string {
	return filepath.Join(m.SdkRoot(), "cmdline-tools", "latest")
}

// scriptName returns the platform-appropriate name for one of cmdline-tools'
// shell-script entry points (sdkmanager/avdmanager): a ".bat" file on
// Windows, an extensionless shell script everywhere else - unlike adb/
// scrcpy/emulator, which are real native binaries (see exeName).
func scriptName(base, goos string) string {
	if goos == "windows" {
		return base + ".bat"
	}
	return base
}

// FetchCmdlineTools downloads and installs the pinned cmdline-tools build
// (see cmdlineToolsRevision) for goos/arch into SdkRoot()/cmdline-tools/latest -
// the layout sdkmanager/avdmanager expect so they can locate their own SDK
// root by walking up two directories from their own script path.
func (m *Manager) FetchCmdlineTools(ctx context.Context, goos, arch string, progress ProgressFunc) error {
	if progress == nil {
		progress = noopProgress
	}
	url, filename, err := cmdlineToolsURL(goos, arch)
	if err != nil {
		return err
	}

	progress(fmt.Sprintf("Downloading Android cmdline-tools (%s)...", filename))
	tmpFile, err := downloadToFileTracked(ctx, url, m.ToolsDir, func(read, total int64) {
		if total > 0 {
			progress(fmt.Sprintf("%d%% - %s", read*100/total, filename))
		}
	})
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	destDir := m.cmdlineToolsDir()
	if err := os.RemoveAll(destDir); err != nil {
		return err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	// The zip's own top-level directory is "cmdline-tools" - stripped so its
	// contents (bin/, lib/, ...) land directly in .../cmdline-tools/latest,
	// not .../cmdline-tools/latest/cmdline-tools.
	if err := extractZip(tmpFile, destDir, "cmdline-tools"); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}
	if goos != "windows" {
		_ = os.Chmod(filepath.Join(destDir, "bin", "sdkmanager"), 0o755)
		_ = os.Chmod(filepath.Join(destDir, "bin", "avdmanager"), 0o755)
	}

	_ = writeVersionMarker(destDir, cmdlineToolsRevision)

	progress("cmdline-tools installed to " + destDir)
	return nil
}

// InstalledCmdlineToolsRevision returns the locally recorded cmdline-tools
// revision marker, or "" if never fetched by this app.
func (m *Manager) InstalledCmdlineToolsRevision() string {
	return readVersionMarker(m.cmdlineToolsDir())
}

// externalSdkRoot returns ANDROID_HOME or ANDROID_SDK_ROOT if either is set
// and points at an existing directory - the "prefer an already-installed
// Android SDK" half of this package's resolution order, checked before the
// bundled copy this app might otherwise fetch for itself.
func externalSdkRoot() string {
	for _, env := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if v := os.Getenv(env); v != "" {
			if info, err := os.Stat(v); err == nil && info.IsDir() {
				return v
			}
		}
	}
	return ""
}

// findUnderCmdlineTools looks for name (a script produced by scriptName)
// under <sdkRoot>/cmdline-tools/<version>/bin/, preferring the "latest"
// version directory - external SDK installs don't always set that up as a
// symlink the way this app's own bootstrap does, so any version directory
// is accepted as a fallback.
func findUnderCmdlineTools(sdkRoot, name string) string {
	preferred := filepath.Join(sdkRoot, "cmdline-tools", "latest", "bin", name)
	if isExecutableFile(preferred) {
		return preferred
	}
	entries, err := os.ReadDir(filepath.Join(sdkRoot, "cmdline-tools"))
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(sdkRoot, "cmdline-tools", e.Name(), "bin", name)
		if isExecutableFile(p) {
			return p
		}
	}
	return ""
}

func resolveSdkTool(bundledSdkRoot, scriptBase string) (ResolvedTool, error) {
	name := scriptName(scriptBase, runtime.GOOS)
	if root := externalSdkRoot(); root != "" {
		if p := findUnderCmdlineTools(root, name); p != "" {
			return ResolvedTool{Path: p, Source: "external-sdk"}, nil
		}
	}
	if p := findUnderCmdlineTools(bundledSdkRoot, name); p != "" {
		return ResolvedTool{Path: p, Source: "bundled"}, nil
	}
	if p, err := exec.LookPath(scriptBase); err == nil {
		return ResolvedTool{Path: p, Source: "system"}, nil
	}
	return ResolvedTool{}, fmt.Errorf("%s not found (no existing Android SDK, none bundled, none on PATH) - run 'android-toolbox emulator setup'", scriptBase)
}

// ResolveSdkManager finds sdkmanager, preferring an existing external
// Android SDK (ANDROID_HOME/ANDROID_SDK_ROOT), then this app's own bundled
// cmdline-tools, then PATH.
func (m *Manager) ResolveSdkManager() (ResolvedTool, error) {
	return resolveSdkTool(m.SdkRoot(), "sdkmanager")
}

// ResolveAvdManager is ResolveSdkManager for avdmanager.
func (m *Manager) ResolveAvdManager() (ResolvedTool, error) {
	return resolveSdkTool(m.SdkRoot(), "avdmanager")
}

// ResolveEmulator finds the emulator binary, same preference order as
// ResolveSdkManager: an existing external SDK's emulator/ directory first,
// then this app's own bundled copy (installed via sdkmanager, see
// InstallSdkPackage), then PATH.
func (m *Manager) ResolveEmulator() (ResolvedTool, error) {
	name := exeName("emulator", runtime.GOOS)
	if root := externalSdkRoot(); root != "" {
		p := filepath.Join(root, "emulator", name)
		if isExecutableFile(p) {
			return ResolvedTool{Path: p, Source: "external-sdk"}, nil
		}
	}
	p := filepath.Join(m.SdkRoot(), "emulator", name)
	if isExecutableFile(p) {
		return ResolvedTool{Path: p, Source: "bundled"}, nil
	}
	if p, err := exec.LookPath("emulator"); err == nil {
		return ResolvedTool{Path: p, Source: "system"}, nil
	}
	return ResolvedTool{}, fmt.Errorf("emulator not found (no existing Android SDK, none bundled, none on PATH) - run 'android-toolbox emulator setup'")
}

// ResolveJava finds a Java runtime via JAVA_HOME, then PATH. Unlike every
// other tool this package resolves, Java is never bundled - a JRE/JDK is a
// large, platform-specific install that Android developers normally already
// have, so this app only ever checks for one rather than fetching it.
func ResolveJava() (ResolvedTool, error) {
	name := exeName("java", runtime.GOOS)
	if home := os.Getenv("JAVA_HOME"); home != "" {
		p := filepath.Join(home, "bin", name)
		if isExecutableFile(p) {
			return ResolvedTool{Path: p, Source: "system"}, nil
		}
	}
	if p, err := exec.LookPath("java"); err == nil {
		return ResolvedTool{Path: p, Source: "system"}, nil
	}
	return ResolvedTool{}, fmt.Errorf("java not found (JAVA_HOME unset, none on PATH) - install a JRE/JDK (11+) to use the emulator manager")
}

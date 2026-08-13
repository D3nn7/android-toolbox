// Package toolsmanager resolves, downloads and verifies the portable adb and
// scrcpy binaries the app ships its own copies of, independent of whatever
// (if anything) is installed system-wide.
package toolsmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Manager resolves and fetches portable tool binaries under a base
// tools directory (typically <configDir>/tools).
type Manager struct {
	ToolsDir string
}

// New creates a Manager rooted at toolsDir.
func New(toolsDir string) *Manager {
	return &Manager{ToolsDir: toolsDir}
}

// ResolvedTool describes where a tool executable was found.
type ResolvedTool struct {
	Path   string
	Source string // "bundled" or "system"
}

func exeName(base, goos string) string {
	if goos == "windows" {
		return base + ".exe"
	}
	return base
}

func (m *Manager) adbDir(goos string) string {
	return filepath.Join(m.ToolsDir, "adb", goos)
}

func (m *Manager) scrcpyDir(goos, arch string) string {
	return filepath.Join(m.ToolsDir, "scrcpy", goos+"-"+arch)
}

// BundledADBPath returns where the portable adb binary would live for the
// current OS, whether or not it has been fetched yet.
func (m *Manager) BundledADBPath() string {
	return filepath.Join(m.adbDir(runtime.GOOS), exeName("adb", runtime.GOOS))
}

// BundledScrcpyPath returns where the portable scrcpy binary would live for
// the current OS/arch, whether or not it has been fetched yet.
func (m *Manager) BundledScrcpyPath() string {
	return filepath.Join(m.scrcpyDir(runtime.GOOS, runtime.GOARCH), exeName("scrcpy", runtime.GOOS))
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// ResolveADB finds adb, preferring the bundled copy and falling back to the
// system PATH.
func (m *Manager) ResolveADB() (ResolvedTool, error) {
	if p := m.BundledADBPath(); isExecutableFile(p) {
		return ResolvedTool{Path: p, Source: "bundled"}, nil
	}
	if p, err := exec.LookPath("adb"); err == nil {
		return ResolvedTool{Path: p, Source: "system"}, nil
	}
	return ResolvedTool{}, fmt.Errorf("adb not found (neither bundled nor on PATH) - run 'android-toolbox tools fetch'")
}

// ResolveScrcpy finds scrcpy, preferring the bundled copy and falling back to
// the system PATH.
func (m *Manager) ResolveScrcpy() (ResolvedTool, error) {
	if p := m.BundledScrcpyPath(); isExecutableFile(p) {
		return ResolvedTool{Path: p, Source: "bundled"}, nil
	}
	if p, err := exec.LookPath("scrcpy"); err == nil {
		return ResolvedTool{Path: p, Source: "system"}, nil
	}
	return ResolvedTool{}, fmt.Errorf("scrcpy not found (neither bundled nor on PATH) - run 'android-toolbox tools fetch'")
}

// ProgressFunc receives human-readable progress messages during a fetch.
type ProgressFunc func(message string)

func noopProgress(string) {}

// FetchADB downloads and installs the portable adb (Google platform-tools)
// for goos into the tool cache.
func (m *Manager) FetchADB(ctx context.Context, goos string, progress ProgressFunc) error {
	if progress == nil {
		progress = noopProgress
	}
	url, err := platformToolsURL(goos)
	if err != nil {
		return err
	}

	progress(fmt.Sprintf("Downloading platform-tools (adb) for %s...", goos))
	tmpFile, err := downloadToFile(ctx, url, m.ToolsDir)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	destDir := m.adbDir(goos)
	if err := os.RemoveAll(destDir); err != nil {
		return err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	if err := extractZip(tmpFile, destDir, "platform-tools"); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}
	if goos != "windows" {
		_ = os.Chmod(filepath.Join(destDir, "adb"), 0o755)
	}

	// Best-effort: record what was just installed so a later CheckADBUpdate
	// has a baseline to compare against. A failure here (e.g. a transient
	// network hiccup on this second, tiny HEAD request) must not undo an
	// otherwise-successful install.
	if version, err := adbRemoteVersion(ctx, goos); err == nil {
		_ = writeVersionMarker(destDir, version)
	}

	progress("adb installed to " + destDir)
	return nil
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

// resolveScrcpyVersion asks GitHub for the latest scrcpy release tag,
// falling back to a hardcoded known-good version if that call fails for any
// reason (offline, rate-limited, ...).
func (m *Manager) resolveScrcpyVersion(ctx context.Context) string {
	text, err := fetchText(ctx, scrcpyLatestReleaseAPI)
	if err != nil {
		return fallbackScrcpyVersion
	}
	var rel githubRelease
	if err := json.Unmarshal([]byte(text), &rel); err != nil || rel.TagName == "" {
		return fallbackScrcpyVersion
	}
	return rel.TagName
}

// FetchScrcpy downloads and installs the portable scrcpy build for
// goos/arch, verifying its SHA256 checksum against the release's published
// SHA256SUMS.txt when that file is reachable.
func (m *Manager) FetchScrcpy(ctx context.Context, goos, arch string, progress ProgressFunc) error {
	if progress == nil {
		progress = noopProgress
	}

	version := m.resolveScrcpyVersion(ctx)
	url, filename, err := scrcpyAssetURL(version, goos, arch)
	if err != nil {
		return err
	}

	progress(fmt.Sprintf("Downloading scrcpy %s (%s)...", version, filename))
	tmpFile, err := downloadToFile(ctx, url, m.ToolsDir)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	if sums, err := fetchText(ctx, scrcpySumsURL(version)); err == nil {
		if err := verifySHA256(tmpFile, sums, filename); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		progress("Checksum verified.")
	} else {
		progress("Warning: SHA256SUMS.txt not reachable, skipping verification.")
	}

	key, isZip, err := scrcpyAssetName(goos, arch)
	if err != nil {
		return err
	}
	destDir := m.scrcpyDir(goos, arch)
	if err := os.RemoveAll(destDir); err != nil {
		return err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	stripPrefix := fmt.Sprintf("scrcpy-%s-%s", key, version)
	if isZip {
		err = extractZip(tmpFile, destDir, stripPrefix)
	} else {
		err = extractTarGz(tmpFile, destDir, stripPrefix)
	}
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}
	if goos != "windows" {
		_ = os.Chmod(filepath.Join(destDir, "scrcpy"), 0o755)
	}

	// Best-effort, same reasoning as FetchADB: don't fail an otherwise
	// successful install over the marker write.
	_ = writeVersionMarker(destDir, version)

	progress("scrcpy installed to " + destDir)
	return nil
}

// FetchAll fetches both adb and scrcpy for goos/arch.
func (m *Manager) FetchAll(ctx context.Context, goos, arch string, progress ProgressFunc) error {
	if err := m.FetchADB(ctx, goos, progress); err != nil {
		return fmt.Errorf("adb: %w", err)
	}
	if err := m.FetchScrcpy(ctx, goos, arch, progress); err != nil {
		return fmt.Errorf("scrcpy: %w", err)
	}
	return nil
}

package toolsmanager

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// versionMarkerPath is where a tool's identifying "version" (see
// writeVersionMarker) is recorded, right alongside the binary itself so it
// travels with it and survives a tool being fetched into a fresh directory.
func versionMarkerPath(dir string) string {
	return filepath.Join(dir, ".version")
}

// readVersionMarker returns the recorded version for a tool directory, or ""
// if it was never fetched by a version of this app that wrote one (e.g. an
// existing bundled copy from before this feature existed).
func readVersionMarker(dir string) string {
	data, err := os.ReadFile(versionMarkerPath(dir))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func writeVersionMarker(dir, version string) error {
	return os.WriteFile(versionMarkerPath(dir), []byte(version), 0o644)
}

// adbRemoteVersion identifies the currently published platform-tools build
// for goos without downloading the (multi-megabyte) archive itself: Google's
// "latest" zip is served with a stable ETag that changes only when the
// underlying build does, so a cheap HEAD request against the same URL
// FetchADB downloads doubles as a version fingerprint.
func adbRemoteVersion(ctx context.Context, goos string) (string, error) {
	url, err := platformToolsURL(goos)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("adb version check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("adb version check failed: HTTP %d", resp.StatusCode)
	}
	if etag := resp.Header.Get("ETag"); etag != "" {
		return etag, nil
	}
	if lastMod := resp.Header.Get("Last-Modified"); lastMod != "" {
		return lastMod, nil
	}
	return "", fmt.Errorf("server returned neither ETag nor Last-Modified for adb - version check not possible")
}

// ToolUpdateStatus reports one tool's currently-installed and latest-known
// version, and whether they differ. Installed is empty when the tool was
// never fetched by a version of this app that recorded a marker (a fresh
// install, or one predating this feature) - Available is still meaningful
// in that case (there's always "an update" over an unknown baseline).
type ToolUpdateStatus struct {
	Installed string
	Latest    string
	Available bool
}

// CheckADBUpdate compares the installed adb build (if any) against what
// Google currently serves as "latest", without downloading anything beyond
// a single HEAD request.
func (m *Manager) CheckADBUpdate(ctx context.Context, goos string) (ToolUpdateStatus, error) {
	installed := readVersionMarker(m.adbDir(goos))
	latest, err := adbRemoteVersion(ctx, goos)
	if err != nil {
		return ToolUpdateStatus{}, err
	}
	return ToolUpdateStatus{Installed: installed, Latest: latest, Available: installed != latest}, nil
}

// CheckScrcpyUpdate compares the installed scrcpy version for goos/arch (if
// any) against the latest GitHub release tag.
func (m *Manager) CheckScrcpyUpdate(ctx context.Context, goos, arch string) ToolUpdateStatus {
	installed := readVersionMarker(m.scrcpyDir(goos, arch))
	latest := m.resolveScrcpyVersion(ctx)
	return ToolUpdateStatus{Installed: installed, Latest: latest, Available: installed != latest}
}

// InstalledADBVersion returns the locally recorded adb version marker for
// goos (see writeVersionMarker), or "" if none is known - a purely local,
// no-network lookup, unlike CheckADBUpdate. Meant for rendering "what do we
// have right now" against a separately (and less frequently) fetched
// "latest known" value, e.g. a cached background-check result.
func (m *Manager) InstalledADBVersion(goos string) string {
	return readVersionMarker(m.adbDir(goos))
}

// InstalledScrcpyVersion is InstalledADBVersion for scrcpy.
func (m *Manager) InstalledScrcpyVersion(goos, arch string) string {
	return readVersionMarker(m.scrcpyDir(goos, arch))
}

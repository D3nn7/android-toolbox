package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"android-toolbox/internal/buildinfo"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

// Release is the subset of a GitHub release this package needs.
type Release struct {
	TagName string
	Version string // TagName with any leading "v" stripped
	HTMLURL string
	Assets  []Asset
}

// Asset is one file attached to a release.
type Asset struct {
	Name string
	URL  string
}

// repoSlug derives "owner/repo" from buildinfo.RepoURL, so the GitHub API
// target stays in sync with the one canonical repo URL shown throughout the
// app rather than being duplicated as a second constant.
func repoSlug() string {
	return strings.TrimPrefix(strings.TrimSuffix(buildinfo.RepoURL, "/"), "https://github.com/")
}

// LatestRelease asks GitHub for this project's most recent release.
// Callers driving an automatic/background check should bound ctx with a
// short timeout - this must never be allowed to stall app startup.
func LatestRelease(ctx context.Context) (Release, error) {
	url := "https://api.github.com/repos/" + repoSlug() + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("GitHub release query failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("GitHub release query failed: HTTP %d", resp.StatusCode)
	}

	var payload struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Release{}, fmt.Errorf("could not read GitHub response: %w", err)
	}

	rel := Release{
		TagName: payload.TagName,
		Version: strings.TrimPrefix(payload.TagName, "v"),
		HTMLURL: payload.HTMLURL,
	}
	for _, a := range payload.Assets {
		rel.Assets = append(rel.Assets, Asset{Name: a.Name, URL: a.BrowserDownloadURL})
	}
	return rel, nil
}

// IsNewer reports whether latestVersion is a newer release than
// currentVersion (both "MAJOR.MINOR.PATCH", optionally "v"-prefixed).
func IsNewer(currentVersion, latestVersion string) bool {
	return CompareVersions(currentVersion, latestVersion) < 0
}

// AssetFor finds the release asset matching this project's own naming
// convention (see .github/workflows/release.yml): e.g.
// "android-toolbox_v1.2.3_windows_amd64.zip".
func AssetFor(rel Release, goos, goarch string) (Asset, error) {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	suffix := fmt.Sprintf("_%s_%s.%s", goos, goarch, ext)
	for _, a := range rel.Assets {
		if strings.HasPrefix(a.Name, "android-toolbox_") && strings.HasSuffix(a.Name, suffix) {
			return a, nil
		}
	}
	return Asset{}, fmt.Errorf("no matching release package found for %s/%s", goos, goarch)
}

// binaryNameFor is the executable's own filename inside a release archive.
func binaryNameFor(goos string) string {
	if goos == "windows" {
		return "android-toolbox.exe"
	}
	return "android-toolbox"
}

// Apply downloads asset, extracts the android-toolbox binary from it, and
// replaces exePath with it. goos identifies which binary name to look for
// inside the archive (see binaryNameFor) - callers pass runtime.GOOS; it's
// a parameter rather than read directly so tests can exercise both binary
// name conventions regardless of the host running them.
//
// Cross-platform binary self-replacement can't overwrite a running
// executable in place, so this renames the current one aside (to
// exePath+".old") rather than deleting it - Windows in particular refuses
// to delete a file that's still executing, but allows renaming it. The
// ".old" file is left for CleanupOldBinary to remove on the next start,
// once nothing still has it open.
func Apply(ctx context.Context, asset Asset, exePath, goos string, progress func(string)) error {
	if progress == nil {
		progress = func(string) {}
	}

	dir := filepath.Dir(exePath)
	progress(fmt.Sprintf("Downloading %s...", asset.Name))
	archivePath, err := downloadToTemp(ctx, asset.URL, dir)
	if err != nil {
		return err
	}
	defer os.Remove(archivePath)

	newBinaryPath := filepath.Join(dir, ".android-toolbox.new")
	defer os.Remove(newBinaryPath)
	if err := extractSingleFile(archivePath, binaryNameFor(goos), newBinaryPath); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}
	if goos != "windows" {
		if err := os.Chmod(newBinaryPath, 0o755); err != nil {
			return err
		}
	}

	oldPath := exePath + ".old"
	_ = os.Remove(oldPath) // clean up a leftover from a previous update, if any
	if err := os.Rename(exePath, oldPath); err != nil {
		return fmt.Errorf("could not rename current executable: %w", err)
	}
	if err := os.Rename(newBinaryPath, exePath); err != nil {
		_ = os.Rename(oldPath, exePath) // best-effort: leave the app in a working state
		return fmt.Errorf("could not put new executable in place: %w", err)
	}

	progress("Update installed.")
	return nil
}

// CleanupOldBinary best-effort removes a ".old" file left behind by a
// previous Apply call. Meant to be called once, early, on every startup -
// by then the old binary is no longer running and can finally be deleted
// (on Windows specifically, a still-running executable can be renamed but
// not deleted, which is why Apply couldn't remove it itself).
func CleanupOldBinary(exePath string) {
	_ = os.Remove(exePath + ".old")
}

func downloadToTemp(ctx context.Context, url, dir string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download %s failed: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s failed: HTTP %d", url, resp.StatusCode)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, "android-toolbox-update-*.tmp")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("download %s failed: %w", url, err)
	}
	return tmp.Name(), nil
}

// extractSingleFile pulls exactly the entry named wantName out of a .zip or
// .tar.gz archive and writes it to destPath - release archives for this
// project contain nothing else (see .github/workflows/release.yml), so
// there's no directory tree or path-traversal concern to handle here, unlike
// internal/toolsmanager's general-purpose extractors.
func extractSingleFile(archivePath, wantName, destPath string) error {
	if isZip(archivePath) {
		return extractSingleFromZip(archivePath, wantName, destPath)
	}
	return extractSingleFromTarGz(archivePath, wantName, destPath)
}

// isZip sniffs the file's magic number rather than trusting archivePath's
// extension, since Apply downloads it to a generic ".tmp" temp file name.
func isZip(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	sig := make([]byte, 4)
	if _, err := io.ReadFull(f, sig); err != nil {
		return false
	}
	return sig[0] == 'P' && sig[1] == 'K'
}

func extractSingleFromZip(archivePath, wantName, destPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if filepath.Base(f.Name) != wantName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		return writeFile(destPath, rc)
	}
	return fmt.Errorf("%s not found in archive", wantName)
}

func extractSingleFromTarGz(archivePath, wantName, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != wantName {
			continue
		}
		return writeFile(destPath, tr)
	}
	return fmt.Errorf("%s not found in archive", wantName)
}

func writeFile(destPath string, r io.Reader) error {
	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, r)
	return err
}

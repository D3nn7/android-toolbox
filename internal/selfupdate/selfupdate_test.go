package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRepoSlugDerivesFromRepoURL(t *testing.T) {
	if got := repoSlug(); got != "d3nn7/android-toolbox" {
		t.Fatalf("expected repoSlug to derive owner/repo from buildinfo.RepoURL, got %q", got)
	}
}

func TestAssetForFindsMatchingPlatformArchive(t *testing.T) {
	rel := Release{Assets: []Asset{
		{Name: "android-toolbox_v1.2.3_windows_amd64.zip", URL: "https://example.invalid/win.zip"},
		{Name: "android-toolbox_v1.2.3_linux_amd64.tar.gz", URL: "https://example.invalid/linux.tar.gz"},
		{Name: "android-toolbox_v1.2.3_darwin_arm64.tar.gz", URL: "https://example.invalid/mac-arm.tar.gz"},
	}}

	a, err := AssetFor(rel, "windows", "amd64")
	if err != nil || a.Name != "android-toolbox_v1.2.3_windows_amd64.zip" {
		t.Fatalf("expected the windows/amd64 asset, got %+v, err %v", a, err)
	}

	a, err = AssetFor(rel, "darwin", "arm64")
	if err != nil || a.Name != "android-toolbox_v1.2.3_darwin_arm64.tar.gz" {
		t.Fatalf("expected the darwin/arm64 asset, got %+v, err %v", a, err)
	}
}

func TestAssetForErrorsWhenNoPlatformMatches(t *testing.T) {
	rel := Release{Assets: []Asset{{Name: "android-toolbox_v1.2.3_windows_amd64.zip"}}}
	if _, err := AssetFor(rel, "linux", "arm64"); err == nil {
		t.Fatal("expected an error when no asset matches the requested platform")
	}
}

func buildZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func buildTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestApplyReplacesTheBinaryAndKeepsTheOldOneAside is the core end-to-end
// check for the self-update mechanics: Asset.URL is entirely
// caller-supplied, so a real httptest.Server can stand in for GitHub's
// asset download without touching the network - this exercises the actual
// download/extract/rename-swap sequence, not just its individual pieces.
func TestApplyReplacesTheBinaryAndKeepsTheOldOneAside(t *testing.T) {
	archive := buildZip(t, map[string]string{"android-toolbox.exe": "new binary content"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer server.Close()

	dir := t.TempDir()
	exePath := filepath.Join(dir, "android-toolbox.exe")
	if err := os.WriteFile(exePath, []byte("old binary content"), 0o755); err != nil {
		t.Fatal(err)
	}

	asset := Asset{Name: "android-toolbox_v1.0.0_windows_amd64.zip", URL: server.URL}
	var progressMsgs []string
	err := Apply(context.Background(), asset, exePath, "windows", func(msg string) { progressMsgs = append(progressMsgs, msg) })
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new binary content" {
		t.Fatalf("expected the binary to be replaced, got %q", got)
	}

	oldContent, err := os.ReadFile(exePath + ".old")
	if err != nil {
		t.Fatalf("expected the original binary to be kept aside as .old: %v", err)
	}
	if string(oldContent) != "old binary content" {
		t.Fatalf("expected .old to hold the original content, got %q", oldContent)
	}

	if len(progressMsgs) == 0 {
		t.Fatal("expected at least one progress message")
	}

	CleanupOldBinary(exePath)
	if _, err := os.Stat(exePath + ".old"); !os.IsNotExist(err) {
		t.Fatal("expected CleanupOldBinary to remove the .old file")
	}
}

// TestApplyExtractsFromTarGzOnUnixPlatforms proves the non-Windows archive
// format (.tar.gz, no file extension on the binary) is handled too.
func TestApplyExtractsFromTarGzOnUnixPlatforms(t *testing.T) {
	archive := buildTarGz(t, map[string]string{"android-toolbox": "unix binary content"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer server.Close()

	dir := t.TempDir()
	exePath := filepath.Join(dir, "android-toolbox")
	if err := os.WriteFile(exePath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	asset := Asset{Name: "android-toolbox_v1.0.0_linux_amd64.tar.gz", URL: server.URL}
	if err := Apply(context.Background(), asset, exePath, "linux", nil); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "unix binary content" {
		t.Fatalf("expected the binary to be replaced, got %q", got)
	}
}

// TestApplyFailsCleanlyWhenBinaryMissingFromArchive proves a malformed or
// unexpected release asset produces a clear error rather than silently
// leaving the current binary renamed aside with nothing to replace it.
func TestApplyFailsCleanlyWhenBinaryMissingFromArchive(t *testing.T) {
	archive := buildZip(t, map[string]string{"readme.txt": "oops, wrong file"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer server.Close()

	dir := t.TempDir()
	exePath := filepath.Join(dir, "android-toolbox.exe")
	if err := os.WriteFile(exePath, []byte("old binary content"), 0o755); err != nil {
		t.Fatal(err)
	}

	asset := Asset{Name: "android-toolbox_v1.0.0_windows_amd64.zip", URL: server.URL}
	if err := Apply(context.Background(), asset, exePath, "windows", nil); err == nil {
		t.Fatal("expected an error when the archive doesn't contain the expected binary")
	}

	// The original binary must be untouched - extraction failed before any
	// renaming happened.
	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old binary content" {
		t.Fatal("expected the original binary to be left in place after a failed extraction")
	}
	if _, err := os.Stat(exePath + ".old"); !os.IsNotExist(err) {
		t.Fatal("expected no .old file to be created when extraction never got that far")
	}
}

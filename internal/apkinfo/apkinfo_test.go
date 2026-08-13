package apkinfo

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// buildTestAPK writes a real zip file at dir/name containing
// AndroidManifest.xml (binary AXML, from tree) plus a couple of filler
// entries, so Analyze can be exercised end to end (zip open, entry lookup,
// AXML parse, hashing, signing-block lookup) without needing a real APK.
func buildTestAPK(t *testing.T, dir, name string, tree testElem) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	manifestData := newAXMLBuilder().build(tree)

	w, err := zw.Create("AndroidManifest.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(manifestData); err != nil {
		t.Fatal(err)
	}

	w, err = zw.Create("classes.dex")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("pretend this is dex bytecode")); err != nil {
		t.Fatal(err)
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAnalyzeEndToEnd(t *testing.T) {
	path := buildTestAPK(t, t.TempDir(), "sample.apk", sampleManifestTree())

	info, err := Analyze(path)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if info.Path != path {
		t.Errorf("Path = %q, want %q", info.Path, path)
	}
	if info.SizeBytes <= 0 {
		t.Errorf("SizeBytes = %d, want > 0", info.SizeBytes)
	}
	if len(info.SHA256) != 64 {
		t.Errorf("SHA256 = %q, want a 64-char hex string", info.SHA256)
	}
	if info.EntryCount != 2 {
		t.Errorf("EntryCount = %d, want 2", info.EntryCount)
	}
	if info.Manifest.PackageName != "com.example.app" {
		t.Errorf("Manifest.PackageName = %q, want com.example.app", info.Manifest.PackageName)
	}
	if info.Manifest.VersionCode != 42 {
		t.Errorf("Manifest.VersionCode = %d, want 42", info.Manifest.VersionCode)
	}
	// No signing block was spliced into this fixture, and there's no
	// META-INF/*.RSA either - a completely unsigned zip should report no
	// scheme at all rather than guessing.
	if info.Signing.SchemeV2 || info.Signing.SchemeV3 || info.Signing.SchemeV1Only {
		t.Errorf("expected no signing scheme detected, got %+v", info.Signing)
	}
}

func TestAnalyzeFileNotFound(t *testing.T) {
	if _, err := Analyze(filepath.Join(t.TempDir(), "does-not-exist.apk")); err == nil {
		t.Fatal("expected an error for a nonexistent path")
	}
}

func TestAnalyzeNotAZipFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-zip.apk")
	if err := os.WriteFile(path, []byte("just some plain bytes, not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Analyze(path); err == nil {
		t.Fatal("expected an error for a file that isn't a zip archive")
	}
}

func TestAnalyzeMissingManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-manifest.apk")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	if _, err := zw.Create("classes.dex"); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := Analyze(path); err == nil {
		t.Fatal("expected an error when AndroidManifest.xml is missing (not a valid apk)")
	}
}

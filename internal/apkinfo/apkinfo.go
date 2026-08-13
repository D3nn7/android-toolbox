package apkinfo

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// Info is the full report Analyze produces for one APK file.
type Info struct {
	Path       string `json:"path"`
	SizeBytes  int64  `json:"sizeBytes"`
	SHA256     string `json:"sha256"`
	EntryCount int    `json:"entryCount"`

	Manifest ManifestInfo `json:"manifest"`
	Signing  SigningInfo  `json:"signing"`
}

// Analyze opens path as a zip archive, extracts and parses
// AndroidManifest.xml, and gathers file/signing metadata. It never shells
// out to any external tool - a pure Go implementation, so it behaves
// identically on Windows, Linux and macOS.
func Analyze(path string) (Info, error) {
	info := Info{Path: path}

	stat, err := os.Stat(path)
	if err != nil {
		return Info{}, fmt.Errorf("apk not found: %w", err)
	}
	info.SizeBytes = stat.Size()

	sum, err := fileSHA256(path)
	if err != nil {
		return Info{}, fmt.Errorf("hashing failed: %w", err)
	}
	info.SHA256 = sum

	zr, err := zip.OpenReader(path)
	if err != nil {
		return Info{}, fmt.Errorf("not a valid apk (zip) file: %w", err)
	}
	defer zr.Close()
	info.EntryCount = len(zr.File)

	manifestData, err := readZipEntry(&zr.Reader, "AndroidManifest.xml")
	if err != nil {
		return Info{}, fmt.Errorf("AndroidManifest.xml: %w", err)
	}
	root, err := ParseManifestXML(manifestData)
	if err != nil {
		return Info{}, fmt.Errorf("parsing AndroidManifest.xml: %w", err)
	}
	info.Manifest = ExtractManifestInfo(root)

	info.Signing = AnalyzeSigning(&zr.Reader, path)

	return info, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func readZipEntry(zr *zip.Reader, name string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("entry %q not found", name)
}

package toolsmanager

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 5 * time.Minute}

// downloadToFile streams url into a new temp file inside dir and returns its
// path. The caller is responsible for removing it once done.
func downloadToFile(ctx context.Context, url, dir string) (string, error) {
	return downloadToFileTracked(ctx, url, dir, nil)
}

// progressWriter counts bytes written through it and reports (read, total)
// to onProgress after every chunk - total is 0 if the server never sent a
// Content-Length, in which case callers should treat the download as
// indeterminate rather than computing a bogus percentage.
type progressWriter struct {
	io.Writer
	read, total int64
	onProgress  func(read, total int64)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	w.read += int64(n)
	if w.onProgress != nil {
		w.onProgress(w.read, w.total)
	}
	return n, err
}

// downloadToFileTracked is downloadToFile with an optional byte-progress
// callback, invoked as data streams in - used to drive a real (non-
// decorative) percentage progress bar for large downloads. onProgress may be
// nil, in which case this behaves exactly like downloadToFile.
func downloadToFileTracked(ctx context.Context, url, dir string, onProgress func(read, total int64)) (string, error) {
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
	tmp, err := os.CreateTemp(dir, "download-*.tmp")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	dst := io.Writer(tmp)
	if onProgress != nil {
		dst = &progressWriter{Writer: tmp, total: resp.ContentLength, onProgress: onProgress}
	}
	if _, err := io.Copy(dst, resp.Body); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("download %s failed: %w", url, err)
	}
	return tmp.Name(), nil
}

// fetchText downloads a small text resource (e.g. a checksum file) and
// returns its contents. A failure here is treated as non-fatal by callers.
func fetchText(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// verifySHA256 checks that the file at path hashes to the expected value
// found in a SHA256SUMS.txt-style listing (lines: "<hex>  <filename>").
func verifySHA256(path, sumsText, filename string) error {
	var expected string
	for _, line := range strings.Split(sumsText, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimSpace(fields[1]) == filename {
			expected = strings.ToLower(fields[0])
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("no checksum for %s found in SHA256SUMS.txt", filename)
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

// extractZip extracts a zip archive into destDir. Any path component equal
// to stripPrefix at the root of each entry is dropped, which lets us unpack
// e.g. "platform-tools/adb.exe" directly into destDir/adb.exe.
func extractZip(archivePath, destDir, stripPrefix string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		rel := stripArchivePrefix(f.Name, stripPrefix)
		if rel == "" {
			continue
		}
		target := filepath.Join(destDir, rel)
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe path in archive: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := extractZipEntry(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extractZipEntry(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	mode := f.Mode()
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

// extractTarGz extracts a .tar.gz archive into destDir with the same
// stripPrefix semantics as extractZip.
func extractTarGz(archivePath, destDir, stripPrefix string) error {
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

		rel := stripArchivePrefix(hdr.Name, stripPrefix)
		if rel == "" {
			continue
		}
		target := filepath.Join(destDir, rel)
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe path in archive: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(hdr.Mode)
			if mode == 0 {
				mode = 0o644
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}

// stripArchivePrefix normalises an archive entry path to forward slashes and
// removes a single leading directory component matching stripPrefix. It
// returns "" if the entry should be skipped (e.g. it *is* the prefix dir).
func stripArchivePrefix(name, stripPrefix string) string {
	name = filepath.ToSlash(name)
	if stripPrefix != "" {
		p := stripPrefix + "/"
		if name == stripPrefix {
			return ""
		}
		if strings.HasPrefix(name, p) {
			name = strings.TrimPrefix(name, p)
		}
	}
	if name == "" || strings.HasPrefix(name, "../") {
		return ""
	}
	return filepath.FromSlash(name)
}

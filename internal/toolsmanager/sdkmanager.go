package toolsmanager

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

// InstallSdkPackage runs "sdkmanager --install <pkg>", used for every SDK
// component beyond cmdline-tools itself (system images, platforms, the
// emulator package): unlike cmdline-tools (see FetchCmdlineTools), none of
// these have a documented stable direct-download URL - sdkmanager resolves
// them live against Google's servers, so shelling out to it (rather than
// reimplementing that resolution) is both simpler and always current.
//
// sdkmanager prompts for license acceptance on stdin the first time a
// licensed package is installed; feeding it a generous stream of "y\n"
// answers up front accepts whatever it asks without needing to parse the
// prompt text, the same non-interactive approach "yes | sdkmanager
// --licenses" scripts commonly use.
func InstallSdkPackage(ctx context.Context, sdkManagerPath, sdkRoot, pkg string, progress ProgressFunc) error {
	if progress == nil {
		progress = noopProgress
	}

	cmd := ScriptCommand(ctx, sdkManagerPath, "--install", pkg, "--sdk_root="+sdkRoot)
	cmd.Stdin = strings.NewReader(strings.Repeat("y\n", 20))

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("sdkmanager --install %s: %w", pkg, err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		scanner.Split(scanLinesOrCR)
		for scanner.Scan() {
			if line := strings.TrimSpace(scanner.Text()); line != "" {
				progress(line)
			}
		}
	}()

	waitErr := cmd.Wait()
	pw.Close()
	<-done
	if waitErr != nil {
		return fmt.Errorf("sdkmanager --install %s failed: %w", pkg, waitErr)
	}
	return nil
}

// scanLinesOrCR is bufio.ScanLines extended to also split on a lone '\r' -
// sdkmanager draws its progress bar by overwriting the current line with
// '\r' rather than starting a new one with '\n' when it detects an
// interactive terminal; treating either as a line boundary means each
// update still reaches the progress callback as its own message instead of
// all being concatenated into one unscannable line.
func scanLinesOrCR(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// ListSdkPackages runs "sdkmanager --list" and returns the "path" column
// (e.g. "system-images;android-34;google_apis;x86_64") of every installed
// and every available package, respectively. Callers filter by prefix
// (e.g. strings.HasPrefix(p, "system-images;")).
func ListSdkPackages(ctx context.Context, sdkManagerPath, sdkRoot string) (installed, available []string, err error) {
	cmd := ScriptCommand(ctx, sdkManagerPath, "--list", "--sdk_root="+sdkRoot)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, nil, fmt.Errorf("sdkmanager --list failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	installed, available = parseSdkPackageList(string(out))
	return installed, available, nil
}

// parseSdkPackageList parses sdkmanager --list's textual table output. Its
// format is two sections ("Installed packages:" / "Available Packages:"),
// each a "|"-delimited table whose first column is the package path,
// preceded by a header row and a "----" separator row.
func parseSdkPackageList(out string) (installed, available []string) {
	var current *[]string
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.EqualFold(trimmed, "Installed packages:"):
			current = &installed
			continue
		case strings.EqualFold(trimmed, "Available Packages:"):
			current = &available
			continue
		case trimmed == "":
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(trimmed, "Path") || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "=") {
			continue
		}
		path := strings.TrimSpace(strings.SplitN(trimmed, "|", 2)[0])
		if path == "" {
			continue
		}
		*current = append(*current, path)
	}
	return installed, available
}

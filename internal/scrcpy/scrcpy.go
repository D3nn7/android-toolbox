// Package scrcpy launches the scrcpy binary as an independent, detached
// process so it can run alongside the TUI without interfering with its
// terminal rendering.
package scrcpy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Launcher starts scrcpy with a configurable set of default arguments.
type Launcher struct {
	BinPath     string
	DefaultArgs []string
	LogDir      string
}

// New creates a Launcher for the scrcpy binary at binPath.
func New(binPath string, defaultArgs []string, logDir string) *Launcher {
	return &Launcher{BinPath: binPath, DefaultArgs: defaultArgs, LogDir: logDir}
}

// Launch starts "scrcpy -s <serial> <defaultArgs...> <extraArgs...>" without
// waiting for it to exit. scrcpy's own stdout/stderr are redirected to a log
// file (rather than inherited) so they never corrupt the Bubbletea TUI's
// terminal output. The returned *exec.Cmd is already started and is reaped
// internally (see the goroutine below) - callers only need cmd.Process.Pid
// and can otherwise ignore it; they must not call cmd.Wait() themselves,
// since exec.Cmd forbids waiting on the same process twice.
func (l *Launcher) Launch(serial string, extraArgs []string) (*exec.Cmd, error) {
	if l.BinPath == "" {
		return nil, fmt.Errorf("scrcpy is not available (see 'android-toolbox tools fetch')")
	}

	args := make([]string, 0, len(l.DefaultArgs)+len(extraArgs)+2)
	args = append(args, "-s", serial)
	args = append(args, l.DefaultArgs...)
	args = append(args, extraArgs...)

	cmd := exec.Command(l.BinPath, args...)

	if l.LogDir != "" {
		if err := os.MkdirAll(l.LogDir, 0o755); err != nil {
			return nil, err
		}
		logPath := filepath.Join(l.LogDir, fmt.Sprintf("scrcpy-%s-%d.log", sanitizeSerial(serial), time.Now().UnixNano()))
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return nil, err
		}
		// Start() dup()s this fd for the child, so our copy can (and must,
		// to avoid leaking it and to let the file be removed/rotated later
		// while the child still holds its own handle) be closed right after.
		defer f.Close()
		cmd.Stdout = f
		cmd.Stderr = f
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("scrcpy could not be started: %w", err)
	}

	// scrcpy runs detached from any TUI screen, so nothing else will ever
	// call Wait() on it - without this, every launch would leave a zombie
	// process entry behind (on Unix) for the rest of the app's lifetime.
	go func() {
		_ = cmd.Wait()
	}()

	return cmd, nil
}

func sanitizeSerial(serial string) string {
	out := make([]rune, 0, len(serial))
	for _, r := range serial {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}

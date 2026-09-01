package avd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Launcher starts the emulator binary as an independent, detached process -
// the AVD equivalent of internal/scrcpy's Launcher, following the exact same
// shape (detached, log-redirected, reaped internally) for the same reason:
// it must run alongside the TUI without interfering with its terminal
// rendering, and nothing else will ever call Wait() on it.
type Launcher struct {
	EmulatorPath string
	LogDir       string
	// SdkRoot is exported to the launched process as ANDROID_SDK_ROOT/
	// ANDROID_HOME (see sdkEnvVars) - without it, the emulator binary only
	// guesses its SDK root from its own path, and refuses to start at all
	// ("Cannot find AVD system path") if that guess doesn't check out.
	SdkRoot string
}

// NewLauncher creates a Launcher for the emulator binary at emulatorPath.
func NewLauncher(emulatorPath, logDir, sdkRoot string) *Launcher {
	return &Launcher{EmulatorPath: emulatorPath, LogDir: logDir, SdkRoot: sdkRoot}
}

// LaunchResult describes an emulator process that was just started.
type LaunchResult struct {
	Cmd *exec.Cmd
	// LogPath is where the emulator's own stdout/stderr are being written -
	// "" if LogDir was empty (no redirection happened). Surfaced to the
	// caller so it can point a stuck/crashed-boot error at a concrete file,
	// rather than a "something went wrong, check somewhere" dead end.
	LogPath string
}

// Launch starts "emulator -avd <name> [-no-window] <extraArgs...>" without
// waiting for it to exit. windowed selects the default visible-window mode
// versus headless (-no-window) - see config.Settings.Emulator.Windowed. The
// emulator's own stdout/stderr are redirected to a log file, same reasoning
// as scrcpy.Launcher.Launch: never let a child process's output corrupt the
// Bubbletea TUI's terminal rendering.
//
// onExit, if non-nil, is called from a background goroutine once the
// process exits, successfully or not - the emulator binary commonly exits
// within a second or two of being started if it can't boot at all (a
// missing/corrupt AVD, no hardware acceleration configured, ...), and
// without this hook that failure is invisible: the caller only ever sees
// "started fine" and has no way to tell "still booting" apart from
// "already dead" other than guessing from a timeout.
func (l *Launcher) Launch(name string, windowed bool, extraArgs []string, onExit func(error)) (LaunchResult, error) {
	if l.EmulatorPath == "" {
		return LaunchResult{}, fmt.Errorf("the emulator binary is not available (see 'android-toolbox emulator setup')")
	}

	args := make([]string, 0, len(extraArgs)+4)
	args = append(args, "-avd", name)
	if !windowed {
		args = append(args, "-no-window")
	}
	args = append(args, extraArgs...)

	cmd := exec.Command(l.EmulatorPath, args...)
	if env := sdkEnvVars(l.SdkRoot); env != nil {
		cmd.Env = append(os.Environ(), env...)
	}

	var logPath string
	if l.LogDir != "" {
		if err := os.MkdirAll(l.LogDir, 0o755); err != nil {
			return LaunchResult{}, err
		}
		logPath = filepath.Join(l.LogDir, fmt.Sprintf("emulator-%s-%d.log", sanitizeName(name), time.Now().UnixNano()))
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return LaunchResult{}, err
		}
		defer f.Close()
		cmd.Stdout = f
		cmd.Stderr = f
	}

	if err := cmd.Start(); err != nil {
		return LaunchResult{}, fmt.Errorf("emulator could not be started: %w", err)
	}

	go func() {
		err := cmd.Wait()
		if onExit != nil {
			onExit(err)
		}
	}()

	return LaunchResult{Cmd: cmd, LogPath: logPath}, nil
}

func sanitizeName(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}

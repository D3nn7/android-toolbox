package scrcpy

import (
	"os"
	"testing"
	"time"
)

// TestLaunchStartsProcess is a light smoke test against the real, portable
// scrcpy binary fetched by toolsmanager: it launches "scrcpy -s <serial>
// --version", which exits immediately without opening any window, letting us
// verify argument construction and log-file redirection without disturbing
// a real device session. It is skipped if the binary or ANDROID_TOOLBOX_TEST_SERIAL
// have not been set up, so it never blocks CI environments without a
// fetched toolchain.
//
// It must NOT call cmd.Wait() itself: Launch already reaps the process
// internally, and exec.Cmd forbids calling Wait twice (the second caller
// gets "Wait was already called" and the two calls race over the same
// *os.ProcessState). Completion is instead observed indirectly via the log
// file scrcpy writes its --version output to.
func TestLaunchStartsProcess(t *testing.T) {
	binPath := os.Getenv("ANDROID_TOOLBOX_TEST_SCRCPY")
	serial := os.Getenv("ANDROID_TOOLBOX_TEST_SERIAL")
	if binPath == "" || serial == "" {
		t.Skip("ANDROID_TOOLBOX_TEST_SCRCPY/ANDROID_TOOLBOX_TEST_SERIAL not set, skipping live smoke test")
	}

	logDir := t.TempDir()
	l := New(binPath, nil, logDir)

	cmd, err := l.Launch(serial, []string{"--version"})
	if err != nil {
		t.Fatalf("Launch failed: %v", err)
	}
	if cmd.Process == nil {
		t.Fatal("expected a started process")
	}

	deadline := time.Now().Add(10 * time.Second)
	var entries []os.DirEntry
	for time.Now().Before(deadline) {
		entries, err = os.ReadDir(logDir)
		if err == nil && len(entries) == 1 {
			if info, statErr := entries[0].Info(); statErr == nil && info.Size() > 0 {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for scrcpy log output, got entries=%v (err=%v)", entries, err)
}

func TestSanitizeSerial(t *testing.T) {
	got := sanitizeSerial("R52W-C07:YCWH")
	want := "R52W_C07_YCWH"
	if got != want {
		t.Fatalf("sanitizeSerial() = %q, want %q", got, want)
	}
}

package device

import (
	"context"
	"testing"

	"android-toolbox/internal/adb"
)

// TestCollectDetectsEmulatorSerial exercises Collect against a
// nonexistent adb binary (every field is best-effort, so the sub-queries
// all fail silently - see Collect's own doc comment) to confirm
// IsEmulator/AVDName are derived without needing a real device or adb
// binary: IsEmulator is a pure string check, and AVDName degrades to ""
// exactly like every other best-effort field would.
func TestCollectDetectsEmulatorSerial(t *testing.T) {
	client := adb.New("this-binary-does-not-exist")

	info, err := Collect(context.Background(), client, "emulator-5554")
	if err != nil {
		t.Fatalf("Collect returned an error: %v", err)
	}
	if !info.IsEmulator {
		t.Fatal("expected IsEmulator to be true for an emulator-* serial")
	}
	if info.AVDName != "" {
		t.Fatalf("expected AVDName to stay empty when adb can't be run, got %q", info.AVDName)
	}
}

func TestCollectDoesNotFlagPhysicalDeviceAsEmulator(t *testing.T) {
	client := adb.New("this-binary-does-not-exist")

	info, err := Collect(context.Background(), client, "R52WC07YCWH")
	if err != nil {
		t.Fatalf("Collect returned an error: %v", err)
	}
	if info.IsEmulator {
		t.Fatal("expected a physical-looking serial to not be flagged as an emulator")
	}
}

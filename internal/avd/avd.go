// Package avd manages Android Virtual Devices on top of the resolved
// avdmanager/sdkmanager/emulator binaries (see internal/toolsmanager),
// mirroring the split between internal/adb (mechanical adb calls) and
// internal/device (interpreted device info): this package owns AVD-specific
// domain logic - lifecycle (list/create/delete), specs (config.ini), launch,
// and emulator-console simulation - on top of those tools.
package avd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"android-toolbox/internal/toolsmanager"
)

// Manager runs avdmanager/sdkmanager against one Android SDK root.
type Manager struct {
	AvdManagerPath string
	SdkManagerPath string
	EmulatorPath   string
	SdkRoot        string
}

// New creates a Manager. Any path may be empty if that particular tool
// wasn't resolved - operations needing it then fail with a clear error
// instead of panicking, the same convention internal/scrcpy's Launcher uses.
func New(avdManagerPath, sdkManagerPath, emulatorPath, sdkRoot string) *Manager {
	return &Manager{
		AvdManagerPath: avdManagerPath,
		SdkManagerPath: sdkManagerPath,
		EmulatorPath:   emulatorPath,
		SdkRoot:        sdkRoot,
	}
}

// AvdHome resolves where AVD definitions live: $ANDROID_AVD_HOME if set,
// else the real Android tooling default (~/.android/avd) - deliberately NOT
// an app-managed location, so AVDs created here are also visible to/from
// Android Studio or a manually installed SDK, consistent with this feature
// preferring an existing SDK over a bundled one.
func AvdHome() string {
	if v := os.Getenv("ANDROID_AVD_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".android", "avd")
}

// sdkEnvVars returns the ANDROID_SDK_ROOT/ANDROID_HOME pair pointed at
// sdkRoot - both are set (rather than just one) since different tool
// versions read one or the other, and "" if sdkRoot itself is unknown.
// Shared by avdEnv (avdmanager/sdkmanager) and Launcher.Launch (the
// emulator binary): all three refuse to reliably locate the SDK without it,
// since none of them have an equivalent command-line flag (sdkmanager's own
// --sdk_root= is the one exception, applied separately where used).
func sdkEnvVars(sdkRoot string) []string {
	if sdkRoot == "" {
		return nil
	}
	return []string{"ANDROID_SDK_ROOT=" + sdkRoot, "ANDROID_HOME=" + sdkRoot}
}

// avdEnv returns the environment avdmanager/sdkmanager subprocesses run
// with: ANDROID_SDK_ROOT/ANDROID_HOME pointed at m.SdkRoot, so package
// resolution/AVD creation works whether that root came from this app's own
// bundle or an external SDK - avdmanager has no equivalent command-line
// flag, unlike sdkmanager's --sdk_root=.
func (m *Manager) avdEnv() []string {
	return append(os.Environ(), sdkEnvVars(m.SdkRoot)...)
}

func (m *Manager) avdManagerCmd(ctx context.Context, args ...string) (string, error) {
	cmd := toolsmanager.ScriptCommand(ctx, m.AvdManagerPath, args...)
	cmd.Env = m.avdEnv()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// AVD describes one Android Virtual Device as reported by
// "avdmanager list avd".
type AVD struct {
	Name, Target, ABI, Device, Path string
	// Broken is true for an AVD avdmanager lists under "The following
	// Android Virtual Devices could not be loaded:" instead of the normal
	// section - typically because its system image was later deleted or
	// moved (observed in the wild: an AVD created by Android Studio, then
	// its system image removed via sdkmanager). Target/ABI/Device stay
	// empty for these, since avdmanager never gets far enough to report
	// them; Error holds its stated reason instead. A broken AVD can't be
	// started until its system image is reinstalled.
	Broken bool
	Error  string
}

// List returns every locally defined AVD.
func (m *Manager) List(ctx context.Context) ([]AVD, error) {
	if m.AvdManagerPath == "" {
		return nil, fmt.Errorf("avdmanager is not available (see 'android-toolbox emulator setup')")
	}
	out, err := m.avdManagerCmd(ctx, "list", "avd")
	if err != nil {
		return nil, fmt.Errorf("avdmanager list avd failed: %w: %s", err, strings.TrimSpace(out))
	}
	return parseAVDList(out), nil
}

func parseAVDList(out string) []AVD {
	var avds []AVD
	var cur *AVD
	broken := false
	flush := func() {
		if cur != nil {
			avds = append(avds, *cur)
			cur = nil
		}
	}
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "The following Android Virtual Devices could not be loaded:"):
			// Everything from here on is a broken AVD (see AVD.Broken) -
			// missing/moved system image, corrupt config.ini, etc.
			flush()
			broken = true
			continue
		case strings.HasPrefix(line, "Name:"):
			flush()
			cur = &AVD{Name: strings.TrimSpace(strings.TrimPrefix(line, "Name:")), Broken: broken}
		case cur == nil:
			continue
		case strings.HasPrefix(line, "Device:"):
			cur.Device = strings.TrimSpace(strings.TrimPrefix(line, "Device:"))
		case strings.HasPrefix(line, "Path:"):
			cur.Path = strings.TrimSpace(strings.TrimPrefix(line, "Path:"))
		case strings.HasPrefix(line, "Target:"):
			cur.Target = strings.TrimSpace(strings.TrimPrefix(line, "Target:"))
		case strings.HasPrefix(line, "Error:"):
			cur.Error = strings.TrimSpace(strings.TrimPrefix(line, "Error:"))
		case strings.Contains(line, "Tag/ABI:"):
			idx := strings.Index(line, "Tag/ABI:")
			cur.ABI = strings.TrimSpace(line[idx+len("Tag/ABI:"):])
		}
	}
	flush()
	return avds
}

// deviceIDRe extracts the quotable short id from "avdmanager list device"
// lines like `id: 12 or "pixel_6"` - that short id (rather than the numeric
// one, which shifts as Google adds device definitions) is what's passed to
// "avdmanager create avd -d".
var deviceIDRe = regexp.MustCompile(`or "([^"]+)"`)

// ListDeviceProfiles returns every built-in hardware device profile id
// (e.g. "pixel_6", "Nexus 5") available for "avdmanager create avd -d".
func (m *Manager) ListDeviceProfiles(ctx context.Context) ([]string, error) {
	if m.AvdManagerPath == "" {
		return nil, fmt.Errorf("avdmanager is not available (see 'android-toolbox emulator setup')")
	}
	out, err := m.avdManagerCmd(ctx, "list", "device")
	if err != nil {
		return nil, fmt.Errorf("avdmanager list device failed: %w: %s", err, strings.TrimSpace(out))
	}
	var ids []string
	for _, line := range strings.Split(out, "\n") {
		if m := deviceIDRe.FindStringSubmatch(line); m != nil {
			ids = append(ids, m[1])
		}
	}
	return ids, nil
}

// CreateOptions configures a new AVD.
type CreateOptions struct {
	Name        string
	SystemImage string // e.g. "system-images;android-34;google_apis;x86_64"
	Device      string // a profile id from ListDeviceProfiles, or "" for avdmanager's own default
	SDCardMB    int    // 0 leaves the default (no SD card)
	Force       bool   // overwrite an existing AVD of the same name
}

// Create makes a new AVD. The system image named by opts.SystemImage must
// already be installed (see toolsmanager.InstallSdkPackage) - avdmanager
// itself does not fetch missing packages.
func (m *Manager) Create(ctx context.Context, opts CreateOptions) error {
	if m.AvdManagerPath == "" {
		return fmt.Errorf("avdmanager is not available (see 'android-toolbox emulator setup')")
	}
	if opts.Name == "" || opts.SystemImage == "" {
		return fmt.Errorf("a name and a system image are required")
	}

	args := []string{"create", "avd", "-n", opts.Name, "-k", opts.SystemImage}
	if opts.Device != "" {
		args = append(args, "-d", opts.Device)
	}
	if opts.SDCardMB > 0 {
		args = append(args, "-c", strconv.Itoa(opts.SDCardMB)+"M")
	}
	if opts.Force {
		args = append(args, "--force")
	}

	cmd := toolsmanager.ScriptCommand(ctx, m.AvdManagerPath, args...)
	cmd.Env = m.avdEnv()
	// avdmanager asks "Do you wish to create a custom hardware profile
	// [no]" after creating the AVD from -d's profile - declining keeps the
	// chosen device profile's defaults instead of dropping into an
	// interactive follow-up wizard this app can't drive.
	cmd.Stdin = strings.NewReader("no\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("avdmanager create avd failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Delete removes an AVD by name.
func (m *Manager) Delete(ctx context.Context, name string) error {
	if m.AvdManagerPath == "" {
		return fmt.Errorf("avdmanager is not available (see 'android-toolbox emulator setup')")
	}
	out, err := m.avdManagerCmd(ctx, "delete", "avd", "-n", name)
	if err != nil {
		return fmt.Errorf("avdmanager delete avd failed: %w: %s", err, strings.TrimSpace(out))
	}
	return nil
}

// Package device aggregates per-device information (model, Android version,
// battery, IP, ...) on top of the raw adb client.
package device

import (
	"context"
	"regexp"
	"strings"

	"android-toolbox/internal/adb"
)

// Info is a snapshot of everything the dashboard shows about one device.
type Info struct {
	Serial         string
	Model          string
	Manufacturer   string
	AndroidVersion string
	SDK            string
	Resolution     string
	IPAddress      string
	Battery        adb.Battery

	// IsEmulator/AVDName let the TUI show richer, emulator-specific info
	// (specs, simulation controls) for emulator-* serials without needing
	// its own serial-format check wherever Info is displayed.
	IsEmulator bool
	AVDName    string
}

var ipRegexp = regexp.MustCompile(`inet\s+(\d+\.\d+\.\d+\.\d+)`)

// Collect gathers Info for serial. Individual sub-queries are best-effort:
// a failure on one (e.g. no wlan0 interface) leaves that field empty rather
// than failing the whole collection.
func Collect(ctx context.Context, client *adb.Client, serial string) (Info, error) {
	info := Info{Serial: serial}

	info.Model, _ = client.GetProp(ctx, serial, "ro.product.model")
	info.Manufacturer, _ = client.GetProp(ctx, serial, "ro.product.manufacturer")
	info.AndroidVersion, _ = client.GetProp(ctx, serial, "ro.build.version.release")
	info.SDK, _ = client.GetProp(ctx, serial, "ro.build.version.sdk")

	if out, err := client.Shell(ctx, serial, "wm size"); err == nil {
		info.Resolution = parseWMSize(out)
	}

	info.IPAddress = firstIP(ctx, client, serial, "wlan0", "eth0")

	if battery, err := client.DumpsysBattery(ctx, serial); err == nil {
		info.Battery = battery
	}

	info.IsEmulator = adb.IsEmulatorSerial(serial)
	if info.IsEmulator {
		info.AVDName, _ = client.EmuAVDName(ctx, serial)
	}

	return info, nil
}

func parseWMSize(out string) string {
	idx := strings.LastIndex(out, ":")
	if idx < 0 {
		return strings.TrimSpace(out)
	}
	return strings.TrimSpace(out[idx+1:])
}

func firstIP(ctx context.Context, client *adb.Client, serial string, ifaces ...string) string {
	for _, iface := range ifaces {
		out, err := client.Shell(ctx, serial, "ip -f inet addr show "+iface)
		if err != nil {
			continue
		}
		if m := ipRegexp.FindStringSubmatch(out); m != nil {
			return m[1]
		}
	}
	return ""
}

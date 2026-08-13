package adb

import (
	"context"
	"strconv"
	"strings"
)

// Battery is the parsed result of "dumpsys battery".
type Battery struct {
	Level      int
	Scale      int
	ACPowered  bool
	USBPowered bool
	Status     int
	Health     int
	Technology string
}

// StatusText renders Status as a human-readable string.
func (b Battery) StatusText() string {
	switch b.Status {
	case 1:
		return "unknown"
	case 2:
		return "charging"
	case 3:
		return "discharging"
	case 4:
		return "not charging"
	case 5:
		return "full"
	default:
		return "unknown"
	}
}

// Charging reports whether the device is currently being charged.
func (b Battery) Charging() bool {
	return b.Status == 2 || b.ACPowered || b.USBPowered
}

// DumpsysBattery runs and parses "dumpsys battery" for the given device.
func (c *Client) DumpsysBattery(ctx context.Context, serial string) (Battery, error) {
	out, err := c.Shell(ctx, serial, "dumpsys battery")
	if err != nil {
		return Battery{}, err
	}
	return ParseDumpsysBattery(out), nil
}

// ParseDumpsysBattery parses "key: value" lines from dumpsys battery output.
func ParseDumpsysBattery(out string) Battery {
	var b Battery
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		switch key {
		case "level":
			b.Level, _ = strconv.Atoi(val)
		case "scale":
			b.Scale, _ = strconv.Atoi(val)
		case "status":
			b.Status, _ = strconv.Atoi(val)
		case "health":
			b.Health, _ = strconv.Atoi(val)
		case "technology":
			b.Technology = val
		case "AC powered":
			b.ACPowered = val == "true"
		case "USB powered":
			b.USBPowered = val == "true"
		}
	}
	return b
}

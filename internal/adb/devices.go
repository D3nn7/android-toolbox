package adb

import (
	"context"
	"strings"
)

// Device is one entry from "adb devices -l".
type Device struct {
	Serial      string
	State       string // device, offline, unauthorized, no permissions, ...
	Product     string
	Model       string
	DeviceName  string
	TransportID string
}

// Connected reports whether the device is in a usable state.
func (d Device) Connected() bool {
	return d.State == "device"
}

// ListDevices runs "adb devices -l" and parses the result.
func (c *Client) ListDevices(ctx context.Context) ([]Device, error) {
	out, err := c.Run(ctx, "devices", "-l")
	if err != nil {
		return nil, err
	}
	return ParseDevicesOutput(out), nil
}

// ParseDevicesOutput parses the textual output of "adb devices -l" into
// structured Device values. It is tolerant of unauthorized/offline devices,
// which carry fewer key:value fields than a fully connected one.
func ParseDevicesOutput(out string) []Device {
	var devices []Device
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices attached") || strings.HasPrefix(line, "*") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		d := Device{Serial: fields[0], State: fields[1]}
		for _, kv := range fields[2:] {
			idx := strings.Index(kv, ":")
			if idx < 0 {
				continue
			}
			key, val := kv[:idx], kv[idx+1:]
			switch key {
			case "product":
				d.Product = val
			case "model":
				d.Model = val
			case "device":
				d.DeviceName = val
			case "transport_id":
				d.TransportID = val
			}
		}
		devices = append(devices, d)
	}
	return devices
}

package adb

import (
	"context"
	"strings"
)

// Emu runs "adb -s <serial> emu <args...>" - the console-command forwarding
// mechanism adb itself provides for talking to a running emulator's control
// console (geo fix, network speed/delay, power, rotate, sms send, gsm
// call, ...), without this app needing its own telnet client for that
// console.
func (c *Client) Emu(ctx context.Context, serial string, args ...string) (string, error) {
	return c.RunForSerial(ctx, serial, append([]string{"emu"}, args...)...)
}

// EmuAVDName returns the AVD name backing the running emulator at serial,
// via "emu avd name". The console echoes the name followed by a trailing
// "OK" status line, which is stripped here so callers get just the name.
func (c *Client) EmuAVDName(ctx context.Context, serial string) (string, error) {
	out, err := c.Emu(ctx, serial, "avd", "name")
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		return "", nil
	}
	if len(lines) > 1 && strings.TrimSpace(lines[len(lines)-1]) == "OK" {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), nil
}

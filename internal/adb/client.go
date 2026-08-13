// Package adb wraps the adb command-line tool: device discovery, shell
// execution, file transfer, and dumpsys-based info gathering.
package adb

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Client runs adb commands against a specific adb binary.
type Client struct {
	BinPath string
	Timeout time.Duration
}

// New creates a Client for the adb binary at binPath, using a sensible
// default per-command timeout.
func New(binPath string) *Client {
	return &Client{BinPath: binPath, Timeout: 30 * time.Second}
}

// Run executes "adb <args...>" and returns its combined stdout/stderr,
// respecting the client's timeout on top of ctx.
func (c *Client) Run(ctx context.Context, args ...string) (string, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.BinPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("adb %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// RunForSerial is a convenience wrapper that prepends "-s <serial>".
func (c *Client) RunForSerial(ctx context.Context, serial string, args ...string) (string, error) {
	full := append([]string{"-s", serial}, args...)
	return c.Run(ctx, full...)
}

// Shell runs "adb -s <serial> shell <command>" as a single remote-shell
// argument, so pipes/redirects inside command are interpreted by the
// device's shell rather than the host's.
func (c *Client) Shell(ctx context.Context, serial, command string) (string, error) {
	return c.RunForSerial(ctx, serial, "shell", command)
}

// GetProp reads a single Android system property.
func (c *Client) GetProp(ctx context.Context, serial, key string) (string, error) {
	out, err := c.Shell(ctx, serial, "getprop "+key)
	return strings.TrimSpace(out), err
}

// Pull copies a file from the device to the host.
func (c *Client) Pull(ctx context.Context, serial, remotePath, localPath string) (string, error) {
	return c.RunForSerial(ctx, serial, "pull", remotePath, localPath)
}

// Push copies a file from the host to the device.
func (c *Client) Push(ctx context.Context, serial, localPath, remotePath string) (string, error) {
	return c.RunForSerial(ctx, serial, "push", localPath, remotePath)
}

// StartServer ensures the adb server is running.
func (c *Client) StartServer(ctx context.Context) (string, error) {
	return c.Run(ctx, "start-server")
}

// Version returns "adb version" output.
func (c *Client) Version(ctx context.Context) (string, error) {
	return c.Run(ctx, "version")
}

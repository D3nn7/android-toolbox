package main

import (
	"strings"
	"testing"

	"android-toolbox/internal/toolsmanager"
)

func TestUpdateStatusLineWhenUpToDate(t *testing.T) {
	line := updateStatusLine(toolsmanager.ToolUpdateStatus{Installed: "v4.1", Latest: "v4.1", Available: false})
	if !strings.Contains(line, "up to date") || !strings.Contains(line, "v4.1") {
		t.Fatalf("expected an up-to-date message mentioning the version, got %q", line)
	}
	if strings.Contains(line, "update available") {
		t.Fatalf("expected no update-available wording when up to date, got %q", line)
	}
}

func TestUpdateStatusLineWhenUpdateAvailable(t *testing.T) {
	line := updateStatusLine(toolsmanager.ToolUpdateStatus{Installed: "v4.0", Latest: "v4.1", Available: true})
	if !strings.Contains(line, "update available") || !strings.Contains(line, "v4.0") || !strings.Contains(line, "v4.1") {
		t.Fatalf("expected an update-available message mentioning both versions, got %q", line)
	}
}

func TestUpdateStatusLineWhenNeverInstalled(t *testing.T) {
	line := updateStatusLine(toolsmanager.ToolUpdateStatus{Installed: "", Latest: "v4.1", Available: true})
	if !strings.Contains(line, "unknown") {
		t.Fatalf("expected an empty Installed to render as 'unknown' rather than an empty string, got %q", line)
	}
}

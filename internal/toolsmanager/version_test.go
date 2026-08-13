package toolsmanager

import (
	"path/filepath"
	"testing"
)

func TestReadVersionMarkerMissingFileReturnsEmpty(t *testing.T) {
	if got := readVersionMarker(t.TempDir()); got != "" {
		t.Fatalf("expected an empty string for a directory with no marker, got %q", got)
	}
}

func TestWriteThenReadVersionMarkerRoundTrips(t *testing.T) {
	dir := t.TempDir()
	if err := writeVersionMarker(dir, "v4.1"); err != nil {
		t.Fatalf("writeVersionMarker: %v", err)
	}
	if got := readVersionMarker(dir); got != "v4.1" {
		t.Fatalf("expected the written version back, got %q", got)
	}
}

func TestReadVersionMarkerTrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	if err := writeVersionMarker(dir, "  v4.1 \n"); err != nil {
		t.Fatalf("writeVersionMarker: %v", err)
	}
	if got := readVersionMarker(dir); got != "v4.1" {
		t.Fatalf("expected whitespace trimmed, got %q", got)
	}
}

func TestVersionMarkerPathIsInsideTheToolDirectory(t *testing.T) {
	dir := filepath.Join("some", "tool", "dir")
	got := versionMarkerPath(dir)
	want := filepath.Join(dir, ".version")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestToolUpdateStatusAvailableReflectsWhetherVersionsDiffer(t *testing.T) {
	cases := []struct {
		name      string
		installed string
		latest    string
		want      bool
	}{
		{"same version", "v4.1", "v4.1", false},
		{"different version", "v4.0", "v4.1", true},
		{"never installed", "", "v4.1", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status := ToolUpdateStatus{Installed: c.installed, Latest: c.latest, Available: c.installed != c.latest}
			if status.Available != c.want {
				t.Fatalf("Available = %v, want %v", status.Available, c.want)
			}
		})
	}
}

package app

import (
	"strings"
	"testing"
)

func TestRenderOutputLinePreservesReadableContent(t *testing.T) {
	// lipgloss disables ANSI styling when it doesn't detect a real terminal
	// (e.g. under `go test`), so this can't assert that styling bytes were
	// actually emitted - only that renderOutputLine routes each format
	// through the right classifier and never loses or garbles the
	// underlying text. Visual styling itself is covered by
	// internal/output's classifier tests plus manual verification in a
	// real terminal.
	cases := []struct {
		name   string
		format string
		line   string
		want   string // expected content after stripping any ANSI styling
	}{
		{
			name:   "logcat colors but keeps the whole line",
			format: "logcat",
			line:   `08-11 12:06:24.522   872   872 E sehhealth-service: Could not open /sys/class/power_supply/battery/lrp`,
			want:   `08-11 12:06:24.522   872   872 E sehhealth-service: Could not open /sys/class/power_supply/battery/lrp`,
		},
		{
			name:   "keyvalue reconstructs the line exactly",
			format: "keyvalue",
			line:   "  level: 100",
			want:   "  level: 100",
		},
		{
			name:   "packages strips the package: prefix by design",
			format: "packages",
			line:   "package:com.microsoft.office.outlook",
			want:   "com.microsoft.office.outlook",
		},
		{
			name:   "unrecognized format is passed through",
			format: "",
			line:   "plain output line",
			want:   "plain output line",
		},
	}

	m := Model{styles: newStyles()}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripANSI(m.renderOutputLine(c.format, c.line))
			if got != c.want {
				t.Fatalf("renderOutputLine(%q, %q) content = %q, want %q", c.format, c.line, got, c.want)
			}
		})
	}
}

// stripANSI removes CSI escape sequences so assertions can check the
// underlying text regardless of whether the environment's color profile
// (which lipgloss auto-detects) actually emits styling.
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

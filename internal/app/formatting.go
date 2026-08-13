package app

import (
	"android-toolbox/internal/output"
)

// styleForKind maps a classified output.Kind onto this app's color palette.
func (m Model) styleForKind(k output.Kind, s string) string {
	switch k {
	case output.KindError:
		return m.styles.Error.Render(s)
	case output.KindWarn:
		return m.styles.Warn.Render(s)
	case output.KindInfo:
		return m.styles.Info.Render(s)
	case output.KindDebug, output.KindVerbose:
		return m.styles.Subtle.Render(s)
	case output.KindLabel:
		return m.styles.Highlight.Render(s)
	default:
		return s
	}
}

// renderOutputLine highlights one line of a running action's output
// according to its Format, instead of showing everything as flat,
// undifferentiated text. Actions without a recognized Format (the default)
// are returned unchanged.
func (m Model) renderOutputLine(format, line string) string {
	var segs []output.Segment
	switch format {
	case "logcat":
		segs = output.Logcat(line)
	case "keyvalue":
		segs = output.KeyValue(line)
	case "packages":
		segs = output.Packages(line)
	default:
		return line
	}
	return output.Render(segs, m.styleForKind)
}

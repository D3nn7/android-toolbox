// Package output classifies one line of an action's raw text output into
// styled segments (e.g. a logcat error line, a "key: value" pair), so the
// TUI can highlight it instead of dumping everything as flat, undifferentiated
// text. Classification is pure text parsing with no rendering dependency -
// internal/app maps the resulting Kind values onto actual lipgloss styles.
package output

import (
	"regexp"
	"strings"
)

// Kind categorises a Segment for styling purposes.
type Kind int

const (
	KindNormal Kind = iota
	KindError
	KindWarn
	KindInfo
	KindDebug
	KindVerbose
	KindLabel
)

// Segment is one styled run of text within a line.
type Segment struct {
	Text string
	Kind Kind
}

// plain is a convenience constructor for an unclassified, single-segment line.
func plain(line string) []Segment {
	return []Segment{{Text: line, Kind: KindNormal}}
}

// logcatBriefRe matches adb's "brief" style, optionally time-prefixed:
// "[date time ]LEVEL/tag(pid): message" - e.g. what `logcat -v time` prints.
var logcatBriefRe = regexp.MustCompile(`^(.*?)([EWIDVF])/([^\s(]+)\(\s*\d+\):\s?(.*)$`)

// logcatThreadtimeRe matches adb's "threadtime" style:
// "date time pid tid LEVEL tag: message" - what a bare `logcat` prints on
// real devices observed during development.
var logcatThreadtimeRe = regexp.MustCompile(`^(\S+\s+\S+)\s+(\d+)\s+(\d+)\s+([EWIDVF])\s+(.*)$`)

// Logcat colors an entire logcat line by its priority level (adb's "brief"
// style - "LEVEL/tag(pid): message", optionally time-prefixed - or its
// "threadtime" style - "date time pid tid LEVEL tag: message", both seen on
// real devices during development). Lines matching neither shape (e.g. the
// "--------- beginning of main" separator adb prints) are returned
// unclassified rather than mis-highlighted. Coloring the whole line rather
// than just the level letter keeps this robust against the two formats'
// differing column widths instead of relying on precise byte offsets.
func Logcat(line string) []Segment {
	if m := logcatBriefRe.FindStringSubmatch(line); m != nil {
		return []Segment{{Text: line, Kind: kindForLevel(m[2])}}
	}
	if m := logcatThreadtimeRe.FindStringSubmatch(line); m != nil {
		return []Segment{{Text: line, Kind: kindForLevel(m[4])}}
	}
	return plain(line)
}

func kindForLevel(level string) Kind {
	switch level {
	case "E", "F":
		return KindError
	case "W":
		return KindWarn
	case "I":
		return KindInfo
	case "D":
		return KindDebug
	case "V":
		return KindVerbose
	default:
		return KindNormal
	}
}

// KeyValue splits a "key: value" or "key=value" line into a bold label
// segment and a plain value segment. Lines without a recognizable separator
// (blank lines, section headers) are returned unclassified.
func KeyValue(line string) []Segment {
	trimmed := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(trimmed)]

	sep := ":"
	idx := strings.Index(trimmed, sep)
	if idx < 0 {
		sep = "="
		idx = strings.Index(trimmed, sep)
	}
	if idx <= 0 {
		return plain(line)
	}

	label := trimmed[:idx]
	value := trimmed[idx+len(sep):]
	return []Segment{
		{Text: indent + label + sep, Kind: KindLabel},
		{Text: value, Kind: KindNormal},
	}
}

// Packages strips the "package:" prefix `pm list packages` prints on every
// line, highlighting the bare package name.
func Packages(line string) []Segment {
	const prefix = "package:"
	if !strings.HasPrefix(line, prefix) {
		return plain(line)
	}
	return []Segment{{Text: strings.TrimPrefix(line, prefix), Kind: KindLabel}}
}

// Render applies segments to renderFn (typically a lipgloss style lookup)
// and concatenates the result - a small helper so callers don't repeat the
// same loop.
func Render(segs []Segment, renderFn func(Kind, string) string) string {
	var b strings.Builder
	for _, seg := range segs {
		b.WriteString(renderFn(seg.Kind, seg.Text))
	}
	return b.String()
}

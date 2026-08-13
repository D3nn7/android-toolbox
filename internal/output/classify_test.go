package output

import "testing"

func kindsOf(segs []Segment) []Kind {
	kinds := make([]Kind, len(segs))
	for i, s := range segs {
		kinds[i] = s.Kind
	}
	return kinds
}

func TestLogcatThreadtimeRealCapture(t *testing.T) {
	cases := []struct {
		line string
		want Kind
	}{
		{`08-11 12:06:24.522   872   872 E sehhealth-service: Could not open /sys/class/power_supply/battery/lrp`, KindError},
		{`08-11 12:06:25.608  1212  1273 D MARsPolicyManager: triggerAction called!`, KindDebug},
		{`08-11 12:06:25.608  1212  1273 I NSLocationMonitor: getGPSUsingApps() called`, KindInfo},
	}
	for _, c := range cases {
		segs := Logcat(c.line)
		if len(segs) != 1 || segs[0].Kind != c.want || segs[0].Text != c.line {
			t.Fatalf("Logcat(%q) = %+v, want single segment kind=%v", c.line, segs, c.want)
		}
	}
}

func TestLogcatBriefTimeRealCapture(t *testing.T) {
	cases := []struct {
		line string
		want Kind
	}{
		{`08-11 12:06:40.660 I/NSLocationMonitor( 1212): getGPSUsingApps() called`, KindInfo},
		{`08-11 12:06:41.920 E/Watchdog( 1212): !@Sync: 8953 heap: 77 / 94 FD: 911`, KindError},
	}
	for _, c := range cases {
		segs := Logcat(c.line)
		if len(segs) != 1 || segs[0].Kind != c.want {
			t.Fatalf("Logcat(%q) = %+v, want kind=%v", c.line, segs, c.want)
		}
	}
}

func TestLogcatUnmatchedLineStaysUnclassified(t *testing.T) {
	segs := Logcat("--------- beginning of main")
	if len(segs) != 1 || segs[0].Kind != KindNormal {
		t.Fatalf("expected unclassified separator line, got %+v", segs)
	}
}

func TestKeyValueRealBatteryCapture(t *testing.T) {
	segs := KeyValue("  level: 100")
	if len(segs) != 2 || segs[0].Kind != KindLabel || segs[0].Text != "  level:" || segs[1].Text != " 100" {
		t.Fatalf("unexpected segments: %+v", segs)
	}
}

func TestKeyValueEqualsSeparator(t *testing.T) {
	segs := KeyValue("Modell=SM-T575")
	if len(segs) != 2 || segs[0].Text != "Modell=" || segs[1].Text != "SM-T575" {
		t.Fatalf("unexpected segments: %+v", segs)
	}
}

func TestKeyValueLineWithoutSeparatorIsUnclassified(t *testing.T) {
	segs := KeyValue("Current Battery Service state:")
	// Note: this line DOES contain a trailing colon with nothing after it -
	// idx would be len-1, which is > 0, so it's still classified as
	// label="Current Battery Service state" value="". That's acceptable
	// (still renders sensibly); verify it doesn't panic and produces 2 segs.
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments even for a trailing-colon header, got %+v", segs)
	}

	segs = KeyValue("no separator here")
	if len(segs) != 1 || segs[0].Kind != KindNormal {
		t.Fatalf("expected unclassified line, got %+v", segs)
	}
}

func TestPackagesRealCapture(t *testing.T) {
	segs := Packages("package:com.microsoft.office.outlook")
	if len(segs) != 1 || segs[0].Kind != KindLabel || segs[0].Text != "com.microsoft.office.outlook" {
		t.Fatalf("unexpected segments: %+v", segs)
	}
}

func TestPackagesLineWithoutPrefix(t *testing.T) {
	segs := Packages("some other output")
	if len(segs) != 1 || segs[0].Kind != KindNormal {
		t.Fatalf("expected unclassified line, got %+v", segs)
	}
}

func TestRenderConcatenatesStyledSegments(t *testing.T) {
	segs := []Segment{{Text: "a", Kind: KindLabel}, {Text: "b", Kind: KindNormal}}
	got := Render(segs, func(k Kind, s string) string {
		if k == KindLabel {
			return "[" + s + "]"
		}
		return s
	})
	if got != "[a]b" {
		t.Fatalf("Render() = %q, want %q", got, "[a]b")
	}
}

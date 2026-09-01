package app

import "testing"

func TestParsePercent(t *testing.T) {
	cases := []struct {
		line string
		want int
	}{
		{"42% - commandlinetools-linux-15859902_latest.zip", 42},
		{"Fetching https://dl.google.com/... 7%", 7},
		{"|====                              | 100%", 100},
		{"Unpacking system-images;android-34;google_apis;x86_64", -1},
		{"150% overshoot clamps", 100},
	}
	for _, c := range cases {
		if got := parsePercent(c.line); got != c.want {
			t.Errorf("parsePercent(%q) = %d, want %d", c.line, got, c.want)
		}
	}
}

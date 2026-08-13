package selfupdate

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"v1.0.0", "1.0.0", 0}, // leading "v" is ignored
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.2.0", "1.10.0", -1}, // numeric, not lexicographic, comparison
		{"1.9.9", "2.0.0", -1},
		{"2.0.0", "1.9.9", 1},
		{"0.1.0", "0.1.0-dev", 0}, // pre-release suffix is dropped, not compared
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestIsNewer(t *testing.T) {
	if !IsNewer("1.0.0", "1.0.1") {
		t.Fatal("expected 1.0.1 to be newer than 1.0.0")
	}
	if IsNewer("1.0.1", "1.0.0") {
		t.Fatal("expected 1.0.0 to NOT be newer than 1.0.1")
	}
	if IsNewer("1.0.0", "1.0.0") {
		t.Fatal("expected equal versions to not count as newer")
	}
}

func TestParseVersionToleratesGarbage(t *testing.T) {
	got := parseVersion("not-a-version")
	want := [3]int{0, 0, 0}
	if got != want {
		t.Fatalf("expected an unparseable string to degrade to 0.0.0, got %v", got)
	}
}

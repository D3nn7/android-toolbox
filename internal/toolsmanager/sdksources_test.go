package toolsmanager

import (
	"strings"
	"testing"
)

func TestCmdlineToolsURL(t *testing.T) {
	cases := []struct {
		goos, arch   string
		wantContains string
		wantErr      bool
	}{
		{"windows", "amd64", "commandlinetools-win-", false},
		{"linux", "amd64", "commandlinetools-linux-", false},
		{"darwin", "arm64", "commandlinetools-mac_arm64-", false},
		{"darwin", "amd64", "commandlinetools-mac_x86_64-", false},
		{"plan9", "amd64", "", true},
	}
	for _, c := range cases {
		url, filename, err := cmdlineToolsURL(c.goos, c.arch)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s/%s: expected an error, got url %q", c.goos, c.arch, url)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s/%s: unexpected error: %v", c.goos, c.arch, err)
		}
		if !strings.Contains(url, c.wantContains) || !strings.Contains(filename, c.wantContains) {
			t.Errorf("%s/%s: url %q / filename %q missing %q", c.goos, c.arch, url, filename, c.wantContains)
		}
	}
}

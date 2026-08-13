package device

import "testing"

func TestParseWMSize(t *testing.T) {
	got := parseWMSize("Physical size: 1200x1920\n")
	if got != "1200x1920" {
		t.Fatalf("unexpected resolution: %q", got)
	}
}

func TestIPRegexpMatchesRealIpAddrOutput(t *testing.T) {
	out := `16: wlan0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP group default qlen 1000
    inet 192.168.42.97/24 brd 192.168.42.255 scope global dynamic wlan0
       valid_lft 22667sec preferred_lft 22667sec
`
	m := ipRegexp.FindStringSubmatch(out)
	if m == nil || m[1] != "192.168.42.97" {
		t.Fatalf("expected to extract 192.168.42.97, got %v", m)
	}
}

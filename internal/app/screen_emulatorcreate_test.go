package app

import (
	"runtime"
	"testing"
)

// TestAndroidVersionLabelDoesNotConfuseAPILevelWithMarketingVersion guards
// against exactly the mix-up a user hit: "android-17" is API level 17,
// which is Android 4.2 (Jelly Bean) - not "Android 17".
func TestAndroidVersionLabelDoesNotConfuseAPILevelWithMarketingVersion(t *testing.T) {
	cases := []struct {
		pkg  string
		want string
	}{
		{"system-images;android-17;google_apis;x86", "Android 4.2 (Jelly Bean)"},
		{"system-images;android-34;google_apis;x86_64", "Android 14"},
		{"system-images;android-35;google_apis_playstore;x86_64", "Android 15"},
		{"system-images;android-999;google_apis;x86_64", ""}, // unrecognized API level
		{"platforms;android-34", "Android 14"},
		{"emulator", ""}, // no API level segment at all
	}
	for _, c := range cases {
		if got := androidVersionLabel(c.pkg); got != c.want {
			t.Errorf("androidVersionLabel(%q) = %q, want %q", c.pkg, got, c.want)
		}
	}
}

// TestSystemImageMatchesHostExcludesArmOnAmd64 guards against exactly the
// crash a user hit: the emulator refuses an arm64-v8a AVD outright on an
// x86_64 host ("Avd's CPU Architecture 'arm64' is not supported by the
// QEMU2 emulator on x86_64 host"), so that combination must never reach the
// picker in the first place.
func TestSystemImageMatchesHostExcludesArmOnAmd64(t *testing.T) {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "386" {
		t.Skipf("this test assumes an x86/x86_64 host, got GOARCH=%s", runtime.GOARCH)
	}
	cases := []struct {
		pkg  string
		want bool
	}{
		{"system-images;android-36;google_apis;arm64-v8a", false},
		{"system-images;android-19;default;armeabi-v7a", false},
		{"system-images;android-35;google_apis;x86_64", true},
		{"system-images;android-17;google_apis;x86", true},
	}
	for _, c := range cases {
		if got := systemImageMatchesHost(c.pkg); got != c.want {
			t.Errorf("systemImageMatchesHost(%q) = %v, want %v", c.pkg, got, c.want)
		}
	}
}

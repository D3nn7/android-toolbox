package avd

import (
	"strings"
	"testing"
)

const realAvdListOutput = `Available Android Virtual Devices:
    Name: Pixel_6_API_34
    Device: pixel_6 (Google)
    Path: /home/user/.android/avd/Pixel_6_API_34.avd
    Target: Google APIs (Google Inc.)
            Based on: Android 14.0 (UpsideDownCake) Tag/ABI: google_apis/x86_64
    Skin: pixel_6
    Sdcard: 512M
---------
    Name: Nexus_5_API_30
    Device: Nexus 5 (Google)
    Path: /home/user/.android/avd/Nexus_5_API_30.avd
    Target: Google APIs (Google Inc.)
            Based on: Android 11.0 (R) Tag/ABI: google_apis/x86
---------
`

func TestParseAVDList(t *testing.T) {
	avds := parseAVDList(realAvdListOutput)
	if len(avds) != 2 {
		t.Fatalf("expected 2 AVDs, got %d: %+v", len(avds), avds)
	}

	first := avds[0]
	if first.Name != "Pixel_6_API_34" || first.Device != "pixel_6 (Google)" || first.ABI != "google_apis/x86_64" {
		t.Fatalf("unexpected first AVD: %+v", first)
	}
	if first.Path != "/home/user/.android/avd/Pixel_6_API_34.avd" {
		t.Fatalf("unexpected path: %q", first.Path)
	}

	second := avds[1]
	if second.Name != "Nexus_5_API_30" || second.ABI != "google_apis/x86" {
		t.Fatalf("unexpected second AVD: %+v", second)
	}
}

func TestParseAVDListEmpty(t *testing.T) {
	if avds := parseAVDList("Available Android Virtual Devices:\n"); len(avds) != 0 {
		t.Fatalf("expected 0 AVDs, got %d", len(avds))
	}
}

// realAvdListWithBrokenEntries is a verbatim capture from a real machine:
// one healthy AVD plus three whose system image had since been removed
// (e.g. via sdkmanager, after the AVD was created by Android Studio) -
// avdmanager lists those separately, with only Name/Path/Error, no
// Device/Target/Tag-ABI. Before AVD.Broken existed, these parsed as
// ordinary AVDs with silently blank Target/ABI/Device instead of being
// flagged as unusable.
const realAvdListWithBrokenEntries = `Available Android Virtual Devices:
    Name: Test_Device_1
  Device: pixel_8a (Google)
    Path: C:\Users\d.schapeit\.android\avd\Test_Device_1.avd
  Target: Google APIs (Google Inc.)
          Based on: Android 16.0 ("Baklava") Tag/ABI: google_apis/x86_64
  Sdcard: 512 MB

The following Android Virtual Devices could not be loaded:
    Name: Medium_Phone_API_36.1
    Path: C:\Users\d.schapeit\.android\avd\Medium_Phone.avd
   Error: Missing system image android-36.1\google_apis_playstore\x86_64.
---------
    Name: Pixel_9a
    Path: C:\Users\d.schapeit\.android\avd\Pixel_9a.avd
   Error: Missing system image android-37.1\google_apis_playstore_ps16k\x86_64.
---------
    Name: Pixel_9a_6_GB_-_Android_17
    Path: C:\Users\d.schapeit\.android\avd\Pixel_9a_6_GB_-_Android_17.avd
   Error: Missing system image android-37.1\google_apis_playstore_ps16k\x86_64.
`

func TestParseAVDListFlagsBrokenEntries(t *testing.T) {
	avds := parseAVDList(realAvdListWithBrokenEntries)
	if len(avds) != 4 {
		t.Fatalf("expected 4 AVDs, got %d: %+v", len(avds), avds)
	}

	healthy := avds[0]
	if healthy.Name != "Test_Device_1" || healthy.Broken || healthy.ABI != "google_apis/x86_64" {
		t.Fatalf("expected a healthy, non-broken first AVD: %+v", healthy)
	}

	for _, broken := range avds[1:] {
		if !broken.Broken {
			t.Errorf("expected %q to be flagged Broken, got %+v", broken.Name, broken)
		}
		if broken.Error == "" {
			t.Errorf("expected %q to carry its Error message, got empty", broken.Name)
		}
		if broken.ABI != "" || broken.Target != "" {
			t.Errorf("expected a broken AVD to have no ABI/Target, got %+v", broken)
		}
	}
	if avds[1].Name != "Medium_Phone_API_36.1" || avds[1].Error != "Missing system image android-36.1\\google_apis_playstore\\x86_64." {
		t.Fatalf("unexpected broken AVD: %+v", avds[1])
	}
}

const realDeviceListOutput = `Available devices definitions:
id: 0 or "Nexus 5"
    Name: Nexus 5
    OEM : Google
---------
id: 1 or "pixel_6"
    Name: Pixel 6
    OEM : Google
---------
`

func TestListDeviceProfilesParsing(t *testing.T) {
	var ids []string
	for _, line := range strings.Split(realDeviceListOutput, "\n") {
		if m := deviceIDRe.FindStringSubmatch(line); m != nil {
			ids = append(ids, m[1])
		}
	}
	if len(ids) != 2 || ids[0] != "Nexus 5" || ids[1] != "pixel_6" {
		t.Fatalf("unexpected device ids: %v", ids)
	}
}

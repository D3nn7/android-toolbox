package toolsmanager

import (
	"bufio"
	"strings"
	"testing"
)

const realSdkListOutput = `Installed packages:
  Path                                          | Version | Description                    | Location
  -------                                       | -------  | -------                        | -------
  platform-tools                                | 34.0.4  | Android SDK Platform-Tools     | platform-tools
  system-images;android-34;google_apis;x86_64   | 9       | Google APIs Intel x86_64 Atom System Image | system-images/android-34/google_apis/x86_64

Available Packages:
  Path                                                     | Version | Description
  -------                                                  | -------  | -------
  add-ons;addon-google_apis-google-15                      | 3        | Google APIs
  emulator                                                 | 35.1.4   | Android Emulator
  system-images;android-35;google_apis;x86_64              | 3        | Google APIs Intel x86_64 Atom System Image

`

func TestParseSdkPackageList(t *testing.T) {
	installed, available := parseSdkPackageList(realSdkListOutput)

	if len(installed) != 2 {
		t.Fatalf("expected 2 installed packages, got %d: %v", len(installed), installed)
	}
	if installed[0] != "platform-tools" || installed[1] != "system-images;android-34;google_apis;x86_64" {
		t.Fatalf("unexpected installed packages: %v", installed)
	}

	if len(available) != 3 {
		t.Fatalf("expected 3 available packages, got %d: %v", len(available), available)
	}
	if available[1] != "emulator" || available[2] != "system-images;android-35;google_apis;x86_64" {
		t.Fatalf("unexpected available packages: %v", available)
	}
}

func TestScanLinesOrCR(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("line one\rline two\nline three"))
	scanner.Split(scanLinesOrCR)

	var got []string
	for scanner.Scan() {
		got = append(got, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	want := []string{"line one", "line two", "line three"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

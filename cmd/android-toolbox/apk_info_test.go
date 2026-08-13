package main

import (
	"strings"
	"testing"

	"android-toolbox/internal/apkinfo"
)

func TestFormatByteSize(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
	}
	for _, c := range cases {
		if got := formatByteSize(c.n); got != c.want {
			t.Errorf("formatByteSize(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestFormatAPKInfoIncludesKeyFields(t *testing.T) {
	info := apkinfo.Info{
		Path:       "test.apk",
		SizeBytes:  2048,
		SHA256:     "deadbeef",
		EntryCount: 3,
		Manifest: apkinfo.ManifestInfo{
			PackageName:      "com.example.app",
			VersionCode:      42,
			VersionName:      "1.2.3",
			MinSDK:           21,
			TargetSDK:        33,
			ApplicationLabel: "Example App",
			MainActivity:     ".MainActivity",
			Permissions:      []string{"android.permission.INTERNET"},
		},
		Signing: apkinfo.SigningInfo{
			SchemeV2: true,
			Certificates: []apkinfo.CertInfo{
				{Subject: "CN=Test", Issuer: "CN=Test", SerialNumber: "1", NotBefore: "2024-01-01", NotAfter: "2050-01-01", SHA256: "cafebabe"},
			},
		},
	}

	out := formatAPKInfo(info)

	for _, want := range []string{
		"com.example.app", "1.2.3", "42", "21", "33", "Example App",
		".MainActivity", "android.permission.INTERNET", "v2", "CN=Test", "cafebabe",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestFormatAPKInfoUnsigned(t *testing.T) {
	info := apkinfo.Info{Manifest: apkinfo.ManifestInfo{PackageName: "com.example.unsigned"}}
	out := formatAPKInfo(info)
	if !strings.Contains(out, "No signature block found") {
		t.Errorf("expected an 'unsigned' message, got:\n%s", out)
	}
}

func TestFormatAPKInfoV1Only(t *testing.T) {
	info := apkinfo.Info{Signing: apkinfo.SigningInfo{SchemeV1Only: true}}
	out := formatAPKInfo(info)
	if !strings.Contains(out, "v1 (JAR signature)") {
		t.Errorf("expected a v1-only message, got:\n%s", out)
	}
}

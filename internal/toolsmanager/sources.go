package toolsmanager

import "fmt"

// fallbackScrcpyVersion is used when the GitHub "latest release" API cannot
// be reached (offline, rate-limited, ...). It is bumped from time to time but
// does not need to track upstream in lock-step because resolveScrcpyVersion
// always prefers the live value.
const fallbackScrcpyVersion = "v4.1"

const scrcpyLatestReleaseAPI = "https://api.github.com/repos/Genymobile/scrcpy/releases/latest"

// platformToolsURL returns Google's official, always-current platform-tools
// download URL for the given GOOS.
func platformToolsURL(goos string) (string, error) {
	name, ok := map[string]string{
		"windows": "windows",
		"linux":   "linux",
		"darwin":  "darwin",
	}[goos]
	if !ok {
		return "", fmt.Errorf("no platform-tools package known for GOOS %q", goos)
	}
	return fmt.Sprintf("https://dl.google.com/android/repository/platform-tools-latest-%s.zip", name), nil
}

// scrcpyAssetName maps a GOOS/GOARCH pair to the asset name fragment used in
// scrcpy's GitHub release, and whether that asset is a .zip or .tar.gz.
func scrcpyAssetName(goos, arch string) (assetKey string, isZip bool, err error) {
	switch goos {
	case "windows":
		switch arch {
		case "amd64":
			return "win64", true, nil
		case "386":
			return "win32", true, nil
		}
	case "linux":
		if arch == "amd64" {
			return "linux-x86_64", false, nil
		}
	case "darwin":
		switch arch {
		case "amd64":
			return "macos-x86_64", false, nil
		case "arm64":
			return "macos-aarch64", false, nil
		}
	}
	return "", false, fmt.Errorf("no scrcpy release known for %s/%s", goos, arch)
}

func scrcpyAssetURL(version, goos, arch string) (url string, filename string, err error) {
	key, isZip, err := scrcpyAssetName(goos, arch)
	if err != nil {
		return "", "", err
	}
	ext := "tar.gz"
	if isZip {
		ext = "zip"
	}
	filename = fmt.Sprintf("scrcpy-%s-%s.%s", key, version, ext)
	url = fmt.Sprintf("https://github.com/Genymobile/scrcpy/releases/download/%s/%s", version, filename)
	return url, filename, nil
}

func scrcpySumsURL(version string) string {
	return fmt.Sprintf("https://github.com/Genymobile/scrcpy/releases/download/%s/SHA256SUMS.txt", version)
}

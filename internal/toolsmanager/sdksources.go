package toolsmanager

import "fmt"

// cmdlineToolsRevision is the pinned Android SDK "command-line tools"
// revision this app bootstraps. Unlike platform-tools (which Google serves
// under a stable "-latest-" URL, see platformToolsURL) or scrcpy (which has
// a GitHub "latest release" API, see resolveScrcpyVersion), Google does not
// publish any stable alias for cmdline-tools - every revision has its own
// numbered filename, documented only on
// https://developer.android.com/studio#command-tools. This constant must
// therefore be bumped by hand from time to time, exactly like
// fallbackScrcpyVersion already is - it does not need to track upstream in
// lock-step, since sdkmanager (once bootstrapped) resolves everything it
// installs afterwards (system images, platforms, the emulator package)
// live against Google's servers regardless of which cmdline-tools revision
// fetched it.
const cmdlineToolsRevision = "15859902"

// cmdlineToolsAssetName maps a GOOS/GOARCH pair to the asset name fragment
// used in the cmdline-tools download filename. Windows and Linux ship one
// arch-independent build each; macOS ships separate Intel/Apple Silicon
// builds.
func cmdlineToolsAssetName(goos, arch string) (string, error) {
	switch goos {
	case "windows":
		return "win", nil
	case "linux":
		return "linux", nil
	case "darwin":
		switch arch {
		case "arm64":
			return "mac_arm64", nil
		default:
			return "mac_x86_64", nil
		}
	}
	return "", fmt.Errorf("no cmdline-tools package known for %s/%s", goos, arch)
}

// cmdlineToolsURL returns the pinned download URL for goos/arch's
// cmdline-tools build.
func cmdlineToolsURL(goos, arch string) (url string, filename string, err error) {
	key, err := cmdlineToolsAssetName(goos, arch)
	if err != nil {
		return "", "", err
	}
	filename = fmt.Sprintf("commandlinetools-%s-%s_latest.zip", key, cmdlineToolsRevision)
	url = fmt.Sprintf("https://dl.google.com/android/repository/%s", filename)
	return url, filename, nil
}

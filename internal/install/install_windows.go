//go:build windows

package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

// Install copies exePath to %LOCALAPPDATA%\Programs\<appName>\ under both
// appName.exe and aliasName.exe, then adds that directory to the current
// user's PATH registry value (HKCU\Environment) if it isn't there already.
func Install(exePath, appName, aliasName string) (Result, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return Result{}, fmt.Errorf("LOCALAPPDATA is not set")
	}

	installDir := filepath.Join(localAppData, "Programs", appName)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("could not create install directory: %w", err)
	}

	mainDest := filepath.Join(installDir, appName+".exe")
	aliasDest := filepath.Join(installDir, aliasName+".exe")
	for _, dest := range []string{mainDest, aliasDest} {
		if err := copyFile(exePath, dest); err != nil {
			return Result{}, fmt.Errorf("copying to %s failed: %w", dest, err)
		}
	}

	alreadyOnPath, err := addToUserPath(installDir)
	if err != nil {
		return Result{}, fmt.Errorf("could not update PATH: %w", err)
	}

	note := ""
	if !alreadyOnPath {
		note = "Open a new terminal for the PATH change to take effect."
	}

	return Result{
		InstallDir:     installDir,
		InstalledFiles: []string{mainDest, aliasDest},
		OnPath:         true,
		Note:           note,
	}, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755)
}

// addToUserPath adds dir to HKCU\Environment\Path if it isn't already
// present, then broadcasts WM_SETTINGCHANGE so already-running processes
// that care (e.g. Explorer) pick it up. It returns true if dir was already
// on PATH (no change made).
func addToUserPath(dir string) (alreadyPresent bool, err error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()

	existing, _, err := k.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return false, err
	}

	for _, part := range strings.Split(existing, ";") {
		if strings.EqualFold(strings.TrimSpace(part), dir) {
			return true, nil
		}
	}

	newPath := dir
	if strings.TrimSpace(existing) != "" {
		newPath = strings.TrimRight(existing, ";") + ";" + dir
	}
	if err := k.SetStringValue("Path", newPath); err != nil {
		return false, err
	}

	broadcastEnvironmentChange()
	return false, nil
}

const (
	hwndBroadcast    = 0xffff
	wmSettingChange  = 0x001A
	smtoAbortIfHung  = 0x0002
	broadcastTimeout = 5000
)

// broadcastEnvironmentChange notifies other top-level windows that the
// environment changed, matching what installers conventionally do so a
// freshly opened shell picks up the new PATH without a full logoff.
func broadcastEnvironmentChange() {
	user32 := syscall.NewLazyDLL("user32.dll")
	sendMessageTimeout := user32.NewProc("SendMessageTimeoutW")

	param, err := syscall.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	sendMessageTimeout.Call(
		uintptr(hwndBroadcast),
		uintptr(wmSettingChange),
		0,
		uintptr(unsafe.Pointer(param)),
		uintptr(smtoAbortIfHung),
		uintptr(broadcastTimeout),
		0,
	)
}

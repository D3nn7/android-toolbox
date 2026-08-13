// Package install copies the running binary into a stable per-user location
// and makes it (plus a short alias) reachable from any shell, without
// requiring administrator privileges.
package install

// Result describes what Install actually did, for display to the user.
type Result struct {
	InstallDir     string
	InstalledFiles []string
	OnPath         bool
	// Note carries any follow-up the user needs to do manually (e.g. add a
	// line to their shell rc file) when this platform can't fully automate
	// PATH registration.
	Note string
}

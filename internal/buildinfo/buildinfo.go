// Package buildinfo holds version metadata, overridable at build time via
// -ldflags "-X android-toolbox/internal/buildinfo.Version=...". The
// repo-root VERSION file is the single source of truth for the current
// semantic version - `make build` reads it locally, and the release
// GitHub Actions workflow (.github/workflows/release.yml) bumps it
// automatically on every push to main before building and tagging a
// release. The fallback below is only what an unadorned `go build`
// (bypassing both) shows.
package buildinfo

var (
	// Version is the semantic version of this build.
	Version = "0.1.0-dev"
	// Commit is the git commit hash this build was produced from.
	Commit = "unknown"
)

// RepoURL is the project's canonical source repository, shown alongside the
// version in the TUI (healthcheck and settings screens) so users always
// know where to find docs/issues for the build they're running.
const RepoURL = "https://github.com/d3nn7/android-toolbox"

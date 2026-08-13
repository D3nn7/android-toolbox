// Package selfupdate checks GitHub for a newer android-toolbox release than
// the one currently running, and can replace the running executable with
// it. It deliberately only understands the plain "MAJOR.MINOR.PATCH" scheme
// this project's own release pipeline produces (see VERSION and
// .github/workflows/release.yml) - no pre-release/build-metadata suffixes.
package selfupdate

import (
	"strconv"
	"strings"
)

// parseVersion splits a "vMAJOR.MINOR.PATCH" or "MAJOR.MINOR.PATCH" string
// into its three numeric components. Any missing or non-numeric component
// is treated as 0 rather than erroring, so a malformed or unexpected tag
// (e.g. a manually-created release with an unusual name) degrades to "looks
// like 0.0.0" instead of blocking the whole update check.
func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.SplitN(v, ".", 3)
	var out [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		// Drop any trailing pre-release/build suffix (e.g. "3-rc1") rather
		// than letting it fail the numeric parse entirely.
		numeric, _, _ := strings.Cut(parts[i], "-")
		n, err := strconv.Atoi(numeric)
		if err != nil {
			continue
		}
		out[i] = n
	}
	return out
}

// CompareVersions returns -1 if a < b, 0 if a == b, and 1 if a > b, treating
// both as MAJOR.MINOR.PATCH (an optional leading "v" is ignored, so "v1.2.0"
// and "1.2.0" compare equal).
func CompareVersions(a, b string) int {
	va, vb := parseVersion(a), parseVersion(b)
	for i := 0; i < 3; i++ {
		switch {
		case va[i] < vb[i]:
			return -1
		case va[i] > vb[i]:
			return 1
		}
	}
	return 0
}

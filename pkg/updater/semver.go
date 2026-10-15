package updater

import (
	"strconv"
	"strings"
)

// Version represents SemVer elements (major.minor.patch)
type Version struct {
	Major int
	Minor int
	Patch int
	Raw   string
}

// ParseVersion extracts semver numbers from strings like "v1.2.0", "1.2.3", "v1.2.0-beta"
func ParseVersion(v string) Version {
	clean := strings.TrimSpace(v)
	clean = strings.TrimPrefix(clean, "v")
	clean = strings.TrimPrefix(clean, "V")

	// Cut off pre-release or build metadata (e.g. "-beta", "+build123")
	if idx := strings.IndexAny(clean, "-+"); idx != -1 {
		clean = clean[:idx]
	}

	parts := strings.Split(clean, ".")
	var major, minor, patch int
	if len(parts) > 0 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) > 2 {
		patch, _ = strconv.Atoi(parts[2])
	}

	return Version{
		Major: major,
		Minor: minor,
		Patch: patch,
		Raw:   v,
	}
}

// CompareVersions returns:
//  -1 if v1 < v2
//   0 if v1 == v2
//   1 if v1 > v2
func CompareVersions(v1, v2 string) int {
	p1 := ParseVersion(v1)
	p2 := ParseVersion(v2)

	if p1.Major != p2.Major {
		if p1.Major > p2.Major {
			return 1
		}
		return -1
	}

	if p1.Minor != p2.Minor {
		if p1.Minor > p2.Minor {
			return 1
		}
		return -1
	}

	if p1.Patch != p2.Patch {
		if p1.Patch > p2.Patch {
			return 1
		}
		return -1
	}

	return 0
}

// IsNewerVersion returns true if latest is strictly greater than current
func IsNewerVersion(current, latest string) bool {
	return CompareVersions(latest, current) > 0
}

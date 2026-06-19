package imageprobe

import (
	"regexp"
	"strconv"
	"strings"
)

var semverRegex = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)

// Version holds a parsed major.minor.patch triple.
type Version struct {
	Major int
	Minor int
	Patch int
}

// ParseVersion extracts the first semver-like pattern from a string.
// Returns the parsed version and true if found, or zero-value and false
// if no version pattern was detected.
func ParseVersion(s string) (Version, bool) {
	match := semverRegex.FindStringSubmatch(s)
	if match == nil {
		return Version{}, false
	}

	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch := 0
	if match[3] != "" {
		patch, _ = strconv.Atoi(match[3])
	}

	return Version{Major: major, Minor: minor, Patch: patch}, true
}

// IsCompatible returns true if installed is compatible with required.
// Compatible means same major version and installed minor >= required minor.
// If either version string is empty or unparseable, the tool is assumed
// compatible (we cannot prove incompatibility).
func IsCompatible(installed, required string) bool {
	installed = strings.TrimSpace(installed)
	required = strings.TrimSpace(required)

	if installed == "" || required == "" {
		return true
	}

	iv, iok := ParseVersion(installed)
	rv, rok := ParseVersion(required)

	if !iok || !rok {
		return true
	}

	if iv.Major != rv.Major {
		return false
	}

	return iv.Minor >= rv.Minor
}

// String returns the version as "major.minor.patch".
func (v Version) String() string {
	return strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
}

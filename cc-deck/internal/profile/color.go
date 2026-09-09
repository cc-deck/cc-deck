package profile

import (
	"fmt"
	"hash/fnv"
)

// palette holds eight visually distinct colors for profile identification.
// Each entry is chosen for adequate contrast against dark terminal backgrounds.
var palette = [8]string{
	"#4FC1E9", // sky
	"#A0D468", // green
	"#ED5565", // red
	"#AC92EC", // violet
	"#FFCE54", // yellow
	"#48CFAD", // mint
	"#EC87C0", // pink
	"#F6BB42", // amber
}

// Derive returns a deterministic hex color for the given profile name.
// It hashes the name with FNV-1a (32-bit) and maps the result to one of
// eight palette entries.
func Derive(name string) string {
	h := fnv.New32a()
	// fnv hash.Write never returns an error, so we can safely ignore it.
	_, _ = h.Write([]byte(name))
	return palette[h.Sum32()%uint32(len(palette))]
}

// ParseHex converts a "#RRGGBB" hex color string into its red, green, and
// blue components. It returns an error when the input does not match the
// expected 7-character format.
func ParseHex(s string) (r, g, b uint8, err error) {
	if len(s) != 7 || s[0] != '#' {
		return 0, 0, 0, fmt.Errorf("invalid hex color %q: expected #RRGGBB", s)
	}
	var rgb [3]uint8
	for i := range 3 {
		hi, ok1 := hexVal(s[1+i*2])
		lo, ok2 := hexVal(s[2+i*2])
		if !ok1 || !ok2 {
			return 0, 0, 0, fmt.Errorf("invalid hex color %q: non-hex character", s)
		}
		rgb[i] = hi<<4 | lo
	}
	return rgb[0], rgb[1], rgb[2], nil
}

// hexVal returns the numeric value of a single hex digit and whether the
// character is valid hex.
func hexVal(c byte) (uint8, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

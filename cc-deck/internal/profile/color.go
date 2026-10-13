package profile

import (
	"fmt"
	"hash/fnv"
	"math"

	"github.com/cc-deck/cc-deck/internal/config"
)

// palette holds eight profile colors in hue order, spaced 45 degrees apart
// (HSL saturation 70 percent, lightness 62 percent; the blue entry uses
// lightness 72 percent for contrast). Every entry has a contrast ratio of at
// least 4.5:1 against black and against a typical dark terminal background.
var palette = [8]string{
	"#E27C5A", // 15, orange-red
	"#E2E25A", // 60, yellow
	"#7CE25A", // 105, green
	"#5AE29E", // 150, mint
	"#5AC0E2", // 195, sky
	"#8686EA", // 240, blue
	"#C05AE2", // 285, violet
	"#E25A9E", // 330, pink
}

// similarHue is the hue distance in degrees below which two chromatic
// colors count as similar.
const similarHue = 40.0

// achromaticSat is the HSL saturation below which a color counts as gray.
const achromaticSat = 0.15

// similarLightness is the lightness difference below which two gray colors
// count as similar.
const similarLightness = 0.15

// Derive returns a deterministic hex color for the given profile name.
// It hashes the name with FNV-1a (32-bit) and maps the result to one of
// eight palette entries.
func Derive(name string) string {
	h := fnv.New32a()
	// fnv hash.Write never returns an error, so we can safely ignore it.
	_, _ = h.Write([]byte(name))
	return palette[h.Sum32()%uint32(len(palette))]
}

// EffectiveColor returns the color a profile shows in the sidebar: its
// declared color when that is a valid #RRGGBB value, otherwise the color
// derived from its name (FR-017). Every place that needs a profile's color
// uses this function so that the wrapper export, the collision warning, the
// color picker and the hook agree.
func EffectiveColor(name string, p config.Profile) string {
	if _, _, _, err := ParseHex(p.Color); err == nil {
		return p.Color
	}
	return Derive(name)
}

// Pick returns the palette entry that maximizes the minimum hue distance to
// the used colors. Ties go to the lowest palette index, and an empty (or
// fully ignored) used set yields the first entry. Unparsable and achromatic
// used entries are ignored, because no palette entry can be similar to them.
func Pick(used []string) string {
	var hues []float64
	for _, c := range used {
		h, s, _, err := hsl(c)
		if err != nil || s < achromaticSat {
			continue
		}
		hues = append(hues, h)
	}
	if len(hues) == 0 {
		return palette[0]
	}

	best, bestDist := palette[0], -1.0
	for _, entry := range palette {
		h, _, _, _ := hsl(entry)
		minDist := 180.0
		for _, u := range hues {
			minDist = math.Min(minDist, hueDelta(h, u))
		}
		if minDist > bestDist {
			best, bestDist = entry, minDist
		}
	}
	return best
}

// HueDistance returns the circular hue distance between two "#RRGGBB"
// colors in degrees (0 to 180). Unparsable input yields 0.
func HueDistance(a, b string) float64 {
	ha, _, _, errA := hsl(a)
	hb, _, _, errB := hsl(b)
	if errA != nil || errB != nil {
		return 0
	}
	return hueDelta(ha, hb)
}

// Similar reports whether two "#RRGGBB" colors are hard to tell apart in the
// sidebar: chromatic colors closer than 40 degrees in hue, or two gray colors
// whose lightness differs by less than 0.15. A gray and a chromatic color are
// never similar. Unparsable input is never similar to anything.
func Similar(a, b string) bool {
	ha, sa, la, errA := hsl(a)
	hb, sb, lb, errB := hsl(b)
	if errA != nil || errB != nil {
		return false
	}
	grayA, grayB := sa < achromaticSat, sb < achromaticSat
	switch {
	case grayA && grayB:
		return math.Abs(la-lb) < similarLightness
	case grayA || grayB:
		return false
	default:
		return hueDelta(ha, hb) < similarHue
	}
}

// hueDelta returns the circular distance between two hues in degrees.
func hueDelta(a, b float64) float64 {
	d := math.Abs(a - b)
	if d > 180 {
		d = 360 - d
	}
	return d
}

// hsl converts a "#RRGGBB" color into hue (degrees, 0 to 360), saturation
// and lightness (both 0 to 1).
func hsl(hex string) (h, s, l float64, err error) {
	r8, g8, b8, err := ParseHex(hex)
	if err != nil {
		return 0, 0, 0, err
	}
	r, g, b := float64(r8)/255, float64(g8)/255, float64(b8)/255
	maxC := math.Max(r, math.Max(g, b))
	minC := math.Min(r, math.Min(g, b))
	l = (maxC + minC) / 2
	d := maxC - minC
	if d == 0 {
		return 0, 0, l, nil
	}
	if l > 0.5 {
		s = d / (2 - maxC - minC)
	} else {
		s = d / (maxC + minC)
	}
	switch maxC {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, s, l, nil
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

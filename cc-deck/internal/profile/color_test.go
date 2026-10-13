package profile

import (
	"math"
	"testing"

	"github.com/cc-deck/cc-deck/internal/config"
)

func TestDeriveDeterministic(t *testing.T) {
	// Calling Derive with the same name must always return the same color.
	name := "my-profile"
	first := Derive(name)
	for i := range 50 {
		got := Derive(name)
		if got != first {
			t.Fatalf("iteration %d: Derive(%q) = %q, want %q", i, name, got, first)
		}
	}
}

func TestDeriveReturnsPaletteMember(t *testing.T) {
	paletteSet := make(map[string]bool, len(palette))
	for _, c := range palette {
		paletteSet[c] = true
	}

	names := []string{"alice", "bob", "carol", "dave", "eve", "frank", "grace", "heidi", "z"}
	for _, name := range names {
		got := Derive(name)
		if !paletteSet[got] {
			t.Errorf("Derive(%q) = %q, not in palette", name, got)
		}
	}
}

// relativeLuminance computes the WCAG 2.x relative luminance with the sRGB
// transfer function.
func relativeLuminance(r, g, b uint8) float64 {
	lin := func(c uint8) float64 {
		v := float64(c) / 255.0
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// contrastRatio returns the WCAG contrast ratio between two luminance values.
func contrastRatio(l1, l2 float64) float64 {
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func TestPaletteContrast(t *testing.T) {
	// Every palette color must have contrast ratio >= 4.5:1 against black and
	// against a typical dark terminal background.
	backgrounds := []string{"#000000", "#1E1E2E"}

	for _, entry := range palette {
		r, g, b, err := ParseHex(entry)
		if err != nil {
			t.Fatalf("palette entry %q failed to parse: %v", entry, err)
		}
		fgLum := relativeLuminance(r, g, b)

		for _, bg := range backgrounds {
			br, bgc, bb, _ := ParseHex(bg)
			ratio := contrastRatio(fgLum, relativeLuminance(br, bgc, bb))
			if ratio < 4.5 {
				t.Errorf("palette %q vs %s: contrast ratio %.2f < 4.5", entry, bg, ratio)
			}
		}
	}
}

func TestPaletteHueSpacing(t *testing.T) {
	for i := range palette {
		for j := i + 1; j < len(palette); j++ {
			if d := HueDistance(palette[i], palette[j]); d < 40 {
				t.Errorf("palette %q and %q are %.1f degrees apart, want >= 40", palette[i], palette[j], d)
			}
		}
	}
}

func TestPickSequenceDistinct(t *testing.T) {
	// SC-011: eight sequential picks yield eight distinct, pairwise
	// non-similar colors.
	var used []string
	for range len(palette) {
		used = append(used, Pick(used))
	}
	seen := map[string]bool{}
	for _, c := range used {
		if seen[c] {
			t.Fatalf("Pick repeated %q in sequence %v", c, used)
		}
		seen[c] = true
	}
	for i := range used {
		for j := i + 1; j < len(used); j++ {
			if Similar(used[i], used[j]) {
				t.Errorf("picked colors %q and %q are similar", used[i], used[j])
			}
		}
	}
}

func TestPickEmptyAndIgnoredInput(t *testing.T) {
	if got := Pick(nil); got != palette[0] {
		t.Errorf("Pick(nil) = %q, want %q", got, palette[0])
	}
	if got := Pick([]string{"red", "#12345", "#808080"}); got != palette[0] {
		t.Errorf("Pick(unparsable and gray) = %q, want %q", got, palette[0])
	}
}

func TestPickAvoidsUsedNeighborhood(t *testing.T) {
	// A non-palette orange (hue about 20) must push the pick to the far side
	// of the hue circle.
	used := []string{"#E2805A"}
	got := Pick(used)
	if Similar(got, used[0]) {
		t.Errorf("Pick(%v) = %q, similar to the used color", used, got)
	}
	if d := HueDistance(got, used[0]); d < 150 {
		t.Errorf("Pick(%v) = %q, only %.1f degrees away", used, got, d)
	}
}

func TestSimilar(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"near hues", "#E27C5A", "#E2805A", true},
		{"far hues", "#E27C5A", "#5AC0E2", false},
		{"adjacent palette entries", "#E27C5A", "#E2E25A", false},
		{"two close grays", "#808080", "#8A8A8A", true},
		{"two distant grays", "#202020", "#E0E0E0", false},
		{"gray versus red", "#808080", "#E25A5A", false},
		{"unparsable", "red", "#E25A5A", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Similar(tc.a, tc.b); got != tc.want {
				t.Errorf("Similar(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestHueDistanceWraps(t *testing.T) {
	// Hue 350 (#E25A79-ish) and hue 10 are 20 degrees apart across 0.
	a := "#E25A71" // about 350
	b := "#E2705A" // about 10
	d := HueDistance(a, b)
	if d < 15 || d > 25 {
		t.Errorf("HueDistance(%q, %q) = %.1f, want about 20", a, b, d)
	}
	if got := HueDistance(palette[0], palette[4]); math.Abs(got-180) > 1 {
		t.Errorf("HueDistance of opposite entries = %.1f, want 180", got)
	}
}

func TestParseHexValid(t *testing.T) {
	tests := []struct {
		input   string
		r, g, b uint8
	}{
		{"#000000", 0, 0, 0},
		{"#FFFFFF", 255, 255, 255},
		{"#ffffff", 255, 255, 255},
		{"#4FC1E9", 0x4F, 0xC1, 0xE9},
		{"#abcdef", 0xAB, 0xCD, 0xEF},
	}
	for _, tc := range tests {
		r, g, b, err := ParseHex(tc.input)
		if err != nil {
			t.Errorf("ParseHex(%q) returned error: %v", tc.input, err)
			continue
		}
		if r != tc.r || g != tc.g || b != tc.b {
			t.Errorf("ParseHex(%q) = (%d,%d,%d), want (%d,%d,%d)",
				tc.input, r, g, b, tc.r, tc.g, tc.b)
		}
	}
}

func TestParseHexInvalid(t *testing.T) {
	invalid := []string{
		"",
		"#",
		"#FFF",
		"#GGGGGG",
		"4FC1E9",
		"#4FC1E9FF",
		"not-a-color",
	}
	for _, s := range invalid {
		_, _, _, err := ParseHex(s)
		if err == nil {
			t.Errorf("ParseHex(%q) should have returned an error", s)
		}
	}
}

func TestEffectiveColor(t *testing.T) {
	tests := []struct {
		name  string
		color string
		want  string
	}{
		{"declared", "#5AC0E2", "#5AC0E2"},
		{"empty", "", Derive("p")},
		{"invalid", "red", Derive("p")},
		{"short hex", "#123", Derive("p")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveColor("p", config.Profile{Color: tc.color}); got != tc.want {
				t.Errorf("EffectiveColor(color=%q) = %q, want %q", tc.color, got, tc.want)
			}
		})
	}
}

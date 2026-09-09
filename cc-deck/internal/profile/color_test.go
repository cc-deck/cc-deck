package profile

import (
	"math"
	"testing"
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

// relativeLuminance computes the WCAG relative luminance using the simplified
// gamma (exponent 2.2) formula: L = 0.2126*R' + 0.7152*G' + 0.0722*B'.
func relativeLuminance(r, g, b uint8) float64 {
	rr := math.Pow(float64(r)/255.0, 2.2)
	gg := math.Pow(float64(g)/255.0, 2.2)
	bb := math.Pow(float64(b)/255.0, 2.2)
	return 0.2126*rr + 0.7152*gg + 0.0722*bb
}

// contrastRatio returns the WCAG contrast ratio between two luminance values.
func contrastRatio(l1, l2 float64) float64 {
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func TestPaletteContrast(t *testing.T) {
	// Every palette color must have contrast ratio >= 3:1 against both
	// background colors used in the sidebar plugin.
	backgrounds := []struct {
		name    string
		r, g, b uint8
	}{
		{"dark-bg(25,45,55)", 25, 45, 55},
		{"black(0,0,0)", 0, 0, 0},
	}

	for _, entry := range palette {
		r, g, b, err := ParseHex(entry)
		if err != nil {
			t.Fatalf("palette entry %q failed to parse: %v", entry, err)
		}
		fgLum := relativeLuminance(r, g, b)

		for _, bg := range backgrounds {
			bgLum := relativeLuminance(bg.r, bg.g, bg.b)
			ratio := contrastRatio(fgLum, bgLum)
			if ratio < 3.0 {
				t.Errorf("palette %q vs %s: contrast ratio %.2f < 3.0", entry, bg.name, ratio)
			}
		}
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

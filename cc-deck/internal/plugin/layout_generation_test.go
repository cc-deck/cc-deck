package plugin

import (
	"strings"
	"testing"
)

func TestValidLayouts(t *testing.T) {
	got := ValidLayouts()
	want := []string{"minimal", "standard", "clean"}
	if len(got) != len(want) {
		t.Fatalf("ValidLayouts() = %v, want %v", got, want)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("ValidLayouts()[%d] = %q, want %q", i, got[i], v)
		}
	}
}

func TestLayoutFilename(t *testing.T) {
	tests := []struct {
		variant LayoutVariant
		want    string
	}{
		{LayoutMinimal, "cc-deck-minimal.kdl"},
		{LayoutClean, "cc-deck-clean.kdl"},
		{LayoutStandard, "cc-deck.kdl"},
		{LayoutVariant("unknown"), "cc-deck.kdl"},
	}
	for _, tt := range tests {
		t.Run(string(tt.variant), func(t *testing.T) {
			if got := LayoutFilename(tt.variant); got != tt.want {
				t.Errorf("LayoutFilename(%q) = %q, want %q", tt.variant, got, tt.want)
			}
		})
	}
}

func TestGenerateLayout(t *testing.T) {
	tests := []struct {
		name       string
		variant    LayoutVariant
		wantSubstr []string
		notWant    []string
	}{
		{
			name:       "minimal layout",
			variant:    LayoutMinimal,
			wantSubstr: []string{"minimal (sidebar + compact-bar)", "compact-bar", "mode \"sidebar\""},
			notWant:    []string{"tab-bar", "status-bar"},
		},
		{
			name:       "standard layout",
			variant:    LayoutStandard,
			wantSubstr: []string{"standard (sidebar + tab-bar top + status-bar bottom)", "tab-bar", "status-bar"},
		},
		{
			name:       "clean layout",
			variant:    LayoutClean,
			wantSubstr: []string{"clean (sidebar only, no bars)", "mode \"sidebar\""},
			notWant:    []string{"tab-bar", "status-bar", "compact-bar"},
		},
		{
			name:       "unknown variant defaults to minimal",
			variant:    LayoutVariant("bogus"),
			wantSubstr: []string{"minimal (sidebar + compact-bar)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateLayout("/plugins", tt.variant)
			for _, want := range tt.wantSubstr {
				if !strings.Contains(got, want) {
					t.Errorf("GenerateLayout(%q) missing %q; got:\n%s", tt.variant, want, got)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(got, notWant) {
					t.Errorf("GenerateLayout(%q) unexpectedly contains %q; got:\n%s", tt.variant, notWant, got)
				}
			}
			if !strings.Contains(got, "/plugins/cc_deck.wasm") {
				t.Errorf("GenerateLayout(%q) missing plugin path; got:\n%s", tt.variant, got)
			}
		})
	}
}

func TestHasInjection(t *testing.T) {
	block := InjectionStart + "\nsome content\n" + InjectionEnd
	if !HasInjection(block) {
		t.Error("should detect markers in block")
	}
	if HasInjection("no markers here") {
		t.Error("should not detect markers in unrelated content")
	}
	if HasInjection(InjectionStart) {
		t.Error("should require both markers")
	}
}

func TestRemoveInjection(t *testing.T) {
	content := "before\n" + InjectionStart + "\ninjected content\n" + InjectionEnd + "\nafter\n"
	got := RemoveInjection(content)
	if strings.Contains(got, InjectionStart) || strings.Contains(got, InjectionEnd) {
		t.Errorf("RemoveInjection() should strip markers, got: %q", got)
	}
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Errorf("RemoveInjection() should preserve surrounding content, got: %q", got)
	}
}

func TestRemoveInjection_NoMarkers(t *testing.T) {
	content := "plain content with no markers"
	if got := RemoveInjection(content); got != content {
		t.Errorf("RemoveInjection() with no markers should return content unchanged, got: %q", got)
	}
}

func TestRemoveInjection_MissingEndMarker(t *testing.T) {
	content := "before\n" + InjectionStart + "\nno end marker"
	if got := RemoveInjection(content); got != content {
		t.Errorf("RemoveInjection() with missing end marker should return content unchanged, got: %q", got)
	}
}

package ws

import (
	"strings"
	"testing"
)

func TestParseZellijSessionNames(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want []string
	}{
		{"empty", "", nil},
		{"blank lines", "\n  \n", nil},
		{"single", "cc-deck-demo\n", []string{"cc-deck-demo"}},
		{"name with trailing fields", "cc-deck-demo [Created 2h ago]\n", []string{"cc-deck-demo"}},
		{"skips exited", "old-session (EXITED - 1h ago)\ncc-deck-demo\n", []string{"cc-deck-demo"}},
		{"multiple live", "alpha\nbeta [Created]\n", []string{"alpha", "beta"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseZellijSessionNames(tc.out)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestSessionRenameHint(t *testing.T) {
	const canonical = "cc-deck-demo"

	if got := sessionRenameHint("demo", canonical, nil); got != "" {
		t.Errorf("no sessions should produce no hint, got %q", got)
	}
	if got := sessionRenameHint("demo", canonical, []string{canonical}); got != "" {
		t.Errorf("canonical session present should produce no hint, got %q", got)
	}

	got := sessionRenameHint("demo", canonical, []string{"old-anon"})
	if got == "" {
		t.Fatal("a mismatched session should produce a hint")
	}
	if !strings.Contains(got, "old-anon") {
		t.Errorf("hint should name the existing session, got %q", got)
	}
	if !strings.Contains(got, "kill-session demo") {
		t.Errorf("hint should include the kill-session recovery command, got %q", got)
	}

	// A canonical session alongside a stray one still needs no hint.
	if got := sessionRenameHint("demo", canonical, []string{"old-anon", canonical}); got != "" {
		t.Errorf("canonical present among others should produce no hint, got %q", got)
	}
}

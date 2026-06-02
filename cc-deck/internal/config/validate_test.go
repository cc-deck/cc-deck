package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

// Helper to create a pointer to an int.
func intPtr(v int) *int { return &v }

// Helper to create a pointer to a float64.
func float64Ptr(v float64) *float64 { return &v }

// findFinding searches findings for one matching the given severity and message substring.
func findFinding(findings []Finding, sev Severity, msgSubstr string) *Finding {
	for i := range findings {
		if findings[i].Severity == sev && strings.Contains(findings[i].Message, msgSubstr) {
			return &findings[i]
		}
	}
	return nil
}

// countBySeverity counts findings of a given severity.
func countBySeverity(findings []Finding, sev Severity) int {
	n := 0
	for _, f := range findings {
		if f.Severity == sev {
			n++
		}
	}
	return n
}

// --- Icon Width Classification Tests ---

func TestIsEmoji(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want bool
	}{
		{"ship emoji U+1F6A2", 0x1F6A2, true},
		{"rocket emoji U+1F680", 0x1F680, true},
		{"sun U+2600", 0x2600, true},
		{"snowman U+2603", 0x2603, true},
		{"scissors U+2702", 0x2702, true},
		{"narrow vertical ellipsis U+22EE", 0x22EE, false},
		{"narrow latin A", 'A', false},
		{"narrow digit 1", '1', false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmoji(tt.r); got != tt.want {
				t.Errorf("isEmoji(U+%04X) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

func TestIsEastAsianWide(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want bool
	}{
		{"trigram for heaven U+2630", 0x2630, true},
		{"CJK ideograph U+4E00", 0x4E00, true},
		{"fullwidth exclamation U+FF01", 0xFF01, true},
		{"katakana A U+30A2", 0x30A2, true},
		{"narrow vertical ellipsis U+22EE", 0x22EE, false},
		{"narrow latin A", 'A', false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEastAsianWide(tt.r); got != tt.want {
				t.Errorf("isEastAsianWide(U+%04X) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

func TestIsEastAsianAmbiguous(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want bool
	}{
		{"black right-pointing triangle U+25B6", 0x25B6, true},
		{"black circle U+25CF", 0x25CF, true},
		{"left arrow U+2190", 0x2190, true},
		{"narrow vertical ellipsis U+22EE", 0x22EE, false},
		{"narrow latin A", 'A', false},
		{"narrow digit 1", '1', false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEastAsianAmbiguous(tt.r); got != tt.want {
				t.Errorf("isEastAsianAmbiguous(U+%04X) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

// --- Badge Value Parsing Tests ---

func TestParseBadgeValue(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantColor string
		wantIcon  string
		wantOk    bool
	}{
		{"plain icon", "⋮", "", "⋮", true},
		{"color prefix and icon", "#FFB43C:⋮", "#FFB43C:", "⋮", true},
		{"color prefix uppercase", "#AABBCC:X", "#AABBCC:", "X", true},
		{"no prefix, just text", "running", "", "running", true},
		{"empty string", "", "", "", true},
		{"hash but no colon", "#FFB43C", "", "#FFB43C", true},
		{"short hex color", "#FFB4:X", "", "", false},
		{"invalid hex chars", "#GGHHII:X", "", "", false},
		{"long hex color", "#FFB43CDD:X", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			color, icon, ok := parseBadgeValue(tt.value)
			if ok != tt.wantOk {
				t.Errorf("parseBadgeValue(%q) ok = %v, want %v", tt.value, ok, tt.wantOk)
				return
			}
			if !ok {
				return
			}
			if color != tt.wantColor {
				t.Errorf("parseBadgeValue(%q) color = %q, want %q", tt.value, color, tt.wantColor)
			}
			if icon != tt.wantIcon {
				t.Errorf("parseBadgeValue(%q) icon = %q, want %q", tt.value, icon, tt.wantIcon)
			}
		})
	}
}

// --- Badge Validation Tests ---

func TestValidateBadges_IconWidth(t *testing.T) {
	tests := []struct {
		name         string
		values       map[string]string
		wantSeverity Severity
		wantCount    int
		wantMsg      string
	}{
		{
			name:         "emoji icon is error",
			values:       map[string]string{"running": "\U0001F6A2"},
			wantSeverity: SeverityError,
			wantCount:    1,
			wantMsg:      "emoji",
		},
		{
			name:         "East Asian Wide icon is error",
			values:       map[string]string{"active": "☰"},
			wantSeverity: SeverityError,
			wantCount:    1,
			wantMsg:      "East Asian Width W",
		},
		{
			name:         "East Asian Ambiguous icon is warning",
			values:       map[string]string{"paused": "▶"},
			wantSeverity: SeverityWarning,
			wantCount:    1,
			wantMsg:      "Ambiguous",
		},
		{
			name:         "narrow icon is clean",
			values:       map[string]string{"ok": "⋮"},
			wantSeverity: "",
			wantCount:    0,
		},
		{
			name:         "color prefixed emoji",
			values:       map[string]string{"status": "#FFB43C:\U0001F6A2"},
			wantSeverity: SeverityError,
			wantCount:    1,
			wantMsg:      "emoji",
		},
		{
			name:         "empty value is skipped",
			values:       map[string]string{"empty": ""},
			wantSeverity: "",
			wantCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			badges := []BadgeRule{{
				Name:    "test",
				File:    "/tmp/test.json",
				Extract: ".status",
				Format:  "json",
				Values:  tt.values,
			}}
			findings := validateBadges(badges)
			if len(findings) != tt.wantCount {
				t.Errorf("got %d findings, want %d: %+v", len(findings), tt.wantCount, findings)
				return
			}
			if tt.wantCount > 0 {
				f := findings[0]
				if f.Severity != tt.wantSeverity {
					t.Errorf("severity = %s, want %s", f.Severity, tt.wantSeverity)
				}
				if !strings.Contains(f.Message, tt.wantMsg) {
					t.Errorf("message %q does not contain %q", f.Message, tt.wantMsg)
				}
				if f.Suggestion == "" {
					t.Errorf("expected non-empty suggestion")
				}
			}
		})
	}
}

func TestValidateBadges_Structure(t *testing.T) {
	t.Run("missing name", func(t *testing.T) {
		badges := []BadgeRule{{Name: "", File: "f", Extract: "e", Format: "json"}}
		findings := validateBadges(badges)
		if f := findFinding(findings, SeverityError, "name is required"); f == nil {
			t.Error("expected error for missing name")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		badges := []BadgeRule{{Name: "n", File: "", Extract: "e", Format: "json"}}
		findings := validateBadges(badges)
		if f := findFinding(findings, SeverityError, "file is required"); f == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("missing extract", func(t *testing.T) {
		badges := []BadgeRule{{Name: "n", File: "f", Extract: "", Format: "json"}}
		findings := validateBadges(badges)
		if f := findFinding(findings, SeverityError, "extract is required"); f == nil {
			t.Error("expected error for missing extract")
		}
	})

	t.Run("unsupported format", func(t *testing.T) {
		badges := []BadgeRule{{Name: "n", File: "f", Extract: "e", Format: "xml"}}
		findings := validateBadges(badges)
		if f := findFinding(findings, SeverityError, "unsupported format"); f == nil {
			t.Error("expected error for unsupported format")
		}
	})

	t.Run("valid format json", func(t *testing.T) {
		badges := []BadgeRule{{Name: "n", File: "f", Extract: "e", Format: "json"}}
		findings := validateBadges(badges)
		if f := findFinding(findings, SeverityError, "unsupported format"); f != nil {
			t.Error("unexpected error for valid format json")
		}
	})
}

func TestValidateBadges_ColorPrefix(t *testing.T) {
	t.Run("valid color prefix", func(t *testing.T) {
		badges := []BadgeRule{{
			Name: "test", File: "f", Extract: "e", Format: "json",
			Values: map[string]string{"ok": "#FFB43C:⋮"},
		}}
		findings := validateBadges(badges)
		if f := findFinding(findings, SeverityError, "malformed color"); f != nil {
			t.Errorf("unexpected color error: %+v", f)
		}
	})

	t.Run("invalid color prefix - short hex", func(t *testing.T) {
		badges := []BadgeRule{{
			Name: "test", File: "f", Extract: "e", Format: "json",
			Values: map[string]string{"bad": "#FFB4:⋮"},
		}}
		findings := validateBadges(badges)
		if f := findFinding(findings, SeverityError, "malformed color"); f == nil {
			t.Error("expected error for short hex color")
		}
	})

	t.Run("invalid color prefix - bad hex chars", func(t *testing.T) {
		badges := []BadgeRule{{
			Name: "test", File: "f", Extract: "e", Format: "json",
			Values: map[string]string{"bad": "#GGHHII:⋮"},
		}}
		findings := validateBadges(badges)
		if f := findFinding(findings, SeverityError, "malformed color"); f == nil {
			t.Error("expected error for invalid hex chars")
		}
	})
}

// --- Profile Validation Tests ---

func TestValidateProfiles(t *testing.T) {
	t.Run("anthropic missing api_key_secret", func(t *testing.T) {
		profiles := map[string]Profile{
			"bad": {Backend: BackendAnthropic},
		}
		findings := validateProfiles(profiles, "")
		if f := findFinding(findings, SeverityError, "api_key_secret"); f == nil {
			t.Error("expected error for missing api_key_secret")
		}
	})

	t.Run("vertex missing project", func(t *testing.T) {
		profiles := map[string]Profile{
			"bad": {Backend: BackendVertex, Region: "us-central1"},
		}
		findings := validateProfiles(profiles, "")
		if f := findFinding(findings, SeverityError, "project"); f == nil {
			t.Error("expected error for missing project")
		}
	})

	t.Run("vertex missing region", func(t *testing.T) {
		profiles := map[string]Profile{
			"bad": {Backend: BackendVertex, Project: "my-project"},
		}
		findings := validateProfiles(profiles, "")
		if f := findFinding(findings, SeverityError, "region"); f == nil {
			t.Error("expected error for missing region")
		}
	})

	t.Run("dangling default_profile", func(t *testing.T) {
		profiles := map[string]Profile{
			"dev": {Backend: BackendAnthropic, APIKeySecret: "secret"},
		}
		findings := validateProfiles(profiles, "nonexistent")
		if f := findFinding(findings, SeverityError, "does not match"); f == nil {
			t.Error("expected error for dangling default_profile")
		}
	})

	t.Run("valid default_profile reference", func(t *testing.T) {
		profiles := map[string]Profile{
			"dev": {Backend: BackendAnthropic, APIKeySecret: "secret"},
		}
		findings := validateProfiles(profiles, "dev")
		if f := findFinding(findings, SeverityError, "does not match"); f != nil {
			t.Error("unexpected error for valid default_profile")
		}
	})

	t.Run("empty default_profile with no profiles", func(t *testing.T) {
		findings := validateProfiles(nil, "")
		if len(findings) != 0 {
			t.Errorf("expected no findings, got %d", len(findings))
		}
	})

	t.Run("valid anthropic profile", func(t *testing.T) {
		profiles := map[string]Profile{
			"prod": {Backend: BackendAnthropic, APIKeySecret: "my-secret"},
		}
		findings := validateProfiles(profiles, "prod")
		if len(findings) != 0 {
			t.Errorf("expected no findings, got %d: %+v", len(findings), findings)
		}
	})
}

// --- Voice Validation Tests ---

func TestValidateVoice(t *testing.T) {
	t.Run("threshold out of range high", func(t *testing.T) {
		voice := VoiceDefaults{Threshold: intPtr(150)}
		findings := validateVoice(voice)
		if f := findFinding(findings, SeverityError, "out of range"); f == nil {
			t.Error("expected error for threshold 150")
		}
	})

	t.Run("threshold out of range negative", func(t *testing.T) {
		voice := VoiceDefaults{Threshold: intPtr(-1)}
		findings := validateVoice(voice)
		if f := findFinding(findings, SeverityError, "out of range"); f == nil {
			t.Error("expected error for threshold -1")
		}
	})

	t.Run("threshold valid", func(t *testing.T) {
		voice := VoiceDefaults{Threshold: intPtr(45)}
		findings := validateVoice(voice)
		if len(findings) != 0 {
			t.Errorf("expected no findings, got %d", len(findings))
		}
	})

	t.Run("silence negative", func(t *testing.T) {
		voice := VoiceDefaults{Silence: float64Ptr(-1)}
		findings := validateVoice(voice)
		if f := findFinding(findings, SeverityError, "negative"); f == nil {
			t.Error("expected error for negative silence")
		}
	})

	t.Run("silence extreme", func(t *testing.T) {
		voice := VoiceDefaults{Silence: float64Ptr(15)}
		findings := validateVoice(voice)
		if f := findFinding(findings, SeverityWarning, "unusually high"); f == nil {
			t.Error("expected warning for extreme silence")
		}
	})

	t.Run("silence valid", func(t *testing.T) {
		voice := VoiceDefaults{Silence: float64Ptr(2.0)}
		findings := validateVoice(voice)
		if len(findings) != 0 {
			t.Errorf("expected no findings, got %d", len(findings))
		}
	})

	t.Run("pre_roll negative", func(t *testing.T) {
		voice := VoiceDefaults{PreRoll: float64Ptr(-0.5)}
		findings := validateVoice(voice)
		if f := findFinding(findings, SeverityError, "negative"); f == nil {
			t.Error("expected error for negative pre_roll")
		}
	})

	t.Run("pre_roll extreme", func(t *testing.T) {
		voice := VoiceDefaults{PreRoll: float64Ptr(3.0)}
		findings := validateVoice(voice)
		if f := findFinding(findings, SeverityWarning, "unusually high"); f == nil {
			t.Error("expected warning for extreme pre_roll")
		}
	})

	t.Run("hangover negative", func(t *testing.T) {
		voice := VoiceDefaults{Hangover: float64Ptr(-1)}
		findings := validateVoice(voice)
		if f := findFinding(findings, SeverityError, "negative"); f == nil {
			t.Error("expected error for negative hangover")
		}
	})

	t.Run("hangover extreme", func(t *testing.T) {
		voice := VoiceDefaults{Hangover: float64Ptr(6.0)}
		findings := validateVoice(voice)
		if f := findFinding(findings, SeverityWarning, "unusually high"); f == nil {
			t.Error("expected warning for extreme hangover")
		}
	})

	t.Run("hangover valid", func(t *testing.T) {
		voice := VoiceDefaults{Hangover: float64Ptr(0.3)}
		findings := validateVoice(voice)
		if len(findings) != 0 {
			t.Errorf("expected no findings, got %d", len(findings))
		}
	})

	t.Run("all nil is valid", func(t *testing.T) {
		voice := VoiceDefaults{}
		findings := validateVoice(voice)
		if len(findings) != 0 {
			t.Errorf("expected no findings, got %d", len(findings))
		}
	})
}

// --- Config.Validate() Integration Tests ---

func TestConfigValidate_Clean(t *testing.T) {
	cfg := &Config{}
	findings := cfg.Validate()
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty config, got %d", len(findings))
	}
}

func TestConfigValidate_MultipleCategories(t *testing.T) {
	cfg := &Config{
		Badges: []BadgeRule{
			{Name: "", File: "f", Extract: "e", Format: "json"}, // structure finding
			{
				Name: "test", File: "f", Extract: "e", Format: "json",
				Values: map[string]string{"wide": "☰"}, // badge icon width finding
			},
		},
		Profiles: map[string]Profile{
			"bad": {Backend: BackendAnthropic},
		},
		Defaults: Defaults{
			Voice: VoiceDefaults{Threshold: intPtr(200)},
		},
	}
	findings := cfg.Validate()
	if len(findings) < 4 {
		t.Errorf("expected at least 4 findings across categories, got %d", len(findings))
	}

	// Check we have findings from each category
	categories := map[Category]bool{}
	for _, f := range findings {
		categories[f.Category] = true
	}
	for _, cat := range []Category{CategoryBadges, CategoryProfiles, CategoryVoice, CategoryStructure} {
		if !categories[cat] {
			t.Errorf("expected finding in category %s", cat)
		}
	}
}

// --- ValidateAndWarn Tests ---

func TestValidateAndWarn_NoIssues(t *testing.T) {
	// Redirect stderr to capture output
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	cfg := &Config{}
	findings := cfg.ValidateAndWarn()

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stderr = oldStderr

	if len(findings) != 0 {
		t.Errorf("expected nil findings, got %d", len(findings))
	}
	if buf.Len() != 0 {
		t.Errorf("expected no stderr output, got %q", buf.String())
	}
}

func TestValidateAndWarn_WithIssues(t *testing.T) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	cfg := &Config{
		Badges: []BadgeRule{
			{
				Name: "test", File: "f", Extract: "e", Format: "json",
				Values: map[string]string{
					"wide": "☰",  // error
					"amb":  "▶",  // warning
				},
			},
			{Name: "", File: "f", Extract: "e", Format: "json"}, // error (missing name)
		},
	}
	findings := cfg.ValidateAndWarn()

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stderr = oldStderr

	if len(findings) == 0 {
		t.Error("expected findings")
	}

	output := buf.String()
	if !strings.Contains(output, "config:") {
		t.Errorf("expected config: prefix in stderr, got %q", output)
	}
	if !strings.Contains(output, "cc-deck config check") {
		t.Errorf("expected 'cc-deck config check' reference in stderr, got %q", output)
	}

	errors := countBySeverity(findings, SeverityError)
	warnings := countBySeverity(findings, SeverityWarning)

	if errors > 0 && !strings.Contains(output, fmt.Sprintf("%d error", errors)) {
		t.Errorf("expected error count in stderr, got %q", output)
	}
	if warnings > 0 && !strings.Contains(output, fmt.Sprintf("%d warning", warnings)) {
		t.Errorf("expected warning count in stderr, got %q", output)
	}
}

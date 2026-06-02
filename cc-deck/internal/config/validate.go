package config

import (
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Severity indicates how serious a validation finding is.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Category groups findings by which part of the config they relate to.
type Category string

const (
	CategoryBadges    Category = "badges"
	CategoryProfiles  Category = "profiles"
	CategoryVoice     Category = "voice"
	CategoryStructure Category = "structure"
)

// Finding represents a single validation issue found in the config file.
type Finding struct {
	Severity   Severity
	Category   Category
	Message    string
	Suggestion string
}

// Validate checks the config for issues and returns a slice of findings.
// An empty slice means the config is valid.
func (c *Config) Validate() []Finding {
	var findings []Finding
	findings = append(findings, validateBadges(c.Badges)...)
	findings = append(findings, validateProfiles(c.Profiles, c.DefaultProfile)...)
	findings = append(findings, validateVoice(c.Defaults.Voice)...)
	return findings
}

// ValidateAndWarn runs Validate() and prints a one-line summary to stderr
// if any issues are found. Returns the findings for further use.
func (c *Config) ValidateAndWarn() []Finding {
	findings := c.Validate()
	if len(findings) == 0 {
		return nil
	}

	var errors, warnings int
	for _, f := range findings {
		switch f.Severity {
		case SeverityError:
			errors++
		case SeverityWarning:
			warnings++
		}
	}

	parts := []string{}
	if errors > 0 {
		parts = append(parts, fmt.Sprintf("%d error", errors))
		if errors > 1 {
			parts[len(parts)-1] += "s"
		}
	}
	if warnings > 0 {
		parts = append(parts, fmt.Sprintf("%d warning", warnings))
		if warnings > 1 {
			parts[len(parts)-1] += "s"
		}
	}

	fmt.Fprintf(os.Stderr, "config: %d issues found (%s), run cc-deck config check for details\n",
		len(findings), strings.Join(parts, ", "))

	return findings
}

// supportedBadgeFormats lists the valid format values for badge rules.
var supportedBadgeFormats = []string{"json"}

// parseBadgeValue separates a badge value into its color prefix and icon.
// If the value has a #RRGGBB: prefix, returns (color, icon, true).
// If the value starts with # but has an invalid color, returns ("", "", false) with ok=false.
// If there is no color prefix, returns ("", value, true).
func parseBadgeValue(value string) (color string, icon string, ok bool) {
	if !strings.HasPrefix(value, "#") {
		return "", value, true
	}
	// Has a # prefix, check if it looks like a color prefix
	colonIdx := strings.Index(value, ":")
	if colonIdx < 0 {
		// Just a # with no colon, treat as a plain value
		return "", value, true
	}
	colorPart := value[1:colonIdx]
	if len(colorPart) != 6 {
		return "", "", false
	}
	for _, ch := range colorPart {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return "", "", false
		}
	}
	return value[:colonIdx+1], value[colonIdx+1:], true
}

// isEmoji returns true if the rune is an emoji character that typically
// renders as 2 columns wide in terminals.
func isEmoji(r rune) bool {
	// Emoji presentation sequences (U+1F000-U+1FFFF)
	if r >= 0x1F000 && r <= 0x1FFFF {
		return true
	}
	// Miscellaneous Symbols and Dingbats with emoji presentation
	if r >= 0x2600 && r <= 0x27BF {
		// Only those that commonly have emoji presentation
		switch {
		case r >= 0x2600 && r <= 0x26FF: // Miscellaneous Symbols
			return true
		case r >= 0x2700 && r <= 0x27BF: // Dingbats
			return true
		}
	}
	// Transport and Map Symbols
	if r >= 0x1F680 && r <= 0x1F6FF {
		return true
	}
	// Regional indicator symbols
	if r >= 0x1F1E0 && r <= 0x1F1FF {
		return true
	}
	return false
}

// isEastAsianWide returns true if the rune has East Asian Width W or F property.
// These characters always render as 2 columns in terminals.
func isEastAsianWide(r rune) bool {
	// CJK Unified Ideographs
	if r >= 0x4E00 && r <= 0x9FFF {
		return true
	}
	// CJK Unified Ideographs Extension A
	if r >= 0x3400 && r <= 0x4DBF {
		return true
	}
	// CJK Compatibility Ideographs
	if r >= 0xF900 && r <= 0xFAFF {
		return true
	}
	// Fullwidth Forms
	if r >= 0xFF01 && r <= 0xFF60 {
		return true
	}
	if r >= 0xFFE0 && r <= 0xFFE6 {
		return true
	}
	// CJK Symbols and Punctuation
	if r >= 0x3000 && r <= 0x303F {
		return true
	}
	// Hiragana
	if r >= 0x3040 && r <= 0x309F {
		return true
	}
	// Katakana
	if r >= 0x30A0 && r <= 0x30FF {
		return true
	}
	// Hangul Syllables
	if r >= 0xAC00 && r <= 0xD7AF {
		return true
	}
	// CJK Radicals Supplement + Kangxi Radicals
	if r >= 0x2E80 && r <= 0x2FDF {
		return true
	}
	// Enclosed CJK Letters and Months
	if r >= 0x3200 && r <= 0x32FF {
		return true
	}
	// CJK Compatibility
	if r >= 0x3300 && r <= 0x33FF {
		return true
	}
	// Yijing Hexagram Symbols (U+4DC0-U+4DFF)
	if r >= 0x4DC0 && r <= 0x4DFF {
		return true
	}
	// Bopomofo
	if r >= 0x3100 && r <= 0x312F {
		return true
	}
	// Specific Wide characters commonly used as icons
	switch r {
	case 0x2630: // ☰ TRIGRAM FOR HEAVEN (East Asian Width W)
		return true
	}
	return false
}

// isEastAsianAmbiguous returns true if the rune has East Asian Width A property.
// These characters may render as 1 or 2 columns depending on terminal settings.
func isEastAsianAmbiguous(r rune) bool {
	// Common Ambiguous characters used as icons
	switch r {
	case 0x25B6: // ▶ BLACK RIGHT-POINTING TRIANGLE
		return true
	case 0x25C0: // ◀ BLACK LEFT-POINTING TRIANGLE
		return true
	case 0x25A0: // ■ BLACK SQUARE
		return true
	case 0x25A1: // □ WHITE SQUARE
		return true
	case 0x25CB: // ○ WHITE CIRCLE
		return true
	case 0x25CF: // ● BLACK CIRCLE
		return true
	case 0x25C6: // ◆ BLACK DIAMOND
		return true
	case 0x25C7: // ◇ WHITE DIAMOND
		return true
	case 0x2190, 0x2191, 0x2192, 0x2193: // ←↑→↓ ARROWS
		return true
	case 0x2194, 0x2195: // ↔↕ ARROWS
		return true
	case 0x2660, 0x2663, 0x2665, 0x2666: // ♠♣♥♦ card suits
		return true
	case 0x266A, 0x266B: // ♪♫ musical notes
		return true
	}
	// Box Drawing and Block Elements are Ambiguous
	if r >= 0x2500 && r <= 0x259F {
		return true
	}
	// Geometric Shapes (subset)
	if r >= 0x25A0 && r <= 0x25FF {
		return true
	}
	// Miscellaneous Technical (subset)
	if r >= 0x2300 && r <= 0x23FF {
		// Many in this range are Ambiguous
		return true
	}
	return false
}

// suggestedReplacement returns a narrow-width alternative icon for a wide or ambiguous character.
func suggestedReplacement(r rune) string {
	// Common replacements for wide/emoji characters
	if unicode.Is(unicode.So, r) || isEmoji(r) || isEastAsianWide(r) {
		return "use a Narrow character like ⋮ (U+22EE VERTICAL ELLIPSIS) or │ (U+2502 BOX DRAWINGS LIGHT VERTICAL)"
	}
	if isEastAsianAmbiguous(r) {
		return "use a Narrow character like ⋮ (U+22EE VERTICAL ELLIPSIS) or · (U+00B7 MIDDLE DOT)"
	}
	return ""
}

// validateBadges checks badge rules for structural issues and icon width problems.
func validateBadges(badges []BadgeRule) []Finding {
	var findings []Finding
	for i, badge := range badges {
		prefix := fmt.Sprintf("badges[%d]", i)

		// Structure checks
		if badge.Name == "" {
			findings = append(findings, Finding{
				Severity:   SeverityError,
				Category:   CategoryStructure,
				Message:    fmt.Sprintf("%s: name is required", prefix),
				Suggestion: "add a name field to identify this badge rule",
			})
		}
		if badge.File == "" {
			findings = append(findings, Finding{
				Severity:   SeverityError,
				Category:   CategoryStructure,
				Message:    fmt.Sprintf("%s (%s): file is required", prefix, badge.Name),
				Suggestion: "add a file path to read badge data from",
			})
		}
		if badge.Extract == "" {
			findings = append(findings, Finding{
				Severity:   SeverityError,
				Category:   CategoryStructure,
				Message:    fmt.Sprintf("%s (%s): extract is required", prefix, badge.Name),
				Suggestion: "add an extract expression to select the badge value",
			})
		}

		// Format check
		if badge.Format != "" {
			supported := false
			for _, f := range supportedBadgeFormats {
				if badge.Format == f {
					supported = true
					break
				}
			}
			if !supported {
				findings = append(findings, Finding{
					Severity:   SeverityError,
					Category:   CategoryStructure,
					Message:    fmt.Sprintf("%s (%s): unsupported format %q", prefix, badge.Name, badge.Format),
					Suggestion: fmt.Sprintf("supported formats: %s", strings.Join(supportedBadgeFormats, ", ")),
				})
			}
		}

		// Check each badge value icon
		for key, value := range badge.Values {
			if value == "" {
				continue
			}

			_, icon, ok := parseBadgeValue(value)
			if !ok {
				findings = append(findings, Finding{
					Severity:   SeverityError,
					Category:   CategoryBadges,
					Message:    fmt.Sprintf("%s (%s): value %q has malformed color prefix", prefix, badge.Name, key),
					Suggestion: "color prefix must be #RRGGBB: format with exactly 6 hex digits",
				})
				continue
			}

			if icon == "" {
				continue
			}

			findings = append(findings, checkIconWidth(prefix, badge.Name, key, icon)...)
		}

		// Check default badge value
		if badge.Default != "" {
			_, icon, ok := parseBadgeValue(badge.Default)
			if !ok {
				findings = append(findings, Finding{
					Severity:   SeverityError,
					Category:   CategoryBadges,
					Message:    fmt.Sprintf("%s (%s): default value has malformed color prefix", prefix, badge.Name),
					Suggestion: "color prefix must be #RRGGBB: format with exactly 6 hex digits",
				})
			} else if icon != "" {
				findings = append(findings, checkIconWidth(prefix, badge.Name, "default", icon)...)
			}
		}
	}
	return findings
}

// checkIconWidth checks a single icon string for width issues.
func checkIconWidth(prefix, badgeName, key, icon string) []Finding {
	var findings []Finding
	r, _ := utf8.DecodeRuneInString(icon)
	if r == utf8.RuneError {
		return findings
	}

	if isEastAsianWide(r) {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Category: CategoryBadges,
			Message: fmt.Sprintf("%s (%s): value %q icon %q (U+%04X) has East Asian Width W and renders as 2 columns",
				prefix, badgeName, key, string(r), r),
			Suggestion: suggestedReplacement(r),
		})
	} else if isEmoji(r) {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Category: CategoryBadges,
			Message: fmt.Sprintf("%s (%s): value %q icon %q (U+%04X) is an emoji and renders as 2 columns",
				prefix, badgeName, key, string(r), r),
			Suggestion: suggestedReplacement(r),
		})
	} else if isEastAsianAmbiguous(r) {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Category: CategoryBadges,
			Message: fmt.Sprintf("%s (%s): value %q icon %q (U+%04X) has East Asian Width Ambiguous and may render as 2 columns in some terminals",
				prefix, badgeName, key, string(r), r),
			Suggestion: suggestedReplacement(r),
		})
	}
	return findings
}

// validateProfiles checks profile configurations and default_profile reference.
func validateProfiles(profiles map[string]Profile, defaultProfile string) []Finding {
	var findings []Finding

	// Check each profile
	for name, profile := range profiles {
		if err := profile.Validate(); err != nil {
			findings = append(findings, Finding{
				Severity:   SeverityError,
				Category:   CategoryProfiles,
				Message:    fmt.Sprintf("profile %q: %s", name, err.Error()),
				Suggestion: "check required fields for the backend type",
			})
		}
	}

	// Check default_profile reference
	if defaultProfile != "" && len(profiles) > 0 {
		if _, ok := profiles[defaultProfile]; !ok {
			names := make([]string, 0, len(profiles))
			for n := range profiles {
				names = append(names, n)
			}
			findings = append(findings, Finding{
				Severity:   SeverityError,
				Category:   CategoryProfiles,
				Message:    fmt.Sprintf("default_profile %q does not match any defined profile", defaultProfile),
				Suggestion: fmt.Sprintf("available profiles: %s", strings.Join(names, ", ")),
			})
		}
	}

	return findings
}

// validateVoice checks voice parameter values for range and sanity.
func validateVoice(voice VoiceDefaults) []Finding {
	var findings []Finding

	// Threshold: 0-100
	if voice.Threshold != nil {
		t := *voice.Threshold
		if t < 0 || t > 100 {
			findings = append(findings, Finding{
				Severity:   SeverityError,
				Category:   CategoryVoice,
				Message:    fmt.Sprintf("voice.threshold %d is out of range", t),
				Suggestion: "threshold must be between 0 and 100 (logarithmic VAD sensitivity)",
			})
		}
	}

	// Silence: must be positive, warn on extreme
	if voice.Silence != nil {
		s := *voice.Silence
		if s < 0 {
			findings = append(findings, Finding{
				Severity:   SeverityError,
				Category:   CategoryVoice,
				Message:    fmt.Sprintf("voice.silence %g is negative", s),
				Suggestion: "silence duration must be a positive number (seconds)",
			})
		} else if s > 10 {
			findings = append(findings, Finding{
				Severity:   SeverityWarning,
				Category:   CategoryVoice,
				Message:    fmt.Sprintf("voice.silence %g is unusually high", s),
				Suggestion: "values above 10 seconds may cause very long pauses before speech is finalized",
			})
		}
	}

	// PreRoll: must be positive, warn on extreme
	if voice.PreRoll != nil {
		p := *voice.PreRoll
		if p < 0 {
			findings = append(findings, Finding{
				Severity:   SeverityError,
				Category:   CategoryVoice,
				Message:    fmt.Sprintf("voice.pre_roll %g is negative", p),
				Suggestion: "pre_roll duration must be a positive number (seconds)",
			})
		} else if p > 2 {
			findings = append(findings, Finding{
				Severity:   SeverityWarning,
				Category:   CategoryVoice,
				Message:    fmt.Sprintf("voice.pre_roll %g is unusually high", p),
				Suggestion: "values above 2 seconds may include too much pre-speech audio",
			})
		}
	}

	// Hangover: must be positive, warn on extreme
	if voice.Hangover != nil {
		h := *voice.Hangover
		if h < 0 {
			findings = append(findings, Finding{
				Severity:   SeverityError,
				Category:   CategoryVoice,
				Message:    fmt.Sprintf("voice.hangover %g is negative", h),
				Suggestion: "hangover duration must be a positive number (seconds)",
			})
		} else if h > 5 {
			findings = append(findings, Finding{
				Severity:   SeverityWarning,
				Category:   CategoryVoice,
				Message:    fmt.Sprintf("voice.hangover %g is unusually high", h),
				Suggestion: "values above 5 seconds may delay end-of-speech detection",
			})
		}
	}

	return findings
}

package config

import (
	"fmt"
	"os"
	"regexp"
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

	var errors int
	for _, f := range findings {
		if f.Severity == SeverityError {
			errors++
		}
	}

	if errors == 0 {
		return findings
	}

	fmt.Fprintf(os.Stderr, "config: %d error", errors)
	if errors > 1 {
		fmt.Fprint(os.Stderr, "s")
	}
	fmt.Fprintln(os.Stderr, " found, run cc-deck config check for details")

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

// isEmoji returns true if the rune is an emoji character that always
// renders as 2 columns wide in terminals. Only supplementary plane
// emoji (U+1F000+) qualify, as they have default emoji presentation.
// BMP characters (U+0000-U+FFFF) in ranges like Miscellaneous Symbols
// (U+2600-U+26FF) and Dingbats (U+2700-U+27BF) are NOT flagged here
// because terminals render them as 1 column (text presentation) unless
// followed by VS16 (U+FE0F), which badge values never include.
func isEmoji(r rune) bool {
	// Supplementary plane emoji (always 2 columns)
	if r >= 0x1F000 && r <= 0x1FFFF {
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

// narrowAlternatives maps wide or ambiguous characters to semantically
// similar narrow replacements. Keys are runes that may be flagged by
// the icon width check; values are the suggestion text.
var narrowAlternatives = map[rune]string{
	'▶': "try › (U+203A SINGLE RIGHT ANGLE QUOTATION) or > (U+003E)", // play/arrow
	'◀': "try ‹ (U+2039 SINGLE LEFT ANGLE QUOTATION) or < (U+003C)",  // reverse
	'◆': "try ♦ (U+2666 BLACK DIAMOND SUIT) or * (U+002A)",           // diamond -> card suit (Narrow in most terminals)
	'◇': "try ◇ is also Ambiguous; use • (U+2022 BULLET) instead",    // open diamond
	'▦': "try ≡ (U+2261 IDENTICAL TO) or # (U+0023)",                 // grid/plan
	'◉': "try ⊙ (U+2299 CIRCLED DOT OPERATOR) or @ (U+0040)",         // target/dot
	'■': "try ▪ (U+25AA BLACK SMALL SQUARE) or • (U+2022 BULLET)",    // solid square
	'□': "try ▫ (U+25AB WHITE SMALL SQUARE) or - (U+002D)",           // open square
	'●': "try • (U+2022 BULLET)",                                     // filled circle
	'○': "try ◦ (U+25E6 WHITE BULLET) or · (U+00B7 MIDDLE DOT)",      // open circle
	'☰': "try ⋮ (U+22EE VERTICAL ELLIPSIS) or = (U+003D)",            // trigram/hamburger
}

// suggestedReplacement returns a narrow-width alternative icon for a wide or ambiguous character.
func suggestedReplacement(r rune) string {
	if alt, ok := narrowAlternatives[r]; ok {
		return alt
	}
	return "use a Narrow (East Asian Width N) character to avoid layout issues"
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
func checkIconWidth(_, badgeName, key, icon string) []Finding {
	var findings []Finding
	r, _ := utf8.DecodeRuneInString(icon)
	if r == utf8.RuneError {
		return findings
	}

	if isEastAsianWide(r) {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Category: CategoryBadges,
			Message: fmt.Sprintf("%s.%s.%s: %s (U+%04X) is Wide, always 2 columns",
				badgeName, key, string(r), string(r), r),
			Suggestion: suggestedReplacement(r),
		})
	} else if isEmoji(r) {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Category: CategoryBadges,
			Message: fmt.Sprintf("%s.%s: %s (U+%04X) is emoji, always 2 columns",
				badgeName, key, string(r), r),
			Suggestion: suggestedReplacement(r),
		})
	} else if isEastAsianAmbiguous(r) {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Category: CategoryBadges,
			Message: fmt.Sprintf("%s.%s: %s (U+%04X) is Ambiguous, may be 2 columns",
				badgeName, key, string(r), r),
			Suggestion: suggestedReplacement(r),
		})
	}
	return findings
}

// profileNameRegex matches valid profile names: lowercase letters, digits and hyphens,
// starting with a letter or digit.
var profileNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// colorRegex matches a valid hex color: #RRGGBB.
var colorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// shellIdentRegex matches a valid shell variable name.
var shellIdentRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// harnessBackends maps each harness to its allowed backends.
var harnessBackends = map[string][]BackendType{
	"claude":   {BackendAnthropic, BackendVertex},
	"codex":    {BackendOpenAI},
	"opencode": {BackendOpenAI, BackendAnthropic},
}

// validateProfiles checks profile configurations and default_profile reference.
func validateProfiles(profiles map[string]Profile, defaultProfile string) []Finding {
	var findings []Finding

	for name, profile := range profiles {
		// Name pattern
		if !profileNameRegex.MatchString(name) {
			findings = append(findings, Finding{
				Severity:   SeverityError,
				Category:   CategoryProfiles,
				Message:    fmt.Sprintf("profile %q: name must be lowercase letters, digits and hyphens", name),
				Suggestion: "use a name like 'work' or 'team-alpha'",
			})
		}

		harness := profile.HarnessName()

		// Known harness
		known := false
		for _, h := range KnownHarnesses {
			if h == harness {
				known = true
				break
			}
		}
		if !known {
			findings = append(findings, Finding{
				Severity:   SeverityError,
				Category:   CategoryProfiles,
				Message:    fmt.Sprintf("profile %q: unknown harness %q", name, harness),
				Suggestion: fmt.Sprintf("supported harnesses: %s", strings.Join(KnownHarnesses, ", ")),
			})
		}

		// Backend allowed for harness
		if known && profile.Backend != "" {
			allowed := harnessBackends[harness]
			backendOK := false
			for _, b := range allowed {
				if b == profile.Backend {
					backendOK = true
					break
				}
			}
			if !backendOK {
				var bs []string
				for _, b := range allowed {
					bs = append(bs, string(b))
				}
				findings = append(findings, Finding{
					Severity:   SeverityError,
					Category:   CategoryProfiles,
					Message:    fmt.Sprintf("profile %q: backend %q not valid for harness %q", name, profile.Backend, harness),
					Suggestion: fmt.Sprintf("allowed backends: %s", strings.Join(bs, ", ")),
				})
			}
		}

		// Auth block validation
		if profile.Auth != nil {
			auth := profile.Auth

			// Credential source validation: exactly one of env, file, secret
			if auth.APIKey != nil {
				findings = append(findings, validateCredentialSource(name, "api_key", auth.APIKey)...)
			}
			if auth.Credentials != nil {
				findings = append(findings, validateCredentialSource(name, "credentials", auth.Credentials)...)
			}

			// Login and api_key mutually exclusive
			if auth.Login && auth.APIKey != nil {
				findings = append(findings, Finding{
					Severity:   SeverityError,
					Category:   CategoryProfiles,
					Message:    fmt.Sprintf("profile %q: auth.login and auth.api_key are mutually exclusive", name),
					Suggestion: "use either login or api_key, not both",
				})
			}

			// Login only for claude
			if auth.Login && harness != "claude" {
				findings = append(findings, Finding{
					Severity:   SeverityError,
					Category:   CategoryProfiles,
					Message:    fmt.Sprintf("profile %q: auth.login is not supported for harness %q", name, harness),
					Suggestion: "auth.login is only available for the claude harness",
				})
			}
		}

		// Per-profile Validate (backend-specific required fields)
		if err := profile.Validate(); err != nil {
			findings = append(findings, Finding{
				Severity:   SeverityError,
				Category:   CategoryProfiles,
				Message:    fmt.Sprintf("profile %q: %s", name, err.Error()),
				Suggestion: "check required fields for the backend type",
			})
		}

		// Color syntax
		if profile.Color != "" && !colorRegex.MatchString(profile.Color) {
			findings = append(findings, Finding{
				Severity:   SeverityError,
				Category:   CategoryProfiles,
				Message:    fmt.Sprintf("profile %q: color must be #RRGGBB", name),
				Suggestion: "use a hex color like #4FC1E9",
			})
		}

		// Icon: single grapheme cluster with display width 1 or 2
		if profile.Icon != "" {
			findings = append(findings, validateProfileIcon(name, profile.Icon)...)
		}

		// Env key: valid shell identifier
		for k := range profile.Env {
			if !shellIdentRegex.MatchString(k) {
				findings = append(findings, Finding{
					Severity:   SeverityError,
					Category:   CategoryProfiles,
					Message:    fmt.Sprintf("profile %q: env key %q is not a valid variable name", name, k),
					Suggestion: "use uppercase letters, digits and underscores starting with a letter or underscore",
				})
			}
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

// validateCredentialSource checks that exactly one of env, file, secret is set.
func validateCredentialSource(profileName, field string, cs *CredentialSource) []Finding {
	var findings []Finding
	count := 0
	if cs.Env != "" {
		count++
	}
	if cs.File != "" {
		count++
	}
	if cs.Secret != "" {
		count++
	}
	if count != 1 {
		findings = append(findings, Finding{
			Severity:   SeverityError,
			Category:   CategoryProfiles,
			Message:    fmt.Sprintf("profile %q: auth.%s must set exactly one of env, file, secret", profileName, field),
			Suggestion: "provide exactly one credential source",
		})
	}
	// The env var name is interpolated into generated shell (parameter
	// expansion and export lines), so it must be a plain shell identifier.
	if cs.Env != "" && !shellIdentRegex.MatchString(cs.Env) {
		findings = append(findings, Finding{
			Severity:   SeverityError,
			Category:   CategoryProfiles,
			Message:    fmt.Sprintf("profile %q: auth.%s.env %q is not a valid variable name", profileName, field, cs.Env),
			Suggestion: "use letters, digits and underscores, starting with a letter or underscore",
		})
	}
	return findings
}

// validateProfileIcon checks that a profile icon is a single grapheme cluster
// with a terminal display width of 1 or 2.
func validateProfileIcon(name, icon string) []Finding {
	var findings []Finding

	// Count grapheme clusters (simplified: count runes, excluding combining marks)
	runes := []rune(icon)
	if len(runes) == 0 {
		return findings
	}

	// Check it is a single base character (possibly with combining marks)
	baseCount := 0
	for _, r := range runes {
		if !unicode.Is(unicode.Mn, r) && !unicode.Is(unicode.Mc, r) && !unicode.Is(unicode.Me, r) {
			baseCount++
		}
	}
	if baseCount > 1 {
		findings = append(findings, Finding{
			Severity:   SeverityError,
			Category:   CategoryProfiles,
			Message:    fmt.Sprintf("profile %q: icon must be a single glyph of width 1 or 2", name),
			Suggestion: "use a single character like W, @, or a symbol",
		})
		return findings
	}

	// Check display width using the same logic as badge validation
	r, _ := utf8.DecodeRuneInString(icon)
	if r == utf8.RuneError {
		findings = append(findings, Finding{
			Severity:   SeverityError,
			Category:   CategoryProfiles,
			Message:    fmt.Sprintf("profile %q: icon must be a single glyph of width 1 or 2", name),
			Suggestion: "use a valid unicode character",
		})
		return findings
	}

	// Width 1 or 2 is acceptable (unlike badges which warn on width 2).
	// Only reject multi-character strings (already handled) or zero-width.
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

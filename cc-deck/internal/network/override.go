package network

import (
	"fmt"
	"os"
	"strings"
)

// OverrideResult holds the result of applying domain overrides.
type OverrideResult struct {
	// Domains is the final list of domain group names/literals after overrides.
	Domains []string
	// Disabled is true when "all" was specified (no filtering).
	Disabled bool
}

// ApplyOverrides parses the --allowed-domains flag value and applies it to
// the manifest's allowed domains list.
//
// Syntax:
//   - "+group" or "+domain": add to manifest defaults
//   - "-group" or "-domain": remove from manifest defaults
//   - "group,group": replace manifest defaults entirely
//   - "all": disable network filtering (with warning)
//
// Mixed +/- entries are allowed. Bare entries (no prefix) trigger full replacement.
func ApplyOverrides(flagValue string, manifestDomains []string) (*OverrideResult, error) {
	if flagValue == "" {
		return &OverrideResult{Domains: manifestDomains}, nil
	}

	flagValue = strings.TrimSpace(flagValue)

	if flagValue == "all" {
		fmt.Fprintln(os.Stderr, "WARNING: --allowed-domains=all disables network filtering entirely. All outbound traffic will be allowed.")
		return &OverrideResult{Disabled: true}, nil
	}

	parts := strings.Split(flagValue, ",")

	// Determine mode: if any entry has +/- prefix, it's additive/subtractive.
	// If all entries are bare, it's full replacement.
	hasModifiers := false
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "+") || strings.HasPrefix(p, "-") {
			hasModifiers = true
			break
		}
	}

	if !hasModifiers {
		// Full replacement mode
		var domains []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				domains = append(domains, p)
			}
		}
		return &OverrideResult{Domains: domains}, nil
	}

	// Additive/subtractive mode: start from manifest defaults
	result := make([]string, len(manifestDomains))
	copy(result, manifestDomains)

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		if strings.HasPrefix(p, "+") {
			name := p[1:]
			if name == "" {
				return nil, fmt.Errorf("empty group name after '+' in --allowed-domains")
			}
			// Add if not already present
			if !containsString(result, name) {
				result = append(result, name)
			}
		} else if strings.HasPrefix(p, "-") {
			name := p[1:]
			if name == "" {
				return nil, fmt.Errorf("empty group name after '-' in --allowed-domains")
			}
			result = removeString(result, name)
		} else {
			return nil, fmt.Errorf("mixed bare and +/- entries in --allowed-domains: %q has no prefix", p)
		}
	}

	return &OverrideResult{Domains: result}, nil
}

func containsString(ss []string, s string) bool {
	for _, item := range ss {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(ss []string, s string) []string {
	var result []string
	for _, item := range ss {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

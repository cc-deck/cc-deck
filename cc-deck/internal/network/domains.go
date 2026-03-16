package network

import (
	"fmt"
	"sort"
	"strings"
)

// Source indicates where a domain group definition originates.
type Source string

const (
	SourceBuiltin  Source = "builtin"
	SourceUser     Source = "user"
	SourceExtended Source = "extended"
)

// DomainGroup is a named collection of domain patterns.
type DomainGroup struct {
	Name     string
	Source   Source
	Domains  []string
	Extends  string
	Includes []string
}

// Resolver expands domain group names into domain pattern lists.
type Resolver struct {
	builtin map[string]DomainGroup
	user    map[string]userConfigGroup
}

// NewResolver creates a Resolver with built-in groups and optional user config.
func NewResolver(userGroups map[string]userConfigGroup) *Resolver {
	return &Resolver{
		builtin: builtinGroups,
		user:    userGroups,
	}
}

// ExpandGroup resolves a single group name to its domain list.
// Returns an error if the group is not found or a cycle is detected.
func (r *Resolver) ExpandGroup(name string) ([]string, error) {
	visited := make(map[string]bool)
	return r.expandGroupRecursive(name, visited)
}

func (r *Resolver) expandGroupRecursive(name string, visited map[string]bool) ([]string, error) {
	if visited[name] {
		return nil, fmt.Errorf("circular include detected: group %q", name)
	}
	visited[name] = true

	// Check user config first (may override or extend built-in)
	if ug, ok := r.user[name]; ok {
		return r.resolveUserGroup(name, ug, visited)
	}

	// Fall back to built-in
	if bg, ok := r.builtin[name]; ok {
		return bg.Domains, nil
	}

	return nil, &UnknownGroupError{Name: name, Available: r.AllGroupNames()}
}

// resolveUserGroup resolves a user-defined group, handling extends and includes.
func (r *Resolver) resolveUserGroup(name string, ug userConfigGroup, visited map[string]bool) ([]string, error) {
	var domains []string

	// If extends: builtin, start with built-in domains
	if ug.Extends == "builtin" {
		if bg, ok := r.builtin[name]; ok {
			domains = append(domains, bg.Domains...)
		}
	}

	// Add user-defined domains
	domains = append(domains, ug.Domains...)

	// Resolve includes
	for _, inc := range ug.Includes {
		incDomains, err := r.expandGroupRecursive(inc, visited)
		if err != nil {
			return nil, fmt.Errorf("expanding include %q in group %q: %w", inc, name, err)
		}
		domains = append(domains, incDomains...)
	}

	return domains, nil
}

// ExpandAll resolves a list of names (group names or literal domains) into
// a deduplicated, wildcard-reduced domain list.
// Names without dots are treated as group names; names with dots are literal domains.
func (r *Resolver) ExpandAll(names []string) ([]string, error) {
	var allDomains []string

	for _, name := range names {
		if isGroupName(name) {
			domains, err := r.ExpandGroup(name)
			if err != nil {
				return nil, err
			}
			allDomains = append(allDomains, domains...)
		} else {
			// Literal domain (contains a dot)
			allDomains = append(allDomains, name)
		}
	}

	return WildcardDedup(allDomains), nil
}

// AllGroupNames returns a sorted list of all available group names (built-in + user).
func (r *Resolver) AllGroupNames() []string {
	seen := make(map[string]bool)
	for name := range r.builtin {
		seen[name] = true
	}
	for name := range r.user {
		seen[name] = true
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sortStrings(names)
	return names
}

// GroupSource returns the source type for a group name.
func (r *Resolver) GroupSource(name string) Source {
	_, inUser := r.user[name]
	_, inBuiltin := r.builtin[name]

	if inUser && inBuiltin {
		ug := r.user[name]
		if ug.Extends == "builtin" {
			return SourceExtended
		}
		return SourceUser // user override
	}
	if inUser {
		return SourceUser
	}
	if inBuiltin {
		return SourceBuiltin
	}
	return ""
}

// isGroupName returns true if the name looks like a group name (no dots).
// Names containing dots are treated as literal domain patterns.
func isGroupName(name string) bool {
	return !strings.Contains(name, ".")
}

// WildcardDedup removes domain entries that are already covered by a wildcard pattern.
// For example, if ".github.com" is present, "api.github.com" is removed since
// the wildcard covers all subdomains. The explicit domain "github.com" is also
// covered by ".github.com".
func WildcardDedup(domains []string) []string {
	if len(domains) == 0 {
		return nil
	}

	// Collect all wildcard patterns (entries starting with ".")
	wildcards := make(map[string]bool)
	for _, d := range domains {
		if strings.HasPrefix(d, ".") {
			wildcards[d] = true
		}
	}

	if len(wildcards) == 0 {
		return dedupStrings(domains)
	}

	// Filter domains: keep if not covered by any wildcard
	var result []string
	seen := make(map[string]bool)
	for _, d := range domains {
		if seen[d] {
			continue
		}
		seen[d] = true

		// Wildcard patterns themselves are always kept
		if strings.HasPrefix(d, ".") {
			result = append(result, d)
			continue
		}

		// Check if this domain is covered by a wildcard
		if isCoveredByWildcard(d, wildcards) {
			continue
		}

		result = append(result, d)
	}

	return result
}

// isCoveredByWildcard checks if a domain is covered by any wildcard pattern.
// ".example.com" covers "example.com" and "sub.example.com".
func isCoveredByWildcard(domain string, wildcards map[string]bool) bool {
	for wc := range wildcards {
		suffix := wc // e.g., ".github.com"
		// Wildcard ".github.com" covers "github.com" (exact base domain)
		if domain == suffix[1:] {
			return true
		}
		// Wildcard ".github.com" covers "api.github.com" (subdomain)
		if strings.HasSuffix(domain, suffix) {
			return true
		}
	}
	return false
}

// dedupStrings removes duplicate strings while preserving order.
func dedupStrings(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// sortStrings sorts a string slice in place.
func sortStrings(ss []string) {
	sort.Strings(ss)
}

// UnknownGroupError is returned when a group name is not found in built-in or user config.
type UnknownGroupError struct {
	Name      string
	Available []string
}

func (e *UnknownGroupError) Error() string {
	return fmt.Sprintf("unknown domain group %q; available groups: %s", e.Name, strings.Join(e.Available, ", "))
}

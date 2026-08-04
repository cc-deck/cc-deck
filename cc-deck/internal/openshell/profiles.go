package openshell

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"

	v1 "github.com/rhuss/openshell-sdk-go/openshell/v1"
)

// agentProfiles maps agent names to their required profile IDs.
var agentProfiles = map[string][]string{
	"claude":  {"anthropic", "claude-agent"},
	"opencode": {"openai", "opencode-agent"},
	"codex":   {"openai", "codex-agent"},
}

// toolProfiles maps tool names (as detected or declared) to profile IDs.
var toolProfiles = map[string]string{
	"python": "python",
	"node":   "nodejs",
	"npm":    "nodejs",
	"go":     "golang",
	"rust":   "rust",
	"cargo":  "rust",
	"docker": "docker",
}

// credentialProfiles maps credential types to profile IDs.
var credentialProfiles = map[string]string{
	"vertex": "vertexai",
}

// alwaysIncludedProfiles are included regardless of manifest content.
var alwaysIncludedProfiles = []string{"github", "gitlab"}

// registryProfiles maps container registry names to profile IDs.
var registryProfiles = map[string]string{
	"quay": "quay",
}

// ResolveProfiles maps agents, tools, and credentials to a deduplicated,
// sorted list of OpenShell profile IDs. Git hosting profiles (github, gitlab)
// are always included.
func ResolveProfiles(agents, tools, credentials []string) []string {
	seen := make(map[string]bool)

	for _, name := range alwaysIncludedProfiles {
		seen[name] = true
	}

	for _, a := range agents {
		if profiles, ok := agentProfiles[a]; ok {
			for _, p := range profiles {
				seen[p] = true
			}
		}
	}

	for _, t := range tools {
		if p, ok := toolProfiles[t]; ok {
			seen[p] = true
		}
	}

	for _, c := range credentials {
		if p, ok := credentialProfiles[c]; ok {
			seen[p] = true
		}
		if p, ok := registryProfiles[c]; ok {
			seen[p] = true
		}
	}

	result := make([]string, 0, len(seen))
	for p := range seen {
		result = append(result, p)
	}
	sort.Strings(result)
	return result
}

// LookupToolProfile returns the profile ID for a tool name, or empty string
// if no mapping exists.
func LookupToolProfile(tool string) string {
	return toolProfiles[tool]
}

// LookupCredentialProfile returns the profile ID for a credential spec name.
// For "api" credentials, the profile comes from the agent mapping (first
// profile ID for the agent). For other credentials, both credentialProfiles
// and registryProfiles tables are consulted.
func LookupCredentialProfile(credName, agentName string) string {
	if credName == "api" {
		if profiles, ok := agentProfiles[agentName]; ok && len(profiles) > 0 {
			return profiles[0]
		}
		return "anthropic"
	}
	if p, ok := credentialProfiles[credName]; ok {
		return p
	}
	return registryProfiles[credName]
}

// VerifyProfiles checks which profile IDs exist on the gateway. Returns the
// verified (existing) profile IDs and the missing ones. Missing profiles are
// logged as warnings. Transient errors (network, auth) are propagated.
func VerifyProfiles(ctx context.Context, client v1.ClientInterface, profileIDs []string) (verified, missing []string, err error) {
	for _, id := range profileIDs {
		_, getErr := client.Providers().Profiles().Get(ctx, id)
		if getErr != nil {
			if v1.IsNotFound(getErr) {
				log.Printf("WARNING: gateway profile %q not found, skipping", id)
				missing = append(missing, id)
				continue
			}
			return nil, nil, fmt.Errorf("verifying profile %q: %w", id, getErr)
		}
		verified = append(verified, id)
	}
	return verified, missing, nil
}

var sanitizeRe = regexp.MustCompile(`[^a-z0-9-]`)
var multiDash = regexp.MustCompile(`-{2,}`)

// SanitizeWorkspaceName produces a valid profile name fragment from a
// workspace name: lowercase, alphanumeric + hyphens only, truncated to 50
// characters.
func SanitizeWorkspaceName(name string) string {
	s := strings.ToLower(name)
	s = sanitizeRe.ReplaceAllString(s, "-")
	s = multiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 50 {
		s = s[:50]
		s = strings.TrimRight(s, "-")
	}
	if s == "" {
		s = "ws"
	}
	return s
}

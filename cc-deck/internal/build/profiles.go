package build

import (
	"gopkg.in/yaml.v3"

	"github.com/cc-deck/cc-deck/internal/openshell"
)

// ProfileManifest lists the OpenShell profiles required by an image.
// Embedded at /etc/openshell/profiles.yaml in the OCI image.
type ProfileManifest struct {
	Profiles       []string              `yaml:"profiles"`
	MCP            []MCPManifestEntry    `yaml:"mcp,omitempty"`
	CustomDomains  []string              `yaml:"custom_domains,omitempty"`
	AgentBinaries  []string              `yaml:"agent_binaries,omitempty"`
}

// MCPManifestEntry carries the minimal MCP server metadata needed at
// workspace creation time to import an ephemeral profile.
type MCPManifestEntry struct {
	Name     string `yaml:"name"`
	Endpoint string `yaml:"endpoint,omitempty"`
}

// BuildProfileManifest generates a ProfileManifest from a build manifest.
// It maps agents, tools, and credentials to profile IDs via the static
// profile mapping table, deduplicates, and sorts the result.
func BuildProfileManifest(manifest *Manifest, matchedComponents []PolicyComponent) *ProfileManifest {
	agents := manifest.EffectiveAgents()

	var tools []string
	for _, comp := range matchedComponents {
		for _, t := range comp.Match.Tools {
			tools = append(tools, t)
		}
	}
	for _, t := range manifest.Tools {
		tools = append(tools, t.Name)
	}

	var credentials []string
	for _, c := range manifest.Credentials {
		credentials = append(credentials, c.Type)
	}

	profiles := openshell.ResolveProfiles(agents, tools, credentials)

	pm := &ProfileManifest{
		Profiles: profiles,
	}

	for _, mcp := range manifest.MCP {
		if mcp.Endpoint == "" {
			continue
		}
		pm.MCP = append(pm.MCP, MCPManifestEntry{
			Name:     mcp.Name,
			Endpoint: mcp.Endpoint,
		})
	}

	if manifest.Network != nil {
		seen := make(map[string]bool)
		for _, d := range manifest.Network.AllowedDomains {
			if !seen[d] {
				pm.CustomDomains = append(pm.CustomDomains, d)
				seen[d] = true
			}
		}
		activeAgents := make(map[string]bool)
		for _, a := range agents {
			activeAgents[a] = true
		}
		for agentName, domains := range manifest.Network.AllowedDomainsPerAgent {
			if !activeAgents[agentName] {
				continue
			}
			for _, d := range domains {
				if !seen[d] {
					pm.CustomDomains = append(pm.CustomDomains, d)
					seen[d] = true
				}
			}
		}
	}

	return pm
}

// MarshalProfileManifest serializes a ProfileManifest to YAML.
func MarshalProfileManifest(pm *ProfileManifest) ([]byte, error) {
	return yaml.Marshal(pm)
}

// ParseProfileManifest deserializes a ProfileManifest from YAML.
func ParseProfileManifest(data []byte) (*ProfileManifest, error) {
	var pm ProfileManifest
	if err := yaml.Unmarshal(data, &pm); err != nil {
		return nil, err
	}
	return &pm, nil
}

// GenerateProfileManifest builds and writes a profile manifest for OpenShell
// targets. It combines the manifest agents/tools/credentials with auto-detected
// tool components to produce a deduplicated profile list. Returns the generated
// ProfileManifest, or nil if no OpenShell target is configured.
func GenerateProfileManifest(manifest *Manifest, matchedComponents []PolicyComponent) *ProfileManifest {
	if manifest.Targets == nil || manifest.Targets.OpenShell == nil {
		return nil
	}
	return BuildProfileManifest(manifest, matchedComponents)
}

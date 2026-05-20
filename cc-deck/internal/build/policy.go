package build

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// OpenShellPolicy defines explicit OpenShell policy overrides.
type OpenShellPolicy struct {
	FilesystemPolicy *FilesystemPolicy        `yaml:"filesystem_policy,omitempty"`
	Landlock         *LandlockConfig           `yaml:"landlock,omitempty"`
	Process          *ProcessConfig            `yaml:"process,omitempty"`
	NetworkPolicies  map[string]NetworkPolicy  `yaml:"network_policies,omitempty"`
}

// FilesystemPolicy defines read-only and read-write filesystem paths.
type FilesystemPolicy struct {
	IncludeWorkdir bool     `yaml:"include_workdir,omitempty"`
	ReadOnly       []string `yaml:"read_only,omitempty"`
	ReadWrite      []string `yaml:"read_write,omitempty"`
}

// LandlockConfig holds Landlock LSM settings.
type LandlockConfig struct {
	Compatibility string `yaml:"compatibility,omitempty"`
}

// ProcessConfig defines sandbox process execution settings.
type ProcessConfig struct {
	RunAsUser  string `yaml:"run_as_user,omitempty"`
	RunAsGroup string `yaml:"run_as_group,omitempty"`
}

// NetworkPolicy defines a named set of endpoint/binary restrictions.
type NetworkPolicy struct {
	Name      string           `yaml:"name"`
	Endpoints []PolicyEndpoint `yaml:"endpoints"`
	Binaries  []PolicyBinary   `yaml:"binaries,omitempty"`
}

// PolicyEndpoint is a host:port pair for network access control.
type PolicyEndpoint struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// PolicyBinary restricts network access to a specific binary path.
type PolicyBinary struct {
	Path string `yaml:"path"`
}

// PolicyFile represents a complete OpenShell policy YAML document.
type PolicyFile struct {
	Version          int                      `yaml:"version"`
	FilesystemPolicy *FilesystemPolicy        `yaml:"filesystem_policy"`
	Landlock         *LandlockConfig          `yaml:"landlock"`
	Process          *ProcessConfig           `yaml:"process"`
	NetworkPolicies  map[string]NetworkPolicy `yaml:"network_policies"`
}

// DefaultPolicy returns a PolicyFile with the FR-013/FR-014/FR-015 defaults.
func DefaultPolicy() *PolicyFile {
	return &PolicyFile{
		Version: 1,
		FilesystemPolicy: &FilesystemPolicy{
			IncludeWorkdir: true,
			ReadOnly:       []string{"/usr", "/lib", "/proc", "/etc", "/var/log"},
			ReadWrite:      []string{"/sandbox", "/tmp", "/dev/null", "/dev/urandom", "/dev/random", "/dev/pts"},
		},
		Landlock: &LandlockConfig{
			Compatibility: "best_effort",
		},
		Process: &ProcessConfig{
			RunAsUser:  "sandbox",
			RunAsGroup: "sandbox",
		},
		NetworkPolicies: map[string]NetworkPolicy{
			"claude_code": {
				Name: "Claude Code",
				Endpoints: []PolicyEndpoint{
					{Host: "api.anthropic.com", Port: 443},
					{Host: "statsig.anthropic.com", Port: 443},
					{Host: "sentry.io", Port: 443},
				},
			},
			"github": {
				Name: "GitHub",
				Endpoints: []PolicyEndpoint{
					{Host: "github.com", Port: 443},
					{Host: "api.github.com", Port: 443},
				},
			},
		},
	}
}

// GeneratePolicy builds a policy from defaults and auto-generates network_policies
// entries from the manifest's allowed_domains and credential endpoints.
func GeneratePolicy(manifest *Manifest) (*PolicyFile, error) {
	policy := DefaultPolicy()

	if manifest.Network != nil {
		for _, domain := range manifest.Network.AllowedDomains {
			slug := slugify(domain)
			policy.NetworkPolicies[slug] = NetworkPolicy{
				Name: domain,
				Endpoints: []PolicyEndpoint{
					{Host: domain, Port: 443},
				},
			}
		}
	}

	// Add package registry endpoints based on detected tools.
	for _, tool := range manifest.Tools {
		addToolEndpoints(policy, tool.Name)
	}
	for _, src := range manifest.Sources {
		for _, t := range src.DetectedTools {
			addToolEndpoints(policy, t)
		}
	}

	// Add credential-specific endpoints.
	for _, cred := range manifest.Credentials {
		switch cred.Type {
		case "vertex":
			// Add GCP OAuth endpoint.
			policy.NetworkPolicies["gcp_oauth"] = NetworkPolicy{
				Name: "GCP OAuth",
				Endpoints: []PolicyEndpoint{
					{Host: "oauth2.googleapis.com", Port: 443},
				},
			}
			// Add Vertex AI regional endpoint.
			region := os.Getenv("CLOUD_ML_REGION")
			if region == "" {
				region = "us-east1"
			}
			host := region + "-aiplatform.googleapis.com"
			policy.NetworkPolicies["gcp_vertex_"+slugify(region)] = NetworkPolicy{
				Name: "GCP Vertex AI (" + region + ")",
				Endpoints: []PolicyEndpoint{
					{Host: host, Port: 443},
				},
			}
		case "generic":
			// Add custom endpoints from the credential entry.
			for _, ep := range cred.Endpoints {
				slug := slugify(ep.Host)
				policy.NetworkPolicies["cred_"+slug] = NetworkPolicy{
					Name: ep.Host,
					Endpoints: []PolicyEndpoint{
						{Host: ep.Host, Port: ep.Port},
					},
				}
			}
		}
	}

	return policy, nil
}

// toolEndpoints maps tool name keywords to package registry endpoints that
// need to be allowed in the network policy for builds and package installs.
var toolEndpoints = map[string][]PolicyEndpoint{
	"rust": {
		{Host: "crates.io", Port: 443},
		{Host: "index.crates.io", Port: 443},
		{Host: "static.crates.io", Port: 443},
	},
	"cargo": {
		{Host: "crates.io", Port: 443},
		{Host: "index.crates.io", Port: 443},
		{Host: "static.crates.io", Port: 443},
	},
	"go": {
		{Host: "proxy.golang.org", Port: 443},
		{Host: "sum.golang.org", Port: 443},
		{Host: "storage.googleapis.com", Port: 443},
	},
	"node": {
		{Host: "registry.npmjs.org", Port: 443},
		{Host: "npmjs.org", Port: 443},
	},
	"npm": {
		{Host: "registry.npmjs.org", Port: 443},
		{Host: "npmjs.org", Port: 443},
	},
	"python": {
		{Host: "pypi.org", Port: 443},
		{Host: "files.pythonhosted.org", Port: 443},
	},
	"pip": {
		{Host: "pypi.org", Port: 443},
		{Host: "files.pythonhosted.org", Port: 443},
	},
	"uv": {
		{Host: "pypi.org", Port: 443},
		{Host: "files.pythonhosted.org", Port: 443},
	},
}

// addToolEndpoints checks a tool name against known package registries and
// adds the corresponding endpoints to the policy if not already present.
func addToolEndpoints(policy *PolicyFile, toolName string) {
	lower := strings.ToLower(toolName)
	for keyword, endpoints := range toolEndpoints {
		if strings.Contains(lower, keyword) {
			slug := "pkg_" + keyword
			if _, exists := policy.NetworkPolicies[slug]; !exists {
				policy.NetworkPolicies[slug] = NetworkPolicy{
					Name:      keyword + " packages",
					Endpoints: endpoints,
				}
			}
		}
	}
}

// MarshalPolicy serializes a PolicyFile to YAML.
func MarshalPolicy(policy *PolicyFile) ([]byte, error) {
	data, err := yaml.Marshal(policy)
	if err != nil {
		return nil, fmt.Errorf("marshaling policy: %w", err)
	}
	return data, nil
}

// MergePolicy applies explicit overrides to a base policy.
func MergePolicy(base *PolicyFile, overrides *OpenShellPolicy) *PolicyFile {
	if overrides == nil {
		return base
	}

	result := *base

	// Deep-copy pointer fields to avoid mutating the base
	if base.FilesystemPolicy != nil {
		fs := *base.FilesystemPolicy
		result.FilesystemPolicy = &fs
	}
	if base.Landlock != nil {
		ll := *base.Landlock
		result.Landlock = &ll
	}
	if base.Process != nil {
		pc := *base.Process
		result.Process = &pc
	}
	merged := make(map[string]NetworkPolicy, len(base.NetworkPolicies))
	for k, v := range base.NetworkPolicies {
		merged[k] = v
	}
	result.NetworkPolicies = merged

	if overrides.FilesystemPolicy != nil {
		result.FilesystemPolicy = overrides.FilesystemPolicy
	}
	if overrides.Landlock != nil {
		result.Landlock = overrides.Landlock
	}
	if overrides.Process != nil {
		result.Process = overrides.Process
	}

	if len(overrides.NetworkPolicies) > 0 {

		overrideHosts := make(map[string]bool)
		for _, np := range overrides.NetworkPolicies {
			for _, ep := range np.Endpoints {
				overrideHosts[ep.Host] = true
			}
		}

		for key, np := range merged {
			for _, ep := range np.Endpoints {
				if overrideHosts[ep.Host] {
					delete(merged, key)
					break
				}
			}
		}

		for k, v := range overrides.NetworkPolicies {
			merged[k] = v
		}
	}

	return &result
}

// WellKnownBinaries maps tool names to their expected binary paths.
// Used as a reference by the AI command spec during policy generation.
var WellKnownBinaries = map[string]string{
	"git":        "/usr/bin/git",
	"node":       "/usr/bin/node",
	"nodejs":     "/usr/bin/node",
	"python3":    "/usr/bin/python3",
	"go":         "/usr/bin/go",
	"claude":     "/usr/local/bin/claude",
	"Claude Code": "/usr/local/bin/claude",
}

// slugify converts a domain name to a YAML-safe key.
func slugify(domain string) string {
	s := strings.ReplaceAll(domain, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

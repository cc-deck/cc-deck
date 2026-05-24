package build

import (
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cc-deck/cc-deck/internal/network"
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
// For endpoints with protocol: rest, OpenShell 0.0.46+ requires either
// an "access" field (e.g., "full") or explicit "rules".
type PolicyEndpoint struct {
	Host        string        `yaml:"host"`
	Port        int           `yaml:"port"`
	Protocol    string        `yaml:"protocol,omitempty"`
	Enforcement string        `yaml:"enforcement,omitempty"`
	Access      string        `yaml:"access,omitempty"`
	Rules       []PolicyRule  `yaml:"rules,omitempty"`
}

// PolicyRule defines an L7 allow/deny rule for rest protocol endpoints.
type PolicyRule struct {
	Allow *PolicyRuleMatch `yaml:"allow,omitempty"`
}

// PolicyRuleMatch matches HTTP method and path patterns.
type PolicyRuleMatch struct {
	Method string `yaml:"method"`
	Path   string `yaml:"path"`
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

// AssemblyOptions controls how policy assembly resolves binary paths.
type AssemblyOptions struct {
	StripBinaries bool
	ProbeReport   *ProbeReport
}

// AssemblyResult holds both the assembled policy and the matched components.
type AssemblyResult struct {
	Policy            *PolicyFile
	MatchedComponents []PolicyComponent
}

// AssemblePolicyWithOptions builds a PolicyFile with control over binary handling.
// When opts.StripBinaries is true, non-explicit binaries are cleared (first-pass mode).
// When opts.ProbeReport is non-nil, probe results and runtime globs are applied.
// Returns both the policy and the matched components for probing.
func AssemblePolicyWithOptions(manifest *Manifest, catalogFS fs.FS, catalogRoot string, userLocalFS fs.FS, userLocalRoot string, opts AssemblyOptions) (*AssemblyResult, error) {
	policy, matched, err := assemblePolicyCore(manifest, catalogFS, catalogRoot, userLocalFS, userLocalRoot, opts)
	if err != nil {
		return nil, err
	}
	return &AssemblyResult{Policy: policy, MatchedComponents: matched}, nil
}

// AssemblePolicy builds a PolicyFile from component files across multiple tiers.
// It loads embedded components, optional catalog and user-local components,
// resolves precedence by filename stem, filters by manifest match conditions,
// and produces a deterministic PolicyFile with alphabetically sorted keys.
func AssemblePolicy(manifest *Manifest, catalogFS fs.FS, catalogRoot string, userLocalFS fs.FS, userLocalRoot string) (*PolicyFile, error) {
	policy, _, err := assemblePolicyCore(manifest, catalogFS, catalogRoot, userLocalFS, userLocalRoot, AssemblyOptions{})
	return policy, err
}

func assemblePolicyCore(manifest *Manifest, catalogFS fs.FS, catalogRoot string, userLocalFS fs.FS, userLocalRoot string, opts AssemblyOptions) (*PolicyFile, []PolicyComponent, error) {
	embedded, embWarnings := LoadEmbeddedComponents()
	for _, w := range embWarnings {
		fmt.Printf("WARNING: embedded component: %v\n", w)
	}

	var catalogTier map[string]PolicyComponent
	if catalogFS != nil {
		var catWarnings []error
		catalogTier, catWarnings = LoadComponentTier(catalogFS, catalogRoot)
		for _, w := range catWarnings {
			fmt.Printf("WARNING: catalog component: %v\n", w)
		}
	}

	var userLocalTier map[string]PolicyComponent
	if userLocalFS != nil {
		var ulWarnings []error
		userLocalTier, ulWarnings = LoadComponentTier(userLocalFS, userLocalRoot)
		for _, w := range ulWarnings {
			fmt.Printf("WARNING: user-local component: %v\n", w)
		}
	}

	resolved := ResolveComponents(embedded, catalogTier, userLocalTier)

	var matched []PolicyComponent
	for i := range resolved {
		if MatchComponent(&resolved[i], manifest) {
			matched = append(matched, resolved[i])
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Key < matched[j].Key
	})

	// Apply binary resolution based on assembly options.
	if opts.StripBinaries {
		matched = stripBinaries(matched)
	} else if opts.ProbeReport != nil {
		matched = applyProbeResults(matched, opts.ProbeReport)
	}

	networkPolicies := make(map[string]NetworkPolicy, len(matched))
	for _, comp := range matched {
		networkPolicies[comp.Key] = NetworkPolicy{
			Name:      comp.Name,
			Endpoints: comp.Endpoints,
			Binaries:  comp.Binaries,
		}
	}

	// Add allowed_domains from manifest.network. Skip entries whose hosts
	// are already covered by matched components (host-based dedup, not just
	// slug-based) to avoid creating unrestricted duplicates that bypass
	// binary restrictions from the probe step.
	if manifest.Network != nil {
		coveredHosts := make(map[string]bool)
		for _, np := range networkPolicies {
			for _, ep := range np.Endpoints {
				coveredHosts[ep.Host] = true
			}
		}

		resolver := network.NewResolver(nil)
		for _, entry := range manifest.Network.AllowedDomains {
			slug := slugify(entry)
			if _, exists := networkPolicies[slug]; exists {
				continue
			}

			if !strings.Contains(entry, ".") {
				// Group name: expand to actual domains.
				domains, err := resolver.ExpandGroup(entry)
				if err != nil {
					return nil, nil, fmt.Errorf("expanding domain group %q: %w", entry, err)
				}
				var endpoints []PolicyEndpoint
				for _, d := range domains {
					if strings.HasPrefix(d, ".") {
						continue
					}
					endpoints = append(endpoints, PolicyEndpoint{Host: d, Port: 443})
				}
				allCovered := len(endpoints) > 0
				for _, ep := range endpoints {
					if !coveredHosts[ep.Host] {
						allCovered = false
						break
					}
				}
				if allCovered {
					continue
				}
				networkPolicies[slug] = NetworkPolicy{
					Name:      entry,
					Endpoints: endpoints,
				}
			} else {
				if coveredHosts[entry] {
					continue
				}
				networkPolicies[slug] = NetworkPolicy{
					Name:      entry,
					Endpoints: []PolicyEndpoint{{Host: entry, Port: 443}},
				}
			}
		}
	}

	// Add generic credential endpoints.
	for _, cred := range manifest.Credentials {
		if cred.Type == "generic" {
			for _, ep := range cred.Endpoints {
				slug := slugify(ep.Host)
				networkPolicies["cred_"+slug] = NetworkPolicy{
					Name: ep.Host,
					Endpoints: []PolicyEndpoint{
						{Host: ep.Host, Port: ep.Port},
					},
				}
			}
		}
	}

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
		NetworkPolicies: networkPolicies,
	}, matched, nil
}

// AssemblePolicyFromDir is a convenience wrapper that loads catalog and
// user-local component tiers from filesystem directories. Pass empty strings
// to skip a tier.
func AssemblePolicyFromDir(manifest *Manifest, catalogDir string, userLocalDir string) (*PolicyFile, error) {
	var catalogFSys fs.FS
	var catalogRoot string
	if catalogDir != "" {
		if info, err := os.Stat(catalogDir); err == nil && info.IsDir() {
			catalogFSys = os.DirFS(catalogDir)
			catalogRoot = "."
		}
	}

	var userLocalFSys fs.FS
	var userLocalRoot string
	if userLocalDir != "" {
		if info, err := os.Stat(userLocalDir); err == nil && info.IsDir() {
			userLocalFSys = os.DirFS(userLocalDir)
			userLocalRoot = "."
		}
	}

	return AssemblePolicy(manifest, catalogFSys, catalogRoot, userLocalFSys, userLocalRoot)
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

// applyProbeResults populates the Binaries field on matched components using
// probe results and runtime globs from the component YAML. Components with
// explicit binaries (len > 0) are preserved unchanged.
func applyProbeResults(components []PolicyComponent, report *ProbeReport) []PolicyComponent {
	result := make([]PolicyComponent, len(components))
	for i, comp := range components {
		result[i] = comp

		if len(comp.Binaries) > 0 {
			continue
		}

		seen := make(map[string]bool)
		var paths []string

		if report != nil {
			if probeResults, ok := report.Results[comp.Key]; ok {
				for _, pr := range probeResults {
					if pr.Method != "not-found" && pr.Path != "" && !seen[pr.Path] {
						seen[pr.Path] = true
						paths = append(paths, pr.Path)
					}
				}
			}
		}

		for _, glob := range comp.RuntimeGlobs {
			if !seen[glob] {
				seen[glob] = true
				paths = append(paths, glob)
			}
		}

		if len(paths) > 0 {
			binaries := make([]PolicyBinary, len(paths))
			for j, p := range paths {
				binaries[j] = PolicyBinary{Path: p}
			}
			result[i].Binaries = binaries
		}
	}
	return result
}

// stripBinaries returns a shallow copy of components with Binaries normalized.
// Components with explicit binaries (from YAML) keep theirs per FR-011.
// Components without explicit binaries have Binaries set to nil, ensuring
// the first-pass policy has no binary restrictions.
func stripBinaries(components []PolicyComponent) []PolicyComponent {
	result := make([]PolicyComponent, len(components))
	for i, comp := range components {
		result[i] = comp
		if len(comp.Binaries) == 0 {
			result[i].Binaries = nil
		}
	}
	return result
}

// slugify converts a domain name to a YAML-safe key.
// Only dots are replaced with underscores; hyphens are preserved
// to avoid collisions between domains that differ only in dot/hyphen.
func slugify(domain string) string {
	return strings.ReplaceAll(domain, ".", "_")
}

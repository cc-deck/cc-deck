package build

import (
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cc-deck/cc-deck/internal/agent"
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
	resolver := network.NewResolver(nil)

	if manifest.Network != nil {
		coveredHosts := make(map[string]bool)
		for _, np := range networkPolicies {
			for _, ep := range np.Endpoints {
				coveredHosts[ep.Host] = true
			}
		}
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

	// Add per-agent domain groups. For each agent in the manifest, collect
	// domain groups from RequiredDomainGroups() and AllowedDomainsPerAgent,
	// then resolve them to endpoints. Groups are deduplicated and only
	// included when the declaring agent is in the manifest.
	{
		manifestAgents := manifest.EffectiveAgents()

		agentGroups := make(map[string][]string)
		for _, agentName := range manifestAgents {
			var groups []string
			// Declared groups from agent adapter.
			if a := agent.Get(agentName); a != nil {
				groups = append(groups, a.RequiredDomainGroups()...)
			}
			// User-configured per-agent groups.
			if manifest.Network != nil && manifest.Network.AllowedDomainsPerAgent != nil {
				groups = append(groups, manifest.Network.AllowedDomainsPerAgent[agentName]...)
			}
			if len(groups) > 0 {
				agentGroups[agentName] = groups
			}
		}

		// Validate and warn about missing groups.
		warnings := ValidateAgentDomainGroups(agentGroups, resolver)
		for _, w := range warnings {
			fmt.Printf("WARNING: %v\n", w)
		}

		// Resolve groups to endpoints, skipping those already covered.
		coveredHosts := make(map[string]bool)
		for _, np := range networkPolicies {
			for _, ep := range np.Endpoints {
				coveredHosts[ep.Host] = true
			}
		}

		// Deduplicate groups across all agents.
		processedGroups := make(map[string]bool)
		for _, agentName := range manifestAgents {
			for _, group := range agentGroups[agentName] {
				if processedGroups[group] {
					continue
				}
				processedGroups[group] = true

				domains, err := resolver.ExpandGroup(group)
				if err != nil {
					// Already warned about missing groups; skip silently.
					continue
				}
				var endpoints []PolicyEndpoint
				for _, d := range domains {
					if strings.HasPrefix(d, ".") {
						continue
					}
					if coveredHosts[d] {
						continue
					}
					endpoints = append(endpoints, PolicyEndpoint{Host: d, Port: 443})
					coveredHosts[d] = true
				}
				if len(endpoints) > 0 {
					slug := "agent_" + slugify(group)
					networkPolicies[slug] = NetworkPolicy{
						Name:      group,
						Endpoints: endpoints,
					}
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

	// Collect binaries from all agent-matched components (components where
	// Match.Agents is non-empty). These binaries are used for MCP endpoint
	// policies and pkg_node augmentation.
	agentBinaries := collectAgentBinaries(matched)

	// Add MCP endpoint entries.
	// Each MCP entry with a non-empty Endpoint generates a network policy
	// keyed as mcp_<slugified_name> using agent component binaries.
	hasMCPEndpoints := false
	if len(agentBinaries) > 0 {
		for _, mcp := range manifest.MCP {
			if mcp.Endpoint == "" {
				continue
			}
			host, port, err := parseMCPEndpoint(mcp.Endpoint)
			if err != nil {
				fmt.Printf("WARNING: MCP %q: %v, skipping\n", mcp.Name, err)
				continue
			}
			hasMCPEndpoints = true
			name := mcp.Description
			if name == "" {
				name = mcp.Name
			}
			key := "mcp_" + slugifyMCPName(mcp.Name)
			networkPolicies[key] = NetworkPolicy{
				Name: name,
				Endpoints: []PolicyEndpoint{
					{Host: host, Port: port},
				},
				Binaries: agentBinaries,
			}
		}
	} else if len(manifest.MCP) > 0 {
		// Check if any MCP entries have endpoints before warning
		for _, mcp := range manifest.MCP {
			if mcp.Endpoint != "" {
				fmt.Println("WARNING: no agent component found, skipping MCP policy entries")
				break
			}
		}
	}

	// Augment pkg_node binaries with agent binaries when MCP endpoints
	// exist. This allows agents to spawn npx processes that access the
	// npm registry for MCP stdio proxies.
	if hasMCPEndpoints {
		if pkgNode, ok := networkPolicies["pkg_node"]; ok && len(agentBinaries) > 0 {
			seen := make(map[string]bool)
			for _, b := range pkgNode.Binaries {
				seen[b.Path] = true
			}
			for _, b := range agentBinaries {
				if !seen[b.Path] {
					pkgNode.Binaries = append(pkgNode.Binaries, b)
					seen[b.Path] = true
				}
			}
			networkPolicies["pkg_node"] = pkgNode
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

// slugifyMCPName converts an MCP server name to a YAML-safe key.
// Hyphens, spaces, and non-alphanumeric characters are replaced with
// underscores and the result is lowercased. This is distinct from slugify()
// which only replaces dots (used for domain names).
var nonAlphanumRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func slugifyMCPName(name string) string {
	slug := nonAlphanumRe.ReplaceAllString(name, "_")
	slug = strings.Trim(slug, "_")
	return strings.ToLower(slug)
}

// collectAgentBinaries merges binaries from all agent-matched components
// (those with non-empty Match.Agents). Deduplicates by path while preserving
// insertion order (first occurrence wins). Components are processed in the
// order they appear in the matched slice (alphabetical by key).
func collectAgentBinaries(matched []PolicyComponent) []PolicyBinary {
	seen := make(map[string]bool)
	var binaries []PolicyBinary
	for _, comp := range matched {
		if len(comp.Match.Agents) == 0 {
			continue
		}
		for _, b := range comp.Binaries {
			if !seen[b.Path] {
				seen[b.Path] = true
				binaries = append(binaries, b)
			}
		}
	}
	return binaries
}

// ValidateAgentDomainGroups checks that domain groups declared by agents exist
// in the resolver. Returns warning messages for any missing groups. Missing
// groups are skipped without failing the build (per spec edge case).
func ValidateAgentDomainGroups(agentGroups map[string][]string, resolver *network.Resolver) []error {
	var warnings []error
	agentNames := make([]string, 0, len(agentGroups))
	for name := range agentGroups {
		agentNames = append(agentNames, name)
	}
	sort.Strings(agentNames)

	for _, agentName := range agentNames {
		for _, group := range agentGroups[agentName] {
			_, err := resolver.ExpandGroup(group)
			if err != nil {
				warnings = append(warnings, fmt.Errorf("agent %q declares domain group %q which is not available: %w", agentName, group, err))
			}
		}
	}
	return warnings
}

// parseMCPEndpoint splits a "host:port" string, validates both parts, and
// returns the host, port, and any error. The port must be a positive integer.
func parseMCPEndpoint(endpoint string) (string, int, error) {
	idx := strings.LastIndex(endpoint, ":")
	if idx < 0 {
		return "", 0, fmt.Errorf("missing port in endpoint %q", endpoint)
	}
	host := endpoint[:idx]
	portStr := endpoint[idx+1:]
	if host == "" {
		return "", 0, fmt.Errorf("empty host in endpoint %q", endpoint)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port %q in endpoint %q: %w", portStr, endpoint, err)
	}
	if port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("port %d out of range in endpoint %q", port, endpoint)
	}
	return host, port, nil
}

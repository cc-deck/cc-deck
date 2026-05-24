package build

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var validBinaryNameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func isValidBinaryName(name string) bool {
	return validBinaryNameRe.MatchString(name)
}

// PolicyComponent is a self-contained policy fragment loaded from a YAML file.
// Each component produces one entry in the output PolicyFile.network_policies map.
type PolicyComponent struct {
	Key           string           `yaml:"key"`
	Name          string           `yaml:"name"`
	Match         MatchCondition   `yaml:"match"`
	Endpoints     []PolicyEndpoint `yaml:"endpoints"`
	Binaries      []PolicyBinary   `yaml:"binaries,omitempty"`
	ProbeBinaries []string         `yaml:"probe_binaries,omitempty"`
	RuntimeGlobs  []string         `yaml:"runtime_globs,omitempty"`
}

// MatchCondition determines whether a component is included in the assembled policy.
// Evaluation uses OR across all fields: any single match includes the component.
type MatchCondition struct {
	Always      bool     `yaml:"always,omitempty"`
	Tools       []string `yaml:"tools,omitempty"`
	Credentials []string `yaml:"credentials,omitempty"`
	Features    []string `yaml:"features,omitempty"`
}

// LoadComponentsFromFS parses all YAML component files from an fs.FS.
// Invalid files are skipped with a warning logged via the returned errors slice.
func LoadComponentsFromFS(fsys fs.FS, root string) ([]PolicyComponent, []error) {
	var components []PolicyComponent
	var warnings []error

	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		return nil, []error{fmt.Errorf("reading component directory %s: %w", root, err)}
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.ToSlash(filepath.Join(root, entry.Name()))
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			warnings = append(warnings, fmt.Errorf("reading %s: %w", entry.Name(), err))
			continue
		}

		var comp PolicyComponent
		if err := yaml.Unmarshal(data, &comp); err != nil {
			warnings = append(warnings, fmt.Errorf("parsing %s: %w", entry.Name(), err))
			continue
		}

		if err := ValidateComponent(&comp, entry.Name()); err != nil {
			warnings = append(warnings, err)
			continue
		}

		components = append(components, comp)
	}

	return components, warnings
}

// ValidateComponent checks that a component has all required fields per
// the component file format contract.
func ValidateComponent(comp *PolicyComponent, filename string) error {
	if comp.Key == "" {
		return fmt.Errorf("validating %s: key is required", filename)
	}
	if comp.Name == "" {
		return fmt.Errorf("validating %s: name is required", filename)
	}
	if !comp.Match.Always && len(comp.Match.Tools) == 0 && len(comp.Match.Credentials) == 0 && len(comp.Match.Features) == 0 {
		return fmt.Errorf("validating %s: match must have at least one field set", filename)
	}
	if len(comp.Endpoints) == 0 {
		return fmt.Errorf("validating %s: endpoints must contain at least one entry", filename)
	}
	for _, pb := range comp.ProbeBinaries {
		if strings.ContainsAny(pb, `/\`) {
			return fmt.Errorf("validating %s: probe_binaries entry %q must not contain path separators", filename, pb)
		}
		if !isValidBinaryName(pb) {
			return fmt.Errorf("validating %s: probe_binaries entry %q contains invalid characters (only alphanumeric, dots, underscores, hyphens allowed)", filename, pb)
		}
	}
	for _, rg := range comp.RuntimeGlobs {
		if !strings.HasPrefix(rg, "/") {
			return fmt.Errorf("validating %s: runtime_globs entry %q must start with /", filename, rg)
		}
	}
	for i, ep := range comp.Endpoints {
		if ep.Host == "" {
			return fmt.Errorf("validating %s: endpoints[%d].host is required", filename, i)
		}
		if ep.Port == 0 {
			return fmt.Errorf("validating %s: endpoints[%d].port is required", filename, i)
		}
		if ep.Protocol == "rest" && ep.Access == "" && len(ep.Rules) == 0 {
			return fmt.Errorf("validating %s: endpoints[%d] (%s): protocol=rest requires access or rules", filename, i, ep.Host)
		}
	}
	return nil
}

// MatchComponent evaluates whether a component should be included based on
// the manifest's tools, credentials, and features. Uses OR semantics:
// any single field match includes the component.
func MatchComponent(comp *PolicyComponent, manifest *Manifest) bool {
	if comp.Match.Always {
		return true
	}

	for _, tool := range comp.Match.Tools {
		lower := strings.ToLower(tool)
		for _, mt := range manifest.Tools {
			if strings.Contains(strings.ToLower(mt.Name), lower) {
				return true
			}
		}
		for _, src := range manifest.Sources {
			for _, dt := range src.DetectedTools {
				if strings.Contains(strings.ToLower(dt), lower) {
					return true
				}
			}
		}
	}

	for _, cred := range comp.Match.Credentials {
		for _, mc := range manifest.Credentials {
			if mc.Type == cred {
				return true
			}
		}
	}

	// Features field reserved for future use; evaluate if present.
	// No manifest.Features field exists yet, so this is a no-op.

	return false
}

// ResolveComponents merges components from multiple tiers (embedded, catalog,
// user-local) using filename-stem precedence. Higher-precedence tiers replace
// lower-precedence components with the same filename stem entirely.
// Each tier is a map from filename stem to component.
func ResolveComponents(tiers ...map[string]PolicyComponent) []PolicyComponent {
	merged := make(map[string]PolicyComponent)

	for _, tier := range tiers {
		for stem, comp := range tier {
			merged[stem] = comp
		}
	}

	stems := make([]string, 0, len(merged))
	for stem := range merged {
		stems = append(stems, stem)
	}
	sort.Strings(stems)

	result := make([]PolicyComponent, 0, len(stems))
	for _, stem := range stems {
		result = append(result, merged[stem])
	}
	return result
}

// LoadComponentTier loads components from a single filesystem tier and returns
// them keyed by filename stem. When duplicate stems exist within a tier, the
// last file in alphabetical filename order wins. Single-pass implementation
// that reads each file exactly once.
func LoadComponentTier(fsys fs.FS, root string) (map[string]PolicyComponent, []error) {
	var warnings []error

	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		return nil, []error{fmt.Errorf("reading component directory %s: %w", root, err)}
	}

	var filenames []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			filenames = append(filenames, entry.Name())
		}
	}
	sort.Strings(filenames)

	result := make(map[string]PolicyComponent)
	for _, fname := range filenames {
		stem := strings.TrimSuffix(fname, ".yaml")
		path := filepath.ToSlash(filepath.Join(root, fname))
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			warnings = append(warnings, fmt.Errorf("reading %s: %w", fname, err))
			continue
		}
		var comp PolicyComponent
		if err := yaml.Unmarshal(data, &comp); err != nil {
			warnings = append(warnings, fmt.Errorf("parsing %s: %w", fname, err))
			continue
		}
		if err := ValidateComponent(&comp, fname); err != nil {
			warnings = append(warnings, err)
			continue
		}
		result[stem] = comp
	}

	return result, warnings
}

// LoadEmbeddedComponents returns components from the binary-embedded policies
// directory, keyed by filename stem.
func LoadEmbeddedComponents() (map[string]PolicyComponent, []error) {
	return LoadComponentTier(embeddedPolicies, "policies")
}

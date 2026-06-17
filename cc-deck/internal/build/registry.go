package build

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// BaseImageEntry describes a single base image in the registry.
type BaseImageEntry struct {
	Name    string `yaml:"name"`
	Ref     string `yaml:"ref"`
	Default bool   `yaml:"default,omitempty"`
}

// BaseImageRegistry is the top-level structure of base-images.yaml.
type BaseImageRegistry struct {
	OpenShell []BaseImageEntry `yaml:"openshell,omitempty"`
	Container []BaseImageEntry `yaml:"container,omitempty"`
}

// LoadBaseImageRegistry reads and parses a base-images.yaml file.
func LoadBaseImageRegistry(path string) (*BaseImageRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading base image registry: %w", err)
	}
	var reg BaseImageRegistry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parsing base image registry: %w", err)
	}
	return &reg, nil
}

// DefaultRef returns the ref of the default entry for the given target type.
// Returns an empty string if no default is set or the target is unknown.
func (r *BaseImageRegistry) DefaultRef(target string) string {
	for _, e := range r.EntriesForTarget(target) {
		if e.Default {
			return e.Ref
		}
	}
	return ""
}

// EntriesForTarget returns all entries for the given target type.
func (r *BaseImageRegistry) EntriesForTarget(target string) []BaseImageEntry {
	switch target {
	case "openshell":
		return r.OpenShell
	case "container":
		return r.Container
	default:
		return nil
	}
}

// LoadBaseImageRegistryOrEmbedded tries the on-disk path first, then falls
// back to the binary's embedded copy.
func LoadBaseImageRegistryOrEmbedded(path string) (*BaseImageRegistry, error) {
	if path != "" {
		if reg, err := LoadBaseImageRegistry(path); err == nil {
			return reg, nil
		}
	}
	return LoadEmbeddedBaseImageRegistry()
}

// LoadEmbeddedBaseImageRegistry parses the base-images.yaml embedded in the binary.
func LoadEmbeddedBaseImageRegistry() (*BaseImageRegistry, error) {
	data, err := EmbeddedBaseImagesYAML()
	if err != nil {
		return nil, fmt.Errorf("reading embedded base image registry: %w", err)
	}
	var reg BaseImageRegistry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parsing embedded base image registry: %w", err)
	}
	return &reg, nil
}

// ResolveDefaultBaseImage loads the registry from the given path (falling back
// to the embedded copy) and returns the default ref for the target type.
// Returns an empty string on any error.
func ResolveDefaultBaseImage(registryPath string, target string) string {
	reg, err := LoadBaseImageRegistryOrEmbedded(registryPath)
	if err != nil {
		return ""
	}
	return reg.DefaultRef(target)
}

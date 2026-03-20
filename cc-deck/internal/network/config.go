package network

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

// configFileName is the user domain groups config file name.
const configFileName = "domains.yaml"

// userConfigGroup represents a domain group as defined in the user's domains.yaml.
type userConfigGroup struct {
	Domains  []string `yaml:"domains,omitempty"`
	Extends  string   `yaml:"extends,omitempty"`
	Includes []string `yaml:"includes,omitempty"`
}

// LoadUserConfig loads user-defined domain groups from the XDG config file.
// Returns an empty map (not an error) if the file does not exist.
func LoadUserConfig() (map[string]userConfigGroup, error) {
	return LoadUserConfigFrom(UserConfigPath())
}

// LoadUserConfigFrom loads user-defined domain groups from a specific file path.
// Returns an empty map (not an error) if the file does not exist.
func LoadUserConfigFrom(path string) (map[string]userConfigGroup, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading domain config %s: %w", path, err)
	}

	var groups map[string]userConfigGroup
	if err := yaml.Unmarshal(data, &groups); err != nil {
		return nil, fmt.Errorf("parsing domain config %s: %w", path, err)
	}

	return groups, nil
}

// UserConfigPath returns the path to the user's domains.yaml config file.
func UserConfigPath() string {
	return filepath.Join(xdg.ConfigHome, "cc-deck", configFileName)
}

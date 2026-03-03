package config

import "fmt"

// BackendType represents the type of AI backend.
type BackendType string

const (
	BackendAnthropic BackendType = "anthropic"
	BackendVertex    BackendType = "vertex"
)

// Profile represents a credential and configuration profile.
type Profile struct {
	Backend           BackendType `yaml:"backend"`
	APIKeySecret      string      `yaml:"api_key_secret,omitempty"`
	Model             string      `yaml:"model,omitempty"`
	Permissions       string      `yaml:"permissions,omitempty"`
	Project           string      `yaml:"project,omitempty"`
	Region            string      `yaml:"region,omitempty"`
	CredentialsSecret string      `yaml:"credentials_secret,omitempty"`
	AllowedEgress     []string    `yaml:"allowed_egress,omitempty"`
}

// Validate checks that the profile has all required fields for its backend type.
func (p *Profile) Validate() error {
	switch p.Backend {
	case BackendAnthropic:
		if p.APIKeySecret == "" {
			return fmt.Errorf("anthropic profile requires api_key_secret")
		}
	case BackendVertex:
		if p.Project == "" {
			return fmt.Errorf("vertex profile requires project")
		}
		if p.Region == "" {
			return fmt.Errorf("vertex profile requires region")
		}
		if p.CredentialsSecret == "" {
			return fmt.Errorf("vertex profile requires credentials_secret")
		}
	default:
		return fmt.Errorf("unknown backend type: %q", p.Backend)
	}
	return nil
}

// AddProfile adds or replaces a profile in the config.
func (c *Config) AddProfile(name string, p Profile) error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("invalid profile %q: %w", name, err)
	}
	if c.Profiles == nil {
		c.Profiles = make(map[string]Profile)
	}
	c.Profiles[name] = p
	return nil
}

// GetProfile returns the profile with the given name.
func (c *Config) GetProfile(name string) (Profile, error) {
	p, ok := c.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("profile %q not found", name)
	}
	return p, nil
}

// DeleteProfile removes a profile from the config.
// Returns an error if the profile does not exist.
func (c *Config) DeleteProfile(name string) error {
	if _, ok := c.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	delete(c.Profiles, name)
	if c.DefaultProfile == name {
		c.DefaultProfile = ""
	}
	return nil
}

// SetDefaultProfile sets the default profile, validating it exists.
func (c *Config) SetDefaultProfile(name string) error {
	if _, ok := c.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	c.DefaultProfile = name
	return nil
}

// ListProfiles returns a sorted list of profile names.
func (c *Config) ListProfiles() []string {
	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}
	return names
}

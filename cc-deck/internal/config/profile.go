package config

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

// BackendType represents the type of AI backend.
type BackendType string

const (
	BackendAnthropic BackendType = "anthropic"
	BackendVertex    BackendType = "vertex"
)

// GitCredentialType represents the type of git credential mounting.
type GitCredentialType string

const (
	GitCredentialSSH   GitCredentialType = "ssh"
	GitCredentialToken GitCredentialType = "token"
)

// Profile represents a credential and configuration profile.
type Profile struct {
	Backend             BackendType       `yaml:"backend"`
	APIKeySecret        string            `yaml:"api_key_secret,omitempty"`
	Model               string            `yaml:"model,omitempty"`
	Permissions         string            `yaml:"permissions,omitempty"`
	Project             string            `yaml:"project,omitempty"`
	Region              string            `yaml:"region,omitempty"`
	CredentialsSecret   string            `yaml:"credentials_secret,omitempty"`
	AllowedEgress       []string          `yaml:"allowed_egress,omitempty"`
	GitCredentialType   GitCredentialType `yaml:"git_credential_type,omitempty"`
	GitCredentialSecret string            `yaml:"git_credential_secret,omitempty"`
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
		// credentials_secret is optional: Workload Identity can be used instead
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
	sort.Strings(names)
	return names
}

// PromptProfile interactively prompts the user for profile details.
// It reads from r and writes prompts to w.
func PromptProfile(r io.Reader, w io.Writer) (Profile, error) {
	scanner := bufio.NewScanner(r)
	p := Profile{}

	// Backend type
	fmt.Fprintf(w, "Backend type (anthropic/vertex): ")
	if !scanner.Scan() {
		return p, fmt.Errorf("reading backend type: %w", scannerErr(scanner))
	}
	backend := strings.TrimSpace(scanner.Text())
	switch strings.ToLower(backend) {
	case "anthropic":
		p.Backend = BackendAnthropic
	case "vertex":
		p.Backend = BackendVertex
	default:
		return p, fmt.Errorf("unknown backend type: %q (must be anthropic or vertex)", backend)
	}

	// Backend-specific fields
	switch p.Backend {
	case BackendAnthropic:
		fmt.Fprintf(w, "API key Secret name (K8s Secret containing 'api-key'): ")
		if !scanner.Scan() {
			return p, fmt.Errorf("reading api_key_secret: %w", scannerErr(scanner))
		}
		p.APIKeySecret = strings.TrimSpace(scanner.Text())
		if p.APIKeySecret == "" {
			return p, fmt.Errorf("api_key_secret is required for anthropic backend")
		}

	case BackendVertex:
		fmt.Fprintf(w, "GCP project ID: ")
		if !scanner.Scan() {
			return p, fmt.Errorf("reading project: %w", scannerErr(scanner))
		}
		p.Project = strings.TrimSpace(scanner.Text())
		if p.Project == "" {
			return p, fmt.Errorf("project is required for vertex backend")
		}

		fmt.Fprintf(w, "GCP region (e.g., us-central1): ")
		if !scanner.Scan() {
			return p, fmt.Errorf("reading region: %w", scannerErr(scanner))
		}
		p.Region = strings.TrimSpace(scanner.Text())
		if p.Region == "" {
			return p, fmt.Errorf("region is required for vertex backend")
		}

		fmt.Fprintf(w, "Credentials Secret name (leave empty for Workload Identity): ")
		if !scanner.Scan() {
			return p, fmt.Errorf("reading credentials_secret: %w", scannerErr(scanner))
		}
		p.CredentialsSecret = strings.TrimSpace(scanner.Text())
	}

	// Model (optional)
	fmt.Fprintf(w, "Model (leave empty for default): ")
	if scanner.Scan() {
		p.Model = strings.TrimSpace(scanner.Text())
	}

	return p, nil
}

func scannerErr(s *bufio.Scanner) error {
	if err := s.Err(); err != nil {
		return err
	}
	return fmt.Errorf("unexpected end of input")
}

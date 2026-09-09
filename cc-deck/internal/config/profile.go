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
	BackendOpenAI    BackendType = "openai"
)

// GitCredentialType represents the type of git credential mounting.
type GitCredentialType string

const (
	GitCredentialSSH   GitCredentialType = "ssh"
	GitCredentialToken GitCredentialType = "token"
)

// SourceKind identifies the origin of a credential value.
type SourceKind string

const (
	SourceEnv    SourceKind = "env"
	SourceFile   SourceKind = "file"
	SourceSecret SourceKind = "secret"
	SourceNone   SourceKind = "none"
)

// CredentialSource describes where a credential value comes from.
// Exactly one of Env, File or Secret must be set.
type CredentialSource struct {
	Env    string `yaml:"env,omitempty"`
	File   string `yaml:"file,omitempty"`
	Secret string `yaml:"secret,omitempty"`
}

// Kind returns the active source kind, or SourceNone if none is set.
func (cs *CredentialSource) Kind() SourceKind {
	if cs == nil {
		return SourceNone
	}
	switch {
	case cs.Env != "":
		return SourceEnv
	case cs.File != "":
		return SourceFile
	case cs.Secret != "":
		return SourceSecret
	default:
		return SourceNone
	}
}

// AuthConfig holds the authentication configuration for a profile.
type AuthConfig struct {
	APIKey      *CredentialSource `yaml:"api_key,omitempty"`
	Credentials *CredentialSource `yaml:"credentials,omitempty"`
	Login       bool              `yaml:"login,omitempty"`
}

// Profile represents a credential and configuration profile.
type Profile struct {
	// Existing fields
	Backend             BackendType       `yaml:"backend,omitempty"`
	APIKeySecret        string            `yaml:"api_key_secret,omitempty"`
	Model               string            `yaml:"model,omitempty"`
	Permissions         string            `yaml:"permissions,omitempty"`
	Project             string            `yaml:"project,omitempty"`
	Region              string            `yaml:"region,omitempty"`
	CredentialsSecret   string            `yaml:"credentials_secret,omitempty"`
	AllowedEgress       []string          `yaml:"allowed_egress,omitempty"`
	GitCredentialType   GitCredentialType `yaml:"git_credential_type,omitempty"`
	GitCredentialSecret string            `yaml:"git_credential_secret,omitempty"`

	// New fields
	Harness string            `yaml:"harness,omitempty"`
	Auth    *AuthConfig       `yaml:"auth,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
	Color   string            `yaml:"color,omitempty"`
	Icon    string            `yaml:"icon,omitempty"`
}

// KnownHarnesses lists every harness name the system supports.
var KnownHarnesses = []string{"claude", "codex", "opencode"}

// HarnessName returns the harness for this profile, defaulting to "claude".
func (p *Profile) HarnessName() string {
	if p.Harness != "" {
		return p.Harness
	}
	return "claude"
}

// harnessDefaultBackend returns the default backend for a harness.
func harnessDefaultBackend(harness string) BackendType {
	switch harness {
	case "claude":
		return BackendAnthropic
	case "codex":
		return BackendOpenAI
	case "opencode":
		return BackendOpenAI
	default:
		return BackendAnthropic
	}
}

// EffectiveBackend returns the declared backend or the per-harness default.
func (p *Profile) EffectiveBackend() BackendType {
	if p.Backend != "" {
		return p.Backend
	}
	return harnessDefaultBackend(p.HarnessName())
}

// EffectiveAuth merges legacy fields into an AuthConfig view without mutating
// the stored struct.
func (p *Profile) EffectiveAuth() AuthConfig {
	var ac AuthConfig
	if p.Auth != nil {
		ac = *p.Auth
	}
	// Map legacy api_key_secret -> auth.api_key.secret
	if p.APIKeySecret != "" && ac.APIKey == nil {
		ac.APIKey = &CredentialSource{Secret: p.APIKeySecret}
	}
	// Map legacy credentials_secret -> auth.credentials.secret
	if p.CredentialsSecret != "" && ac.Credentials == nil {
		ac.Credentials = &CredentialSource{Secret: p.CredentialsSecret}
	}
	return ac
}

// WrapperName returns the wrapper command name for a profile: the harness
// binary followed by a hyphen and the profile name (for example claude-work).
// Every place that derives a wrapper name from a profile uses this function.
func WrapperName(binary, name string) string {
	return binary + "-" + name
}

// Validate checks that the profile has all required fields for its backend type.
// This is the per-profile validation used by AddProfile. For cross-profile and
// extended rule checks, see validateProfiles in validate.go.
func (p *Profile) Validate() error {
	harness := p.HarnessName()
	backend := p.EffectiveBackend()
	auth := p.EffectiveAuth()

	// Legacy profiles: accept the old validation path
	if p.Auth == nil && p.Harness == "" {
		switch backend {
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
		default:
			return fmt.Errorf("unknown backend type: %q", p.Backend)
		}
		return nil
	}

	// New-style profiles
	switch backend {
	case BackendAnthropic:
		if auth.APIKey == nil && !auth.Login {
			if harness == "claude" {
				return fmt.Errorf("anthropic backend requires auth.api_key or auth.login")
			}
			return fmt.Errorf("anthropic backend requires auth.api_key")
		}
	case BackendVertex:
		if p.Project == "" {
			return fmt.Errorf("vertex profile requires project")
		}
		if p.Region == "" {
			return fmt.Errorf("vertex profile requires region")
		}
	case BackendOpenAI:
		if auth.APIKey == nil && !auth.Login {
			return fmt.Errorf("openai backend requires auth.api_key or auth.login")
		}
	default:
		return fmt.Errorf("unknown backend type: %q", backend)
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

	// Harness type
	fmt.Fprintf(w, "Harness (claude/codex/opencode) [claude]: ")
	if !scanner.Scan() {
		return p, fmt.Errorf("reading harness: %w", scannerErr(scanner))
	}
	harness := strings.TrimSpace(scanner.Text())
	if harness == "" {
		harness = "claude"
	}
	found := false
	for _, h := range KnownHarnesses {
		if h == harness {
			found = true
			break
		}
	}
	if !found {
		return p, fmt.Errorf("unknown harness: %q (must be claude, codex, or opencode)", harness)
	}
	p.Harness = harness

	// Backend type
	defaultBackend := string(harnessDefaultBackend(harness))
	fmt.Fprintf(w, "Backend type (%s) [%s]: ", backendsForHarness(harness), defaultBackend)
	if !scanner.Scan() {
		return p, fmt.Errorf("reading backend type: %w", scannerErr(scanner))
	}
	backend := strings.TrimSpace(scanner.Text())
	if backend == "" {
		backend = defaultBackend
	}
	p.Backend = BackendType(backend)

	// Auth source
	fmt.Fprintf(w, "Auth method (api-key-env/api-key-file/login) [api-key-env]: ")
	if !scanner.Scan() {
		return p, fmt.Errorf("reading auth method: %w", scannerErr(scanner))
	}
	authMethod := strings.TrimSpace(scanner.Text())
	if authMethod == "" {
		authMethod = "api-key-env"
	}

	switch authMethod {
	case "api-key-env":
		fmt.Fprintf(w, "Environment variable name: ")
		if !scanner.Scan() {
			return p, fmt.Errorf("reading env var name: %w", scannerErr(scanner))
		}
		envName := strings.TrimSpace(scanner.Text())
		if envName == "" {
			return p, fmt.Errorf("environment variable name is required")
		}
		p.Auth = &AuthConfig{APIKey: &CredentialSource{Env: envName}}
	case "api-key-file":
		fmt.Fprintf(w, "File path: ")
		if !scanner.Scan() {
			return p, fmt.Errorf("reading file path: %w", scannerErr(scanner))
		}
		filePath := strings.TrimSpace(scanner.Text())
		if filePath == "" {
			return p, fmt.Errorf("file path is required")
		}
		p.Auth = &AuthConfig{APIKey: &CredentialSource{File: filePath}}
	case "login":
		p.Auth = &AuthConfig{Login: true}
	default:
		return p, fmt.Errorf("unknown auth method: %q", authMethod)
	}

	// Vertex-specific fields
	if p.Backend == BackendVertex {
		fmt.Fprintf(w, "GCP project ID: ")
		if !scanner.Scan() {
			return p, fmt.Errorf("reading project: %w", scannerErr(scanner))
		}
		p.Project = strings.TrimSpace(scanner.Text())
		if p.Project == "" {
			return p, fmt.Errorf("project is required for vertex backend")
		}

		fmt.Fprintf(w, "GCP region (e.g., us-east5): ")
		if !scanner.Scan() {
			return p, fmt.Errorf("reading region: %w", scannerErr(scanner))
		}
		p.Region = strings.TrimSpace(scanner.Text())
		if p.Region == "" {
			return p, fmt.Errorf("region is required for vertex backend")
		}

		fmt.Fprintf(w, "Credentials file (leave empty for Workload Identity): ")
		if !scanner.Scan() {
			return p, fmt.Errorf("reading credentials file: %w", scannerErr(scanner))
		}
		credFile := strings.TrimSpace(scanner.Text())
		if credFile != "" {
			if p.Auth == nil {
				p.Auth = &AuthConfig{}
			}
			p.Auth.Credentials = &CredentialSource{File: credFile}
		}
	}

	// Model (optional)
	fmt.Fprintf(w, "Model (leave empty for default): ")
	if scanner.Scan() {
		p.Model = strings.TrimSpace(scanner.Text())
	}

	return p, nil
}

// backendsForHarness returns the display string of valid backends for a harness.
func backendsForHarness(harness string) string {
	switch harness {
	case "claude":
		return "anthropic/vertex"
	case "codex":
		return "openai"
	case "opencode":
		return "openai/anthropic"
	default:
		return "anthropic"
	}
}

func scannerErr(s *bufio.Scanner) error {
	if err := s.Err(); err != nil {
		return err
	}
	return fmt.Errorf("unexpected end of input")
}

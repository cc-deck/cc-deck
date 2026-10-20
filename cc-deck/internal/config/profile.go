package config

import (
	"bufio"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
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
	// OAuthToken is a long-lived Claude subscription token (claude setup-token).
	// Only env and file sources are allowed.
	OAuthToken *CredentialSource `yaml:"oauth_token,omitempty"`
	Login      bool              `yaml:"login,omitempty"`
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
	Harness string            `yaml:"-"`
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

	// New-style profiles. A subscription token is checked first so that a
	// token on the wrong harness reports the token, not a missing api_key.
	if auth.OAuthToken != nil && (harness != "claude" || backend != BackendAnthropic) {
		return fmt.Errorf("auth.oauth_token requires harness claude and backend anthropic")
	}

	switch backend {
	case BackendAnthropic:
		if auth.APIKey == nil && auth.OAuthToken == nil && !auth.Login {
			if harness == "claude" {
				return fmt.Errorf("anthropic backend requires auth.api_key, auth.oauth_token or auth.login")
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

// ProfileKey is the exact identity of a harness profile.
type ProfileKey struct{ Harness, Name string }

func (k ProfileKey) String() string { return k.Harness + "/" + k.Name }

// ProfileGroups is the sole persisted profile schema: harness, then short name.
type ProfileGroups map[string]map[string]Profile

// ProfileGroupsFromFlat converts an in-memory legacy profile set. It does not
// participate in loading config files, which only accept the nested schema.
func ProfileGroupsFromFlat(flat map[string]Profile) ProfileGroups {
	out := make(ProfileGroups)
	for name, p := range flat {
		h := p.HarnessName()
		if out[h] == nil {
			out[h] = make(map[string]Profile)
		}
		out[h][name] = p
	}
	return out
}

func (groups *ProfileGroups) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("profiles must group entries by harness: profiles.<harness>.<name>")
	}
	out := make(ProfileGroups)
	for i := 0; i < len(node.Content); i += 2 {
		h := node.Content[i].Value
		if !knownHarness(h) {
			return fmt.Errorf("profiles.%s: unknown harness or legacy flat profile; expected profiles.<harness>.<name> (claude, codex, opencode)", h)
		}
		if _, exists := out[h]; exists {
			return fmt.Errorf("duplicate harness group %q", h)
		}
		group := node.Content[i+1]
		if group.Kind != yaml.MappingNode {
			return fmt.Errorf("profiles.%s must map profile names to definitions", h)
		}
		out[h] = make(map[string]Profile)
		for j := 0; j < len(group.Content); j += 2 {
			name := group.Content[j].Value
			if _, exists := out[h][name]; exists {
				return fmt.Errorf("duplicate profile %q", ProfileKey{h, name})
			}
			body := group.Content[j+1]
			if body.Kind != yaml.MappingNode {
				return fmt.Errorf("profile %q must be a mapping", ProfileKey{h, name})
			}
			key := ProfileKey{h, name}
			if err := validateProfileYAMLFields(body, reflect.TypeOf(Profile{}), key, ""); err != nil {
				return err
			}
			var p Profile
			if err := body.Decode(&p); err != nil {
				return fmt.Errorf("profile %q: %w", ProfileKey{h, name}, err)
			}
			p.Harness = h
			out[h][name] = p
		}
	}
	*groups = out
	return nil
}

func validateProfileYAMLFields(node *yaml.Node, typ reflect.Type, key ProfileKey, prefix string) error {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	switch typ.Kind() {
	case reflect.Struct:
		if node.Kind != yaml.MappingNode {
			return nil // The typed decoder below reports a shape mismatch.
		}
		fields := make(map[string]reflect.Type, typ.NumField())
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			tag := strings.Split(field.Tag.Get("yaml"), ",")[0]
			if tag != "" && tag != "-" {
				fields[tag] = field.Type
			}
		}
		seen := make(map[string]struct{}, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			name := node.Content[i].Value
			fieldPath := name
			if prefix != "" {
				fieldPath = prefix + "." + name
			}
			if _, exists := seen[name]; exists {
				return fmt.Errorf("profile %q: duplicate field %q", key, fieldPath)
			}
			seen[name] = struct{}{}
			if prefix == "" && name == "harness" {
				return fmt.Errorf("profile %q: remove inner harness; use profiles.%s.%s", key, key.Harness, key.Name)
			}
			fieldType, ok := fields[name]
			if !ok {
				return fmt.Errorf("profile %q: unknown field %q", key, fieldPath)
			}
			if err := validateProfileYAMLFields(node.Content[i+1], fieldType, key, fieldPath); err != nil {
				return err
			}
		}
	case reflect.Map:
		if node.Kind != yaml.MappingNode {
			return nil
		}
		seen := make(map[string]struct{}, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			name := node.Content[i].Value
			if _, exists := seen[name]; exists {
				fieldPath := name
				if prefix != "" {
					fieldPath = prefix + "." + name
				}
				return fmt.Errorf("profile %q: duplicate field %q", key, fieldPath)
			}
			seen[name] = struct{}{}
			if err := validateProfileYAMLFields(node.Content[i+1], typ.Elem(), key, prefix); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		if node.Kind != yaml.SequenceNode {
			return nil
		}
		for _, item := range node.Content {
			if err := validateProfileYAMLFields(item, typ.Elem(), key, prefix); err != nil {
				return err
			}
		}
	}
	return nil
}

func knownHarness(h string) bool {
	for _, candidate := range KnownHarnesses {
		if h == candidate {
			return true
		}
	}
	return false
}

func validKey(k ProfileKey) error {
	if !knownHarness(k.Harness) {
		return fmt.Errorf("unknown harness %q", k.Harness)
	}
	if !profileNameRegex.MatchString(k.Name) {
		return fmt.Errorf("invalid profile name %q", k.Name)
	}
	return nil
}

func (c *Config) ListProfileKeys() []ProfileKey {
	keys := []ProfileKey{}
	for h, group := range c.Profiles {
		for name := range group {
			keys = append(keys, ProfileKey{h, name})
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	return keys
}

func (c *Config) GetProfileFor(harness, name string) (Profile, error) {
	if group := c.Profiles[harness]; group != nil {
		if p, ok := group[name]; ok {
			p.Harness = harness
			return p, nil
		}
	}
	return Profile{}, fmt.Errorf("profile %q not found", ProfileKey{harness, name})
}

func (c *Config) ResolveProfileRef(ref string) (ProfileKey, error) {
	if strings.Contains(ref, "/") {
		parts := strings.Split(ref, "/")
		if len(parts) != 2 {
			return ProfileKey{}, fmt.Errorf("invalid profile selector %q; use <harness>/<name>", ref)
		}
		k := ProfileKey{parts[0], parts[1]}
		if err := validKey(k); err != nil {
			return ProfileKey{}, err
		}
		if _, err := c.GetProfileFor(k.Harness, k.Name); err != nil {
			return ProfileKey{}, err
		}
		return k, nil
	}
	if !profileNameRegex.MatchString(ref) {
		return ProfileKey{}, fmt.Errorf("invalid profile selector %q; use <harness>/<name>", ref)
	}
	matches := []ProfileKey{}
	for _, k := range c.ListProfileKeys() {
		if k.Name == ref {
			matches = append(matches, k)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return ProfileKey{}, fmt.Errorf("profile %q not found", ref)
	}
	choices := make([]string, len(matches))
	for i, k := range matches {
		choices[i] = k.String()
	}
	return ProfileKey{}, fmt.Errorf("profile %q is ambiguous; use %s", ref, strings.Join(choices, " or "))
}

// AddProfile rejects an existing pair, including when the other harness has the same name.
func (c *Config) AddProfile(key ProfileKey, p Profile) error {
	if err := validKey(key); err != nil {
		return err
	}
	if p.Harness != "" && p.Harness != key.Harness {
		return fmt.Errorf("profile %q has inconsistent harness %q", key, p.Harness)
	}
	p.Harness = key.Harness
	if err := p.Validate(); err != nil {
		return fmt.Errorf("invalid profile %q: %w", key, err)
	}
	if c.Profiles == nil {
		c.Profiles = make(ProfileGroups)
	}
	if c.Profiles[key.Harness] == nil {
		c.Profiles[key.Harness] = make(map[string]Profile)
	}
	if _, exists := c.Profiles[key.Harness][key.Name]; exists {
		return fmt.Errorf("profile %q already exists", key)
	}
	c.Profiles[key.Harness][key.Name] = p
	return nil
}

// GetProfile resolves a qualified or unambiguous short name.
func (c *Config) GetProfile(ref string) (Profile, error) {
	k, err := c.ResolveProfileRef(ref)
	if err != nil {
		return Profile{}, err
	}
	return c.GetProfileFor(k.Harness, k.Name)
}

// DeleteProfile removes a profile from the config.
// Returns an error if the profile does not exist.
func (c *Config) DeleteProfile(key ProfileKey) error {
	if _, err := c.GetProfileFor(key.Harness, key.Name); err != nil {
		return err
	}
	defaultKey, defaultErr := c.ResolveProfileRef(c.DefaultProfile)
	delete(c.Profiles[key.Harness], key.Name)
	if len(c.Profiles[key.Harness]) == 0 {
		delete(c.Profiles, key.Harness)
	}
	if c.DefaultProfile == key.String() || (defaultErr == nil && defaultKey == key) {
		c.DefaultProfile = ""
	}
	return nil
}

// SetDefaultProfile sets the default profile, validating it exists.
func (c *Config) SetDefaultProfile(ref string) error {
	k, err := c.ResolveProfileRef(ref)
	if err != nil {
		return err
	}
	c.DefaultProfile = k.String()
	return nil
}

// ListProfiles returns a sorted list of profile names.
func (c *Config) ListProfiles() []string {
	names := []string{}
	for _, k := range c.ListProfileKeys() {
		names = append(names, k.String())
	}
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

	// Vertex authenticates via ADC (Workload Identity or a credentials file),
	// resolved in the vertex block below, so it skips the api-key/oauth/login
	// prompt. Offering it would force an answer that Validate then rejects.
	if p.Backend != BackendVertex {
		// Auth source. Claude also offers a subscription token (claude setup-token).
		authMethods := "api-key-env/api-key-file/login"
		if harness == "claude" {
			authMethods = "api-key-env/api-key-file/oauth-token-env/oauth-token-file/login"
		}
		fmt.Fprintf(w, "Auth method (%s) [api-key-env]: ", authMethods)
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
		case "oauth-token-env", "oauth-token-file":
			if harness != "claude" {
				return p, fmt.Errorf("unknown auth method: %q", authMethod)
			}
			src := &CredentialSource{}
			if authMethod == "oauth-token-env" {
				fmt.Fprintf(w, "Environment variable holding the subscription token: ")
			} else {
				fmt.Fprintf(w, "File holding the subscription token: ")
			}
			if !scanner.Scan() {
				return p, fmt.Errorf("reading token source: %w", scannerErr(scanner))
			}
			ref := strings.TrimSpace(scanner.Text())
			if ref == "" {
				return p, fmt.Errorf("token source is required")
			}
			if authMethod == "oauth-token-env" {
				src.Env = ref
			} else {
				src.File = ref
			}
			p.Auth = &AuthConfig{OAuthToken: src}
		case "login":
			p.Auth = &AuthConfig{Login: true}
		default:
			return p, fmt.Errorf("unknown auth method: %q", authMethod)
		}
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

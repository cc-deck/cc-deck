package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

const (
	configDirName = "cc-deck"
	configFile    = "config.yaml"
)

// Config represents the top-level cc-deck configuration.
type Config struct {
	DefaultProfile string            `yaml:"default_profile,omitempty"`
	Defaults       Defaults          `yaml:"defaults,omitempty"`
	Profiles       map[string]Profile `yaml:"profiles,omitempty"`
	Sessions       []Session         `yaml:"sessions,omitempty"`
}

// Defaults holds default values for sessions.
type Defaults struct {
	Namespace   string `yaml:"namespace,omitempty"`
	StorageSize string `yaml:"storage_size,omitempty"`
	Image       string `yaml:"image,omitempty"`
	ImageTag    string `yaml:"image_tag,omitempty"`
}

// Session represents a running or previously deployed Claude Code session.
type Session struct {
	Name       string     `yaml:"name"`
	Namespace  string     `yaml:"namespace,omitempty"`
	Profile    string     `yaml:"profile,omitempty"`
	Status     string     `yaml:"status,omitempty"`
	PodName    string     `yaml:"pod_name,omitempty"`
	Connection Connection `yaml:"connection,omitempty"`
	CreatedAt  string     `yaml:"created_at,omitempty"`
	SyncDir    string     `yaml:"sync_dir,omitempty"`
}

// Connection holds connection details for a session.
type Connection struct {
	ExecTarget string `yaml:"exec_target,omitempty"`
	WebURL     string `yaml:"web_url,omitempty"`
	WebPort    int    `yaml:"web_port,omitempty"`
	Method     string `yaml:"method,omitempty"`
}

// DefaultConfigPath returns the XDG-compliant config file path.
func DefaultConfigPath() string {
	return filepath.Join(xdg.ConfigHome, configDirName, configFile)
}

// Load reads and parses the config file at the given path.
// If path is empty, the default XDG path is used.
// Returns a zero-value Config (not an error) if the file does not exist.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}

// Save writes the config to the given path.
// If path is empty, the default XDG path is used.
// Parent directories are created as needed.
func (c *Config) Save(path string) error {
	if path == "" {
		path = DefaultConfigPath()
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// AddSession adds a session record to the config.
func (c *Config) AddSession(s Session) {
	if s.CreatedAt == "" {
		s.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	c.Sessions = append(c.Sessions, s)
}

// RemoveSession removes a session by name from the config.
// Returns true if the session was found and removed.
func (c *Config) RemoveSession(name string) bool {
	for i, s := range c.Sessions {
		if s.Name == name {
			c.Sessions = append(c.Sessions[:i], c.Sessions[i+1:]...)
			return true
		}
	}
	return false
}

// FindSession returns a pointer to the session with the given name, or nil.
func (c *Config) FindSession(name string) *Session {
	for i := range c.Sessions {
		if c.Sessions[i].Name == name {
			return &c.Sessions[i]
		}
	}
	return nil
}

// ResolveProfile returns the profile name to use, preferring the flag override,
// then the config default.
func (c *Config) ResolveProfile(flagProfile string) string {
	if flagProfile != "" {
		return flagProfile
	}
	return c.DefaultProfile
}

package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/cc-deck/cc-deck/internal/config"
)

// opencodeTranslator implements Translator for the OpenCode harness.
type opencodeTranslator struct{}

func init() {
	Register(&opencodeTranslator{})
}

func (c *opencodeTranslator) Harness() string     { return "opencode" }
func (c *opencodeTranslator) ConfigDirEnv() string { return "OPENCODE_CONFIG" }

func (c *opencodeTranslator) IsolatedEntries() []string {
	return []string{"opencode.json"}
}

func (c *opencodeTranslator) SupportsLogin() bool { return false }

func (c *opencodeTranslator) Backends() []config.BackendType {
	return []config.BackendType{config.BackendOpenAI, config.BackendAnthropic}
}

func (c *opencodeTranslator) DefaultConfigSubdir() string { return ".config/opencode" }

func (c *opencodeTranslator) Render(rp ResolvedProfile) (WrapperScript, error) {
	if rp.Login {
		return WrapperScript{}, fmt.Errorf("profile %q: login is not supported for %s", rp.Name, c.Harness())
	}
	if rp.APIKey != nil && rp.APIKey.Kind() == config.SourceSecret {
		return WrapperScript{}, fmt.Errorf("profile %q: secret source is not renderable outside Kubernetes", rp.Name)
	}

	lines := wrapperLines{
		// OPENCODE_CONFIG points at the generated JSON file, not a directory.
		ConfigDirLine: fmt.Sprintf("export OPENCODE_CONFIG=\"$HOME/.local/share/cc-deck/profiles/%s/opencode/opencode.json\"", rp.Name),
		ExecLine:      fmt.Sprintf("exec %s \"$@\"", rp.Harness.Binary()),
	}

	// No ModelLine: the model is written into opencode.json by
	// PrepareConfigDir instead of exported as an env var.

	// User env entries, sorted by key.
	if len(rp.Env) > 0 {
		keys := make([]string, 0, len(rp.Env))
		for k := range rp.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			lines.EnvLines = append(lines.EnvLines, fmt.Sprintf("export %s=%s", k, shQuote(rp.Env[k])))
		}
	}

	// Credential handling: map APIKey to the backend-specific env var.
	if rp.APIKey != nil {
		envVar := opencodeAPIKeyEnv(rp.Backend)
		switch rp.APIKey.Kind() {
		case config.SourceEnv:
			lines.CredentialChecks = append(lines.CredentialChecks, envCheckBlock(rp.Name, rp.APIKey.Env))
			lines.CredentialExports = append(lines.CredentialExports, fmt.Sprintf("export %s=\"$%s\"", envVar, rp.APIKey.Env))
		case config.SourceFile:
			varRef := fmt.Sprintf("$HOME/.config/cc-deck/profiles/%s/api_key", rp.Name)
			lines.CredentialChecks = append(lines.CredentialChecks,
				fmt.Sprintf("_f=\"%s\"", varRef),
			)
			lines.CredentialChecks = append(lines.CredentialChecks, fileCheckBlock(rp.Name, "$_f"))
			lines.CredentialExports = append(lines.CredentialExports, fmt.Sprintf("export %s=\"$(cat \"$_f\")\"", envVar))
		}
	}

	return renderWrapper(rp, lines)
}

func (c *opencodeTranslator) PrepareConfigDir(rp ResolvedProfile, defaultDir string) ([]string, error) {
	warnings, err := PrepareSharedDir(rp.ConfigDir, defaultDir, c.IsolatedEntries())
	if err != nil {
		return warnings, fmt.Errorf("profile %q: prepare config dir: %w", rp.Name, err)
	}

	// Build the opencode.json config. Start from the default if it exists,
	// then overlay the model field.
	cfg := make(map[string]any)

	defaultConfig := filepath.Join(defaultDir, "opencode.json")
	if data, readErr := os.ReadFile(defaultConfig); readErr == nil {
		// Best-effort parse; ignore malformed defaults.
		_ = json.Unmarshal(data, &cfg)
	}

	// Ensure the $schema key is present.
	if _, ok := cfg["$schema"]; !ok {
		cfg["$schema"] = "https://opencode.ai/config.json"
	}

	// Set the model in <provider>/<model> format when specified.
	if rp.Model != "" {
		cfg["model"] = opencodeModelValue(rp.Backend, rp.Model)
	}

	encoded, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return warnings, fmt.Errorf("profile %q: marshal opencode.json: %w", rp.Name, err)
	}
	encoded = append(encoded, '\n')

	configPath := filepath.Join(rp.ConfigDir, "opencode.json")
	if err := os.WriteFile(configPath, encoded, 0644); err != nil {
		return warnings, fmt.Errorf("profile %q: write opencode.json: %w", rp.Name, err)
	}

	return warnings, nil
}

func (c *opencodeTranslator) ProviderType(backend config.BackendType) string {
	switch backend {
	case config.BackendOpenAI:
		return "openai"
	case config.BackendAnthropic:
		return "claude"
	default:
		return ""
	}
}

// opencodeModelValue formats the model string for opencode.json.
// OpenAI models use the bare name; Anthropic models get an "anthropic/" prefix.
func opencodeModelValue(backend config.BackendType, model string) string {
	switch backend {
	case config.BackendAnthropic:
		return "anthropic/" + model
	default:
		// OpenAI and any future backends use the bare model name.
		return model
	}
}

// opencodeAPIKeyEnv returns the environment variable name for the API key
// based on the backend type.
func opencodeAPIKeyEnv(backend config.BackendType) string {
	switch backend {
	case config.BackendAnthropic:
		return "ANTHROPIC_API_KEY"
	default:
		return "OPENAI_API_KEY"
	}
}

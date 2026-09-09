package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cc-deck/cc-deck/internal/config"
)

// codexTranslator implements Translator for the Codex harness.
type codexTranslator struct{}

func init() {
	Register(&codexTranslator{})
}

func (c *codexTranslator) Harness() string      { return "codex" }
func (c *codexTranslator) ConfigDirEnv() string { return "CODEX_HOME" }

func (c *codexTranslator) IsolatedEntries() []string {
	return []string{"auth.json", "config.toml", "sessions"}
}

func (c *codexTranslator) SupportsLogin() bool { return false }

func (c *codexTranslator) Backends() []config.BackendType {
	return []config.BackendType{config.BackendOpenAI}
}

func (c *codexTranslator) DefaultConfigSubdir() string { return ".codex" }

func (c *codexTranslator) Render(rp ResolvedProfile) (WrapperScript, error) {
	if rp.Login {
		return WrapperScript{}, fmt.Errorf("profile %q: login is not supported for %s", rp.Name, c.Harness())
	}
	if rp.APIKey != nil && rp.APIKey.Kind() == config.SourceSecret {
		return WrapperScript{}, fmt.Errorf("profile %q: secret source is not renderable outside Kubernetes", rp.Name)
	}

	lines := wrapperLines{
		ConfigDirLine: fmt.Sprintf("export CODEX_HOME=\"$HOME/.local/share/cc-deck/profiles/%s/codex\"", rp.Name),
		ExecLine:      fmt.Sprintf("exec %s \"$@\"", rp.Harness.Binary()),
	}

	// No model export line for Codex; model is written to config.toml
	// by PrepareConfigDir instead.

	// User env entries, sorted by key
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

	// Credential handling
	if rp.APIKey != nil {
		switch rp.APIKey.Kind() {
		case config.SourceEnv:
			lines.CredentialChecks = append(lines.CredentialChecks, envCheckBlock(rp.Name, rp.APIKey.Env))
			lines.CredentialExports = append(lines.CredentialExports, fmt.Sprintf("export OPENAI_API_KEY=\"$%s\"", rp.APIKey.Env))
		case config.SourceFile:
			varRef := fmt.Sprintf("$HOME/.config/cc-deck/profiles/%s/api_key", rp.Name)
			lines.CredentialChecks = append(lines.CredentialChecks,
				fmt.Sprintf("_f=\"%s\"", varRef),
			)
			lines.CredentialChecks = append(lines.CredentialChecks, fileCheckBlock(rp.Name, "$_f"))
			lines.CredentialExports = append(lines.CredentialExports, "export OPENAI_API_KEY=\"$(cat \"$_f\")\"")
		}
	}

	return renderWrapper(rp, lines)
}

func (c *codexTranslator) PrepareConfigDir(rp ResolvedProfile, defaultDir string) ([]string, error) {
	warnings, err := PrepareSharedDir(rp.ConfigDir, defaultDir, c.IsolatedEntries())
	if err != nil {
		return warnings, fmt.Errorf("profile %q: prepare config dir: %w", rp.Name, err)
	}

	// config.toml is isolated (it carries the per-profile model), so the
	// user's default settings are carried over by overlaying the model on a
	// copy of the default file. The file is always rewritten so that a model
	// removed from the profile does not linger from an earlier sync.
	configPath := filepath.Join(rp.ConfigDir, "config.toml")
	content := overlayCodexModel(readDefaultCodexConfig(defaultDir), rp.Model)
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		return warnings, fmt.Errorf("profile %q: write config.toml: %w", rp.Name, err)
	}

	return warnings, nil
}

// readDefaultCodexConfig returns the content of the default config.toml, or
// an empty string when there is none.
func readDefaultCodexConfig(defaultDir string) string {
	if defaultDir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(defaultDir, "config.toml"))
	if err != nil {
		return ""
	}
	return string(data)
}

// overlayCodexModel drops any top-level "model = ..." line from base and, when
// model is non-empty, prepends the profile's model line. Top-level means a
// line that appears before the first [table] header, which is where Codex
// reads the model key from.
func overlayCodexModel(base, model string) string {
	var out []string
	inTable := false
	for _, line := range strings.Split(strings.TrimRight(base, "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inTable = true
		}
		if !inTable && strings.HasPrefix(trimmed, "model") {
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "model"))
			if strings.HasPrefix(rest, "=") {
				continue
			}
		}
		if line != "" || len(out) > 0 {
			out = append(out, line)
		}
	}
	var b strings.Builder
	if model != "" {
		fmt.Fprintf(&b, "model = %q\n", model)
	}
	if len(out) > 0 {
		b.WriteString(strings.Join(out, "\n"))
		b.WriteString("\n")
	}
	return b.String()
}

func (c *codexTranslator) ProviderType(_ config.BackendType) string {
	return "openai"
}

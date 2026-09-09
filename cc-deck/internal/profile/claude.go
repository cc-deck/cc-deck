package profile

import (
	"fmt"
	"sort"

	"github.com/cc-deck/cc-deck/internal/config"
)

// claudeTranslator implements Translator for the Claude Code harness.
type claudeTranslator struct{}

func init() {
	Register(&claudeTranslator{})
}

func (c *claudeTranslator) Harness() string     { return "claude" }
func (c *claudeTranslator) ConfigDirEnv() string { return "CLAUDE_CONFIG_DIR" }

func (c *claudeTranslator) IsolatedEntries() []string {
	return []string{".credentials.json", "statsig"}
}

func (c *claudeTranslator) SupportsLogin() bool { return true }

func (c *claudeTranslator) Backends() []config.BackendType {
	return []config.BackendType{config.BackendAnthropic, config.BackendVertex}
}

func (c *claudeTranslator) Render(rp ResolvedProfile) (WrapperScript, error) {
	if rp.APIKey != nil && rp.APIKey.Kind() == config.SourceSecret {
		return WrapperScript{}, fmt.Errorf("profile %q: secret source is not renderable outside Kubernetes", rp.Name)
	}
	if rp.Credentials != nil && rp.Credentials.Kind() == config.SourceSecret {
		return WrapperScript{}, fmt.Errorf("profile %q: secret source is not renderable outside Kubernetes", rp.Name)
	}

	lines := wrapperLines{
		ConfigDirLine: fmt.Sprintf("export CLAUDE_CONFIG_DIR=\"$HOME/.local/share/cc-deck/profiles/%s/claude\"", rp.Name),
		ExecLine:      fmt.Sprintf("exec %s \"$@\"", rp.Harness.Binary()),
	}

	// Model line
	if rp.Model != "" {
		lines.ModelLine = fmt.Sprintf("export ANTHROPIC_MODEL=%s", shQuote(rp.Model))
	}

	// Backend-specific lines
	switch rp.Backend {
	case config.BackendVertex:
		lines.BackendLines = append(lines.BackendLines, "export CLAUDE_CODE_USE_VERTEX=1")
		if rp.Project != "" {
			lines.BackendLines = append(lines.BackendLines, fmt.Sprintf("export ANTHROPIC_VERTEX_PROJECT_ID=%s", shQuote(rp.Project)))
		}
		if rp.Region != "" {
			lines.BackendLines = append(lines.BackendLines, fmt.Sprintf("export CLOUD_ML_REGION=%s", shQuote(rp.Region)))
		}
	case config.BackendAnthropic:
		// No extra backend lines for direct Anthropic.
	}

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
	if rp.Login {
		// Login profile: no credentials needed.
		return renderWrapper(rp, lines)
	}

	if rp.APIKey != nil {
		switch rp.APIKey.Kind() {
		case config.SourceEnv:
			lines.CredentialChecks = append(lines.CredentialChecks, envCheckBlock(rp.Name, rp.APIKey.Env))
			lines.CredentialExports = append(lines.CredentialExports, fmt.Sprintf("export ANTHROPIC_API_KEY=\"$%s\"", rp.APIKey.Env))
		case config.SourceFile:
			varRef := fmt.Sprintf("$HOME/.config/cc-deck/profiles/%s/api_key", rp.Name)
			lines.CredentialChecks = append(lines.CredentialChecks,
				fmt.Sprintf("_f=\"%s\"", varRef),
			)
			lines.CredentialChecks = append(lines.CredentialChecks, fileCheckBlock(rp.Name, "$_f"))
			lines.CredentialExports = append(lines.CredentialExports, "export ANTHROPIC_API_KEY=\"$(cat \"$_f\")\"")
		}
	}

	if rp.Credentials != nil {
		switch rp.Credentials.Kind() {
		case config.SourceEnv:
			lines.CredentialChecks = append(lines.CredentialChecks, envCheckBlock(rp.Name, rp.Credentials.Env))
			lines.CredentialExports = append(lines.CredentialExports, fmt.Sprintf("export GOOGLE_APPLICATION_CREDENTIALS=\"$%s\"", rp.Credentials.Env))
		case config.SourceFile:
			varRef := fmt.Sprintf("$HOME/.config/cc-deck/profiles/%s/credentials", rp.Name)
			lines.CredentialChecks = append(lines.CredentialChecks,
				fmt.Sprintf("_f=\"%s\"", varRef),
			)
			lines.CredentialChecks = append(lines.CredentialChecks, fileCheckBlock(rp.Name, "$_f"))
			lines.CredentialExports = append(lines.CredentialExports, "export GOOGLE_APPLICATION_CREDENTIALS=\"$_f\"")
		}
	}

	return renderWrapper(rp, lines)
}

func (c *claudeTranslator) PrepareConfigDir(rp ResolvedProfile, defaultDir string) error {
	warnings, err := PrepareSharedDir(rp.ConfigDir, defaultDir, c.IsolatedEntries())
	if err != nil {
		return fmt.Errorf("profile %q: prepare config dir: %w", rp.Name, err)
	}
	_ = warnings // Warnings are collected by the caller via SyncResult.
	return nil
}

func (c *claudeTranslator) ProviderType(backend config.BackendType) string {
	switch backend {
	case config.BackendAnthropic:
		return "claude"
	case config.BackendVertex:
		return "google-cloud"
	default:
		return ""
	}
}

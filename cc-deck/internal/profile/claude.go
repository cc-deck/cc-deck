package profile

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/cc-deck/cc-deck/internal/config"
)

// claudeTranslator implements Translator for the Claude Code harness.
type claudeTranslator struct{}

func init() {
	Register(&claudeTranslator{})
}

func (c *claudeTranslator) Harness() string      { return "claude" }
func (c *claudeTranslator) ConfigDirEnv() string { return "CLAUDE_CONFIG_DIR" }

// IsolatedEntries lists the per-profile entries of CLAUDE_CONFIG_DIR.
// .claude.json holds the account next to user configuration, so it is never
// linked; PrepareConfigDir merges the configuration part into it instead.
// backups holds Claude Code's backups of .claude.json, which belong to the
// same account.
func (c *claudeTranslator) IsolatedEntries() []string {
	return []string{".claude.json", ".credentials.json", "backups", "statsig"}
}

func (c *claudeTranslator) SupportsLogin() bool { return true }

func (c *claudeTranslator) SupportsOAuthToken() bool { return true }

// AuthVars lists the Claude Code variables per auth kind (research R16).
// GOOGLE_APPLICATION_CREDENTIALS, ANTHROPIC_VERTEX_PROJECT_ID and
// CLOUD_ML_REGION are deliberately absent: they are inert without
// CLAUDE_CODE_USE_VERTEX, and clearing the first would break Vertex profiles
// that rely on ambient application default credentials.
func (c *claudeTranslator) AuthVars() map[AuthKind][]string {
	return map[AuthKind][]string{
		AuthAPIKey:     {"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"},
		AuthOAuthToken: {"CLAUDE_CODE_OAUTH_TOKEN"},
		AuthVertex:     {"CLAUDE_CODE_USE_VERTEX"},
		AuthBedrock:    {"CLAUDE_CODE_USE_BEDROCK"},
	}
}

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
	if rp.OAuthToken != nil && rp.OAuthToken.Kind() == config.SourceSecret {
		return WrapperScript{}, fmt.Errorf("profile %q: auth.oauth_token does not support secret sources", rp.Name)
	}

	lines := wrapperLines{
		UnsetLine:     unsetLine(c.AuthVars(), rp.Kind()),
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
			// Distinct variable per credential: all checks render before all
			// exports, so a shared name would be clobbered by the credentials
			// block below when both use file sources.
			varRef := fmt.Sprintf("$HOME/.config/cc-deck/profiles/%s/%s/api_key", rp.Harness.Name(), rp.Name)
			lines.CredentialChecks = append(lines.CredentialChecks,
				fmt.Sprintf("_f_apikey=\"%s\"", varRef),
			)
			lines.CredentialChecks = append(lines.CredentialChecks, fileCheckBlock(rp.Name, "$_f_apikey"))
			lines.CredentialExports = append(lines.CredentialExports, "export ANTHROPIC_API_KEY=\"$(cat \"$_f_apikey\")\"")
		}
	}

	if rp.OAuthToken != nil {
		switch rp.OAuthToken.Kind() {
		case config.SourceEnv:
			lines.CredentialChecks = append(lines.CredentialChecks, envCheckBlock(rp.Name, rp.OAuthToken.Env))
			lines.CredentialExports = append(lines.CredentialExports, fmt.Sprintf("export CLAUDE_CODE_OAUTH_TOKEN=\"$%s\"", rp.OAuthToken.Env))
		case config.SourceFile:
			varRef := fmt.Sprintf("$HOME/.config/cc-deck/profiles/%s/%s/oauth_token", rp.Harness.Name(), rp.Name)
			lines.CredentialChecks = append(lines.CredentialChecks,
				fmt.Sprintf("_f_oauthtoken=\"%s\"", varRef),
			)
			lines.CredentialChecks = append(lines.CredentialChecks, fileCheckBlock(rp.Name, "$_f_oauthtoken"))
			lines.CredentialExports = append(lines.CredentialExports, "export CLAUDE_CODE_OAUTH_TOKEN=\"$(cat \"$_f_oauthtoken\")\"")
		}
	}

	if rp.Credentials != nil {
		switch rp.Credentials.Kind() {
		case config.SourceEnv:
			lines.CredentialChecks = append(lines.CredentialChecks, envCheckBlock(rp.Name, rp.Credentials.Env))
			lines.CredentialExports = append(lines.CredentialExports, fmt.Sprintf("export GOOGLE_APPLICATION_CREDENTIALS=\"$%s\"", rp.Credentials.Env))
		case config.SourceFile:
			varRef := fmt.Sprintf("$HOME/.config/cc-deck/profiles/%s/%s/credentials", rp.Harness.Name(), rp.Name)
			lines.CredentialChecks = append(lines.CredentialChecks,
				fmt.Sprintf("_f_creds=\"%s\"", varRef),
			)
			lines.CredentialChecks = append(lines.CredentialChecks, fileCheckBlock(rp.Name, "$_f_creds"))
			lines.CredentialExports = append(lines.CredentialExports, "export GOOGLE_APPLICATION_CREDENTIALS=\"$_f_creds\"")
		}
	}

	return renderWrapper(rp, lines)
}

func (c *claudeTranslator) DefaultConfigSubdir() string { return ".claude" }

func (c *claudeTranslator) PrepareConfigDir(rp ResolvedProfile, defaultDir string) ([]string, error) {
	warnings, err := PrepareSharedDir(rp.ConfigDir, defaultDir, c.IsolatedEntries())
	if err != nil {
		return warnings, fmt.Errorf("profile %q: prepare config dir: %w", rp.Name, err)
	}
	// Share user-scope MCP servers and project trust with the default setup.
	warning, err := mergeClaudeJSON(defaultClaudeJSONPath(defaultDir), filepath.Join(rp.ConfigDir, ".claude.json"))
	if err != nil {
		return warnings, fmt.Errorf("profile %q: merge .claude.json: %w", rp.Name, err)
	}
	if warning != "" {
		warnings = append(warnings, warning)
	}
	return warnings, nil
}

// ProviderType maps the backend to an OpenShell provider type. API key,
// subscription token and login profiles all use the anthropic backend and map
// to "claude" (FR-026).
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

package profile

import (
	"bytes"
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/cc-deck/cc-deck/internal/config"
)

//go:embed templates/wrapper.sh.tmpl
var wrapperTemplate string

var tmpl = template.Must(template.New("wrapper").Parse(wrapperTemplate))

// wrapperLines holds the per-translator variable lines that the template
// concatenates into a wrapper script. Each translator builds these from its
// own backend knowledge; the template does not interpret them.
type wrapperLines struct {
	// UnsetLine clears the auth variables of every auth kind other than the
	// profile's own, e.g. unset ANTHROPIC_API_KEY CLAUDE_CODE_USE_VERTEX.
	// Empty when nothing needs clearing.
	UnsetLine string

	// ConfigDirLine is the export line for the harness config directory,
	// e.g. export CLAUDE_CONFIG_DIR="$HOME/.local/share/cc-deck/profiles/work/claude".
	ConfigDirLine string

	// ModelLine is the model export, e.g. export ANTHROPIC_MODEL='claude-sonnet-5'.
	// Empty when the model is unset or written to a config file instead.
	ModelLine string

	// BackendLines are backend-specific exports such as CLAUDE_CODE_USE_VERTEX,
	// ANTHROPIC_VERTEX_PROJECT_ID, and CLOUD_ML_REGION.
	BackendLines []string

	// EnvLines are sorted user env exports, each single-quoted.
	EnvLines []string

	// CredentialChecks are if/fi blocks that exit 1 when a required
	// credential is missing.
	CredentialChecks []string

	// CredentialExports are the export lines that map credential sources
	// to the env vars the harness reads.
	CredentialExports []string

	// ExecLine is the final exec line, e.g. exec claude "$@".
	ExecLine string
}

// wrapperData is the template context passed to wrapper.sh.tmpl.
type wrapperData struct {
	Name  string
	Color string // single-quoted effective color
	Icon  string // single-quoted icon, '' when not declared
	Lines wrapperLines
}

// renderWrapper executes the embedded template with the given data and
// returns a WrapperScript ready to be written to disk. The caller (a
// Translator.Render implementation) is responsible for populating the
// wrapperLines struct with the correct per-backend lines.
func renderWrapper(rp ResolvedProfile, lines wrapperLines) (WrapperScript, error) {
	var buf bytes.Buffer
	// The icon is always exported, empty when none is declared, so a wrapper
	// started from inside another profiled session does not inherit that
	// session's icon.
	data := wrapperData{
		Name:  rp.Name,
		Color: shQuote(rp.Color),
		Icon:  shQuote(rp.Icon),
		Lines: lines,
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return WrapperScript{}, fmt.Errorf("rendering wrapper for profile %q: %w", rp.Name, err)
	}
	return WrapperScript{
		Name:    config.WrapperName(rp.Harness.Binary(), rp.Name),
		Content: buf.Bytes(),
		Mode:    0755,
	}, nil
}

// shQuote returns s wrapped in POSIX single quotes with embedded single
// quotes escaped as '\”. This is the standard POSIX sh quoting idiom:
// end the current single-quoted string, add an escaped single quote, and
// open a new single-quoted string.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// envCheckBlock returns a POSIX sh if-block that exits 1 when the given
// environment variable is empty or unset.
func envCheckBlock(profileName, envVar string) string {
	return fmt.Sprintf(
		"if [ -z \"${%s:-}\" ]; then\n  echo \"cc-deck profile '%s': credential %s is not available\" >&2\n  exit 1\nfi",
		envVar, profileName, envVar,
	)
}

// fileCheckBlock returns a POSIX sh if-block that exits 1 when the given
// file path (expressed as a variable reference) is not readable.
func fileCheckBlock(profileName, varRef string) string {
	return fmt.Sprintf(
		"if [ ! -r \"%s\" ]; then\n  echo \"cc-deck profile '%s': credential file %s is not available\" >&2\n  exit 1\nfi",
		varRef, profileName, varRef,
	)
}

// clearedVars returns, sorted, every variable in vars whose kind differs from
// own. These are the variables the wrapper unsets before applying the
// profile's own environment.
func clearedVars(vars map[AuthKind][]string, own AuthKind) []string {
	var out []string
	for kind, names := range vars {
		if kind == own {
			continue
		}
		out = append(out, names...)
	}
	sort.Strings(out)
	return out
}

// unsetLine returns the wrapper's single unset line for a profile of kind
// own, or "" when no variable needs clearing.
func unsetLine(vars map[AuthKind][]string, own AuthKind) string {
	names := clearedVars(vars, own)
	if len(names) == 0 {
		return ""
	}
	return "unset " + strings.Join(names, " ")
}

// checkClearedSources rejects a profile whose env credential source reads a
// variable the wrapper clears: the credential check would always fail,
// because the unset line runs before it.
func checkClearedSources(rp ResolvedProfile, vars map[AuthKind][]string) error {
	cleared := map[string]bool{}
	for _, n := range clearedVars(vars, rp.Kind()) {
		cleared[n] = true
	}
	sources := []struct {
		field string
		src   *config.CredentialSource
	}{
		{"api_key", rp.APIKey},
		{"credentials", rp.Credentials},
		{"oauth_token", rp.OAuthToken},
	}
	for _, s := range sources {
		if s.src != nil && s.src.Env != "" && cleared[s.src.Env] {
			return fmt.Errorf("profile %q: auth.%s reads %q, which the wrapper clears for this auth kind", rp.Name, s.field, s.src.Env)
		}
	}
	return nil
}

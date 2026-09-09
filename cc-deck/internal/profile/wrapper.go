package profile

import (
	"bytes"
	_ "embed"
	"fmt"
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
	Lines wrapperLines
}

// renderWrapper executes the embedded template with the given data and
// returns a WrapperScript ready to be written to disk. The caller (a
// Translator.Render implementation) is responsible for populating the
// wrapperLines struct with the correct per-backend lines.
func renderWrapper(rp ResolvedProfile, lines wrapperLines) (WrapperScript, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, wrapperData{
		Name:  rp.Name,
		Lines: lines,
	}); err != nil {
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

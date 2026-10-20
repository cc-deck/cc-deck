package profile

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/xdg"
)

// Translator defines the per-harness knowledge needed to render a profile
// into a wrapper script, prepare its config directory, and map its backend
// to an OpenShell provider type.
type Translator interface {
	// Harness returns the registry key, which equals agent.Agent.Name().
	Harness() string
	// ConfigDirEnv returns the environment variable that points the harness
	// at a custom config directory (e.g. "CLAUDE_CONFIG_DIR").
	ConfigDirEnv() string
	// IsolatedEntries returns the top-level directory entries that must never
	// be shared with the default config directory.
	IsolatedEntries() []string
	// SupportsLogin reports whether auth.login is allowed for this harness.
	SupportsLogin() bool
	// Backends returns the allowed backends; the first is the default.
	Backends() []config.BackendType
	// DefaultConfigSubdir returns the conventional config subdirectory
	// relative to $HOME for this harness (e.g. ".claude", ".codex",
	// ".config/opencode"). Used by Sync and Provision to locate the
	// default config directory when DetectConfig returns empty.
	DefaultConfigSubdir() string
	// Render produces a wrapper script from a resolved profile.
	Render(rp ResolvedProfile) (WrapperScript, error)
	// PrepareConfigDir creates the per-profile config directory with shared
	// symlinks and isolated entries. Returns any non-fatal warnings (e.g.
	// entries that could not be symlinked because a real file exists).
	PrepareConfigDir(rp ResolvedProfile, defaultDir string) (warnings []string, err error)
	// ProviderType maps a backend to an OpenShell provider type.
	// Returns "" if the backend has no provider mapping.
	ProviderType(backend config.BackendType) string
	// SupportsOAuthToken reports whether auth.oauth_token is allowed for
	// this harness.
	SupportsOAuthToken() bool
	// AuthVars returns, per auth kind, the harness environment variables that
	// select or carry that kind. Render clears the variables of every kind
	// other than the profile's own (research R16).
	AuthVars() map[AuthKind][]string
}

// AuthKind identifies how a profile authenticates against its harness.
type AuthKind string

const (
	AuthAPIKey     AuthKind = "api_key"
	AuthOAuthToken AuthKind = "oauth_token"
	AuthLogin      AuthKind = "login"
	AuthVertex     AuthKind = "vertex"
	AuthBedrock    AuthKind = "bedrock"
)

// ResolvedProfile holds everything needed to render a wrapper and prepare
// a config directory for one profile.
type ResolvedProfile struct {
	Name        string
	Harness     agent.Agent
	Backend     config.BackendType
	Model       string
	Env         map[string]string
	Color       string
	Icon        string
	APIKey      *config.CredentialSource
	Credentials *config.CredentialSource
	OAuthToken  *config.CredentialSource
	Login       bool
	Project     string
	Region      string
	ConfigDir   string
	CredDir     string
	BinDir      string
}

// Kind returns the profile's own auth kind. The vertex backend wins over the
// credential kind, because Vertex profiles select their mode through
// CLAUDE_CODE_USE_VERTEX. A profile without a credential (login) owns no
// auth variable.
func (rp ResolvedProfile) Kind() AuthKind {
	switch {
	case rp.Backend == config.BackendVertex:
		return AuthVertex
	case rp.OAuthToken != nil:
		return AuthOAuthToken
	case rp.APIKey != nil:
		return AuthAPIKey
	default:
		return AuthLogin
	}
}

// WrapperScript is the rendered output for one profile.
type WrapperScript struct {
	Name    string
	Content []byte
	Mode    uint32 // os.FileMode as uint32 for portability
}

// SyncResult reports what a Sync or Provision call did.
type SyncResult struct {
	Written   []string
	Removed   []string
	Skipped   []Skip
	Warnings  []string
	RCChanged bool
}

// Skip records a profile that was not rendered and the reason.
type Skip struct {
	Profile string
	Reason  string
}

var (
	tmu         sync.Mutex
	translators = map[string]Translator{}
)

// Register adds a translator to the global registry.
// Panics on a duplicate Harness().
func Register(t Translator) {
	tmu.Lock()
	defer tmu.Unlock()
	h := t.Harness()
	if _, exists := translators[h]; exists {
		panic(fmt.Sprintf("profile: duplicate translator for harness %q", h))
	}
	translators[h] = t
}

// Lookup returns the translator for the named harness.
func Lookup(harness string) (Translator, bool) {
	tmu.Lock()
	defer tmu.Unlock()
	t, ok := translators[harness]
	return t, ok
}

// All returns all registered translators.
func All() map[string]Translator {
	tmu.Lock()
	defer tmu.Unlock()
	result := make(map[string]Translator, len(translators))
	for k, v := range translators {
		result[k] = v
	}
	return result
}

// ResetRegistry clears the translator registry. Only for tests.
func ResetRegistry() {
	tmu.Lock()
	defer tmu.Unlock()
	translators = map[string]Translator{}
}

// Resolve builds a ResolvedProfile from config fields using the local
// XDG directories.
func Resolve(name string, p config.Profile, a agent.Agent) (ResolvedProfile, error) {
	return resolve(name, p, a, xdg.DataHome, xdg.ConfigHome)
}

// resolve is the shared implementation for both local (Resolve) and
// remote (resolveRemote) profile resolution. The only difference is
// the base directories for data and config.
func resolve(name string, p config.Profile, a agent.Agent, dataHome, configHome string) (ResolvedProfile, error) {
	harness := p.HarnessName()
	auth := p.EffectiveAuth()
	color := EffectiveColor(name, p)

	rp := ResolvedProfile{
		Name:      name,
		Harness:   a,
		Backend:   p.EffectiveBackend(),
		Model:     p.Model,
		Env:       p.Env,
		Color:     color,
		Icon:      p.Icon,
		Login:     auth.Login,
		Project:   p.Project,
		Region:    p.Region,
		ConfigDir: filepath.Join(dataHome, "cc-deck", "profiles", name, harness),
		CredDir:   filepath.Join(configHome, "cc-deck", "profiles", harness, name),
		BinDir:    filepath.Join(dataHome, "cc-deck", "bin"),
	}

	if auth.APIKey != nil {
		rp.APIKey = auth.APIKey
	}
	if auth.Credentials != nil {
		rp.Credentials = auth.Credentials
	}
	if auth.OAuthToken != nil {
		rp.OAuthToken = auth.OAuthToken
	}

	if tr, ok := Lookup(harness); ok {
		if rp.OAuthToken != nil && !tr.SupportsOAuthToken() {
			return ResolvedProfile{}, fmt.Errorf("profile %q: auth.oauth_token is not supported for harness %q", name, harness)
		}
		if err := checkClearedSources(rp, tr.AuthVars()); err != nil {
			return ResolvedProfile{}, err
		}
	}

	return rp, nil
}

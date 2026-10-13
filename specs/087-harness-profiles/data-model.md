# Data Model: Harness Profiles

**Feature**: 087-harness-profiles | **Date**: 2026-09-09 | **Revised**: 2026-10-12 (research R12 to R19)

## Go: `config.Profile` (extended)

```go
type Profile struct {
    // Existing fields, unchanged tags
    Backend             BackendType       `yaml:"backend,omitempty"`      // now optional; per-harness default
    APIKeySecret        string            `yaml:"api_key_secret,omitempty"`   // legacy, == auth.api_key.secret
    Model               string            `yaml:"model,omitempty"`
    Permissions         string            `yaml:"permissions,omitempty"`
    Project             string            `yaml:"project,omitempty"`
    Region              string            `yaml:"region,omitempty"`
    CredentialsSecret   string            `yaml:"credentials_secret,omitempty"` // legacy, == auth.credentials.secret
    AllowedEgress       []string          `yaml:"allowed_egress,omitempty"`
    GitCredentialType   GitCredentialType `yaml:"git_credential_type,omitempty"`
    GitCredentialSecret string            `yaml:"git_credential_secret,omitempty"`

    // New fields
    Harness string            `yaml:"harness,omitempty"` // "claude" (default) | "codex" | "opencode"
    Auth    *AuthConfig       `yaml:"auth,omitempty"`
    Env     map[string]string `yaml:"env,omitempty"`
    Color   string            `yaml:"color,omitempty"`   // "#RRGGBB"
    Icon    string            `yaml:"icon,omitempty"`    // single grapheme, width 1 or 2
}

type AuthConfig struct {
    APIKey      *CredentialSource `yaml:"api_key,omitempty"`
    Credentials *CredentialSource `yaml:"credentials,omitempty"` // Vertex ADC JSON
    OAuthToken  *CredentialSource `yaml:"oauth_token,omitempty"` // claude subscription token; env or file only
    Login       bool              `yaml:"login,omitempty"`
}

type CredentialSource struct {
    Env    string `yaml:"env,omitempty"`
    File   string `yaml:"file,omitempty"`
    Secret string `yaml:"secret,omitempty"`
}
```

### Derived accessors

- `Profile.HarnessName() string`: `Harness` or `"claude"`.
- `Profile.EffectiveBackend() BackendType`: `Backend`, else per-harness default (`claude: anthropic`, `codex: openai`, `opencode: openai`).
- `Profile.EffectiveAuth() AuthConfig`: merges legacy fields into the `Auth` view (`APIKeySecret` becomes `APIKey.Secret`, `CredentialsSecret` becomes `Credentials.Secret`) without mutating the stored struct.
- `CredentialSource.Kind() SourceKind`: `env | file | secret | none`.
- `Profile.WrapperName(binary string) string`: `binary + "-" + name` (name supplied by the caller from the map key).

### Validation rules (config/validate.go, category `profiles`)

| Rule | Severity | Message pattern |
|------|----------|-----------------|
| profile name matches `^[a-z0-9][a-z0-9-]*$` | error | `profile "%s": name must be lowercase letters, digits and hyphens` |
| `harness` in registry | error | `profile "%s": unknown harness %q` |
| `backend` allowed for harness | error | `profile "%s": backend %q not valid for harness %q` |
| each `CredentialSource` has exactly one of env, file, secret | error | `profile "%s": auth.%s must set exactly one of env, file, secret` |
| at most one of `login`, `api_key`, `oauth_token` | error | `profile "%s": auth.login, auth.api_key and auth.oauth_token are mutually exclusive` |
| `oauth_token` only for harness `claude` with backend `anthropic` | error | `profile "%s": auth.oauth_token requires harness claude and backend anthropic` |
| `oauth_token` source is `env` or `file` | error | `profile "%s": auth.oauth_token must set exactly one of env, file` |
| credential source `env` name is not a variable the wrapper clears (R16) | error | `profile "%s": auth.%s reads %q, which the wrapper clears for this auth kind` |
| `env` key belongs to a different auth kind (R16; emitted by `profile sync`, not at config load) | warning | `profile "%s": env %q belongs to auth kind %q and overrides the profile's credential` |
| `login` only for harness `claude` | error | `profile "%s": auth.login is not supported for harness %q` |
| anthropic/openai backend has api_key (any source, legacy included), oauth_token (claude only) or login | error | `profile "%s": %s backend requires auth.api_key, auth.oauth_token or auth.login` |
| vertex backend has project and region | error | (existing message) |
| `color` matches `^#[0-9a-fA-F]{6}$` | error | `profile "%s": color must be #RRGGBB` |
| `icon` is one grapheme cluster with display width 1 or 2 | error | `profile "%s": icon must be a single glyph of width 1 or 2` |
| wrapper name collides with an existing command on PATH other than a cc-deck wrapper | error | `profile "%s": wrapper %q shadows an existing command` |
| `default_profile` references an existing profile | error | (existing) |
| `env` key is a valid shell identifier | error | `profile "%s": env key %q is not a valid variable name` |

Legacy profiles (`backend: anthropic` + `api_key_secret`, or `backend: vertex` + project/region) pass without change.

## Go: `profile.ResolvedProfile` (in-memory, per render)

```go
type ResolvedProfile struct {
    Name        string
    Harness     agent.Agent          // adapter (Binary(), InstallHooksAt(), ResumeArgs())
    Backend     config.BackendType
    Model       string
    Env         map[string]string    // profile env, sorted keys when rendered
    Color       string               // "#RRGGBB", declared or derived
    Icon        string
    APIKey      *config.CredentialSource
    Credentials *config.CredentialSource
    OAuthToken  *config.CredentialSource
    Login       bool
    Project     string               // vertex
    Region      string               // vertex
    ConfigDir   string               // $XDG_DATA_HOME/cc-deck/profiles/<name>/<harness>
    CredDir     string               // $XDG_CONFIG_HOME/cc-deck/profiles/<name>
    BinDir      string               // $XDG_DATA_HOME/cc-deck/bin
}
```

Paths are expressed with `$HOME`-relative form inside rendered scripts so the same script works on the host and on remotes.

## Go: `profile.WrapperScript`

```go
type WrapperScript struct {
    Name    string // "claude-work"
    Content []byte // rendered POSIX sh
    Mode    os.FileMode // 0755
}
```

## Go: `profile.SyncResult`

```go
type SyncResult struct {
    Written  []string  // wrapper names created or updated
    Removed  []string  // stale wrappers removed
    Skipped  []Skip    // {Profile, Reason} e.g. harness not installed, secret-only source
    Warnings []string  // e.g. env var not set on host, provider missing on workspace
    RCChanged bool     // rc block created or changed: tell the user to open a new shell
}
```

State transitions for a wrapper file: absent → written (profile valid and harness installed) → updated (profile changed) → removed (profile deleted or became invalid). Sync is idempotent: a second run with no config change reports empty `Written` and `Removed`.

## Go: `profile.AuthKind` (R16)

```go
type AuthKind string // "api_key" | "oauth_token" | "vertex" | "bedrock" | "login"
```

`ResolvedProfile.Kind() AuthKind` derives the profile's own kind: vertex backend gives `vertex`; otherwise `oauth_token`, `api_key` or `login` from the auth block. Translators expose `AuthVars() map[AuthKind][]string`; `Render` unsets the union of all other kinds' variables.

## Go: `agent.NormalizedPayload` (extended)

```go
Profile      string `json:"profile,omitempty"`
ProfileColor string `json:"profile_color,omitempty"` // "#RRGGBB"
ProfileIcon  string `json:"profile_icon,omitempty"`  // declared icon, single printable glyph
```

`AgentIndicator` is replaced by the profile icon when the profile declares one. Source order (R17): `CC_DECK_PROFILE_COLOR` / `CC_DECK_PROFILE_ICON` from the hook environment when the color is valid `#RRGGBB`; otherwise the `config.yaml` lookup; otherwise `Derive(name)`.

## Go: `session.SessionEntry` and `pluginSession` (extended)

```go
Agent   string `json:"agent,omitempty"`   // "claude" | "codex" | "opencode"; empty means claude (pre-feature files)
Profile string `json:"profile,omitempty"` // empty means no profile
```

Snapshot `Version` stays 1; both fields are optional. `PendingOverride` (restore-meta pipe) gains `Profile string` and `ProfileColor string`.

## Rust: `Session` (extended, `session.rs`)

```rust
#[serde(default)] pub profile: Option<String>,
#[serde(default)] pub profile_color: Option<(u8, u8, u8)>,
#[serde(default)] pub profile_icon: Option<String>,
```

Set once on the first hook that carries `profile` (same guard as `agent_name`); reset or replaced along with `agent_name` on session replacement.

## Rust: `HookPayload` (extended, `pipe_handler.rs`)

```rust
#[serde(default)] pub profile: Option<String>,
#[serde(default)] pub profile_color: Option<String>, // "#RRGGBB", parsed in the controller
#[serde(default)] pub profile_icon: Option<String>,  // declared icon, legend only
```

## Rust: `RenderSession` and `RenderPayload` (extended, `lib.rs`)

```rust
pub profile_color: Option<(u8, u8, u8)>,      // RenderSession: edge bar color; None = unprofiled (renamed from agent_color, R13)
pub profile_legend: Vec<LegendEntry>,         // RenderPayload
pub struct LegendEntry { pub icon: String, pub color: (u8, u8, u8), pub name: String }
```

`show_agent_indicators` = number of distinct `agent_name` values among visible sessions > 1 (pre-feature rule, R13). The glyph always uses the harness brand color. `profile_legend` lists each distinct profile name among visible sessions, sorted by name; `icon` is the session's `profile_icon` when declared, else empty.

Sidebar entry rendering (R12): column 1 of both text lines is `▌` in `profile_color` on the default background when `profile_color` is `Some`, else the existing margin space. For profiled rows the active, cursor and rename highlight backgrounds start at column 2.

## Color palette (`profile/color.go`, R14)

Eight hues 45 degrees apart. `Derive(name)` = FNV-1a 32-bit of the name modulo 8. `Pick(used)` = entry maximizing the minimum hue distance to `used`, ties to the lowest index. `Similar(a, b)` = circular hue distance under 40 degrees, or both achromatic (HSL saturation under 0.15) with lightness difference under 0.15.

| # | Hex | Hue |
|---|-----|-----|
| 0 | `#E27C5A` | 15 |
| 1 | `#E2E25A` | 60 |
| 2 | `#7CE25A` | 105 |
| 3 | `#5AE29E` | 150 |
| 4 | `#5AC0E2` | 195 |
| 5 | `#8686EA` | 240 |
| 6 | `#C05AE2` | 285 |
| 7 | `#E25A9E` | 330 |

Every entry has a contrast ratio of at least 4.5:1 against black and against `#1E1E2E`. The bar never sits on a highlight background (R12), so contrast against `ACTIVE_BG` and `CURSOR_BG` is not required.

## Filesystem layout (host and workspace, `$HOME`-relative)

```text
~/.config/cc-deck/config.yaml                     profiles (host only)
~/.config/cc-deck/profiles/<name>/api_key         0600, from {file:} source (host copy and remote upload)
~/.config/cc-deck/profiles/<name>/credentials     0600, Vertex ADC from {file:} source
~/.config/cc-deck/profiles/<name>/oauth_token     0600, subscription token from {file:} source
~/.local/share/cc-deck/bin/<binary>-<name>        0755 wrapper
~/.local/share/cc-deck/profiles/<name>/claude/    CLAUDE_CONFIG_DIR (symlinks + isolated entries)
~/.local/share/cc-deck/profiles/<name>/codex/     CODEX_HOME
~/.local/share/cc-deck/profiles/<name>/opencode/opencode.json   OPENCODE_CONFIG
~/.bashrc, ~/.zshrc                               managed block adds bin dir to PATH
```

On OpenShell `$HOME` is `/sandbox`.

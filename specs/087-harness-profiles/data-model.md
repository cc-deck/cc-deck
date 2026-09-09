# Data Model: Harness Profiles

**Feature**: 087-harness-profiles | **Date**: 2026-09-09

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
| `login` and `api_key` not both set | error | `profile "%s": auth.login and auth.api_key are mutually exclusive` |
| `login` only for harness `claude` | error | `profile "%s": auth.login is not supported for harness %q` |
| anthropic/openai backend has api_key (any source, legacy included) or login | error | `profile "%s": %s backend requires auth.api_key or auth.login` |
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

## Go: `agent.NormalizedPayload` (extended)

```go
Profile      string `json:"profile,omitempty"`
ProfileColor string `json:"profile_color,omitempty"` // "#RRGGBB"
```

`AgentIndicator` is replaced by the profile icon when the profile declares one.

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
```

Set once on the first hook that carries `profile` (same guard as `agent_name`); reset or replaced along with `agent_name` on session replacement.

## Rust: `HookPayload` (extended, `pipe_handler.rs`)

```rust
#[serde(default)] pub profile: Option<String>,
#[serde(default)] pub profile_color: Option<String>, // "#RRGGBB", parsed in the controller
```

## Rust: `RenderSession` and `RenderPayload` (extended, `lib.rs`)

```rust
pub agent_color: Option<(u8, u8, u8)>,        // RenderSession: overrides brand color
pub profile_legend: Vec<LegendEntry>,         // RenderPayload
pub struct LegendEntry { pub indicator: String, pub color: (u8, u8, u8), pub name: String }
```

`show_agent_indicators` = number of distinct `(agent_name, profile)` pairs among visible sessions > 1. `profile_legend` lists each distinct pair with a non-empty profile, sorted by name.

## Color palette (`profile/color.go`)

Index = FNV-1a 32-bit of the profile name modulo 8.

| # | Hex | Note |
|---|-----|------|
| 0 | `#4FC1E9` | sky |
| 1 | `#A0D468` | green |
| 2 | `#ED5565` | red |
| 3 | `#AC92EC` | violet |
| 4 | `#FFCE54` | yellow |
| 5 | `#48CFAD` | mint |
| 6 | `#EC87C0` | pink |
| 7 | `#F6BB42` | amber |

All entries exceed a 3:1 contrast ratio against `ACTIVE_BG (25,45,55)` and a black background. `#FFAA32` (Claude brand) and `#3CBEBE` (OpenCode brand) are excluded so unprofiled sessions stay distinguishable.

## Filesystem layout (host and workspace, `$HOME`-relative)

```text
~/.config/cc-deck/config.yaml                     profiles (host only)
~/.config/cc-deck/profiles/<name>/api_key         0600, from {file:} source (host copy and remote upload)
~/.config/cc-deck/profiles/<name>/credentials     0600, Vertex ADC from {file:} source
~/.local/share/cc-deck/bin/<binary>-<name>        0755 wrapper
~/.local/share/cc-deck/profiles/<name>/claude/    CLAUDE_CONFIG_DIR (symlinks + isolated entries)
~/.local/share/cc-deck/profiles/<name>/codex/     CODEX_HOME
~/.local/share/cc-deck/profiles/<name>/opencode/opencode.json   OPENCODE_CONFIG
~/.bashrc, ~/.zshrc                               managed block adds bin dir to PATH
```

On OpenShell `$HOME` is `/sandbox`.

# Contract: Profile schema in `config.yaml`

**Feature**: 087-harness-profiles

## Schema

```yaml
default_profile: work            # unchanged meaning: Kubernetes deploy default and git credentials
profiles:
  <name>:                        # ^[a-z0-9][a-z0-9-]*$
    harness: claude              # claude (default) | codex | opencode
    backend: anthropic           # optional; allowed set and default depend on harness
    model: claude-sonnet-5       # optional
    auth:                        # optional for legacy profiles, required otherwise
      api_key:                   # exactly one of env, file, secret
        env: ANTHROPIC_API_KEY_WORK
        # file: ~/.secrets/anthropic-work
        # secret: anthropic-work        (Kubernetes deploy only)
      credentials:               # vertex only, ADC JSON; exactly one of env, file, secret
        file: ~/.config/gcloud/work-adc.json
      login: true                # claude only; mutually exclusive with api_key
    project: my-gcp-project      # vertex
    region: us-east5             # vertex
    env:                         # optional extra environment for the wrapper
      ANTHROPIC_DEFAULT_HAIKU_MODEL: claude-haiku-4-5-20251001
    color: "#4FC1E9"             # optional, #RRGGBB
    icon: "W"                    # optional, single glyph, width 1 or 2
    # legacy fields still accepted:
    # api_key_secret, credentials_secret, permissions, allowed_egress,
    # git_credential_type, git_credential_secret
```

## Worked example: two Anthropic accounts plus Codex

```yaml
profiles:
  work:
    harness: claude
    backend: vertex
    project: acme-ml
    region: us-east5
    model: claude-sonnet-5
    auth:
      credentials: {file: ~/.config/gcloud/acme-adc.json}
    color: "#4FC1E9"
  private:
    harness: claude
    model: claude-opus-5
    auth:
      login: true
    color: "#EC87C0"
  team:
    harness: codex
    model: gpt-5
    auth:
      api_key: {env: OPENAI_API_KEY_TEAM}
```

Produces wrappers `claude-work`, `claude-private`, `codex-team`.

## Backward compatibility

| Existing file | Interpretation |
|---------------|----------------|
| `backend: anthropic`, `api_key_secret: s` | harness claude, `auth.api_key.secret: s`; valid; no wrapper on non-Kubernetes backends (secret source), reported once by `sync` as skipped |
| `backend: vertex`, `project`, `region`, `credentials_secret` | harness claude, vertex; same as above |
| profile with `git_credential_*` only | still resolved by `ws/repos.go` for every backend; no harness fields required if `default_profile` points at it and it has a legacy backend |

Loading never rewrites the file. `Save()` writes new fields only when set (`omitempty`).

## Validation messages

Listed in `data-model.md` under "Validation rules". All are `error` severity in category `profiles`, surfaced through `Config.Validate()` and `ValidateAndWarn()` at load time, and by `cc-deck config profile add` before saving.

## CLI mapping (`cc-deck config profile`)

| Command | Behavior |
|---------|----------|
| `add <name> [flags]` | Flags: `--harness`, `--backend`, `--model`, `--api-key-env`, `--api-key-file`, `--credentials-file`, `--login`, `--project`, `--region`, `--env K=V` (repeatable), `--color`, `--icon`. Without flags, prompts interactively (harness first, then backend and auth). Validates before saving. Sets `default_profile` when it is the first profile. |
| `list [-o table\|json\|yaml]` | Columns NAME, HARNESS, BACKEND, AUTH (`env:NAME`, `file`, `secret`, `login`), MODEL, DEFAULT. |
| `show <name>` | Every field; sources are printed as references, never values; shows the resolved wrapper name and color. |
| `use <name>` | Unchanged: sets `default_profile`. |
| `delete <name>` | Removes the profile; clears `default_profile` if it pointed there; prints a reminder to run `sync` to remove the wrapper. |
| `sync [--workspace <name>]` | Local sync by default; with `--workspace`, provisions the named SSH or OpenShell workspace. Prints written, removed, skipped, warnings, and the new-shell hint. Exit 0 even with skips; exit 1 on config validation errors. |

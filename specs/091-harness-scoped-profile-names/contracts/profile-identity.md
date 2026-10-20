# Contract: Profile Identity and Selection

## Configuration

```yaml
default_profile: claude/private
profiles:
  claude:
    private:
      auth:
        oauth_token: {env: CLAUDE_TOKEN_PRIVATE}
      color: "#E27C5A"
  codex:
    private:
      auth:
        login: true
      color: "#5AC0E2"
```

The supported outer keys are `claude`, `codex`, and `opencode`. The inner keys use the existing short-name grammar. An inner `harness` property is invalid. Existing profile fields keep their meanings. YAML duplicate keys are errors, including duplicate harness and profile keys. A flat `profiles.private` entry is invalid and reports the required nested shape.

## Selectors

- Canonical: `claude/private`.
- Shorthand: `private`, only if exactly one harness has that name.
- Ambiguous shorthand: error with all qualified matches; no mutation.
- Missing selector: error identifying the requested reference.
- `default_profile`: qualified when written by `profile use` or migration; an unambiguous bare value can be read, but ambiguity is an error.

## CLI

`cc-deck config profile add private --harness claude` creates `profiles.claude.private`; interactive add prompts for harness before checking existence. Add rejects a duplicate pair and permits equal names in other harnesses. `list` includes both `name` and `harness` in all output formats. `show`, `use`, and `delete` accept selectors under the rules above. `sync` handles every pair independently and reports qualified identities where equal names would otherwise be confused.

## Runtime

Generated command names stay `claude-private`, `codex-private`, and `opencode-private`. Wrappers continue exporting the short `CC_DECK_PROFILE` value. Runtime consumers combine it with the known harness when they need a profile lookup. Sidebar legend entries are keyed and labeled by `harness/name`; snapshots retain agent and short profile in their existing fields.

Credential files reside under `~/.config/cc-deck/profiles/<harness>/<name>/` in host and supported workspaces. Existing data/login directories under `~/.local/share/cc-deck/profiles/<name>/<harness>/` stay valid.

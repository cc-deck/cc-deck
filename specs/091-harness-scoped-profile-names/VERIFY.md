# Verification

## Local migration

- Backup: `~/.config/cc-deck/config.yaml.backup-091-20261020T042125Z` (mode `0600`)
- Migrated profiles: `claude/private`, `claude/work`
- Default: `claude/private`
- Copied credential files: `profiles/private/oauth_token` → `profiles/claude/private/oauth_token`; `profiles/work/credentials` → `profiles/claude/work/credentials`
- Both copied files compare byte-for-byte with their retained sources and keep mode `0600`.
- `cc-deck config validate` passes. Human `cc-deck config profile list` output displays wrapper names; JSON/YAML retain separate harness and short-name fields.
- `claude-private` and `claude-work` wrappers are executable and point to their existing `~/.local/share/cc-deck/profiles/<name>/claude` login data directories. Those data directories were left in place.
- `cc-deck config profile sync` refreshed `claude-work`; `claude-private` was already current.

Credential values are intentionally omitted.

## Project checks

- `make test` passes: all Go packages, including Compose smoke tests, and 485 Rust tests. The final run reused Go and Rust test cache entries from the immediately preceding full run of this unchanged code; all tests had passed there, including live Compose lifecycle tests.
- `make lint` passes (`go vet ./...` and `cargo clippy -- -D warnings`).
- `git diff --check` passes.
- The configured prose plugin is not present in this Codex session, so documentation was checked manually against the nested schema and selector behavior.

## Interactive walkthrough

- All 8 runbook steps passed. The human list displayed `claude-private` and `codex-private`; qualified `show` commands matched each wrapper and harness; the bare name reported both qualified choices.
- Selecting `codex/private` marked only that profile as default. The walkthrough restored `claude/private`, deleted `codex/private`, synchronized wrappers, and confirmed the Claude profile remained the default.
- Sync exited successfully and left several cache files unchanged because they were regular files rather than symlinks.

## Spec verification

- Closeout gate: passed; no unresolved Critical or Important review findings.
- Code hygiene: reviewed the changed profile-list display path and the previously reviewed configuration, CLI, profile sync/provisioning, credential, workspace, hook, session restore, and sidebar paths. The human table derives wrapper names from the configured harness; JSON/YAML retain short name and harness separately. No dead branches, stale profile-key consumers, or unsafe pair sharing were found.
- Functional requirements: FR-001 through FR-014 implemented. Evidence is summarized in [REVIEW-CODE.md](REVIEW-CODE.md); tests cover nested strict parsing, pair selectors, ambiguity, defaults, credential isolation, wrappers, hooks, workspace provisioning, and snapshot restore.
- Spec drift: none found between the implementation and `spec.md`.
- Success criteria: SC-001 and SC-002 are covered by wrapper/selector tests; SC-003 by the live migration checks above; SC-004 by the same-name snapshot round-trip test; SC-005 by the README and reference documentation updates.
- Overall: PASS. Documentation was manually reviewed because the configured prose plugin is unavailable in this session; no automated prose-plugin result is claimed.

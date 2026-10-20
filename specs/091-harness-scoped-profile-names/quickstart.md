# Quickstart: Validate Harness-Scoped Profiles

Use a temporary cc-deck configuration and credentials for the first checks. Do not point these examples at the live config until the one-time migration has been reviewed and backed up.

## 1. Validate the schema

Create a configuration with `profiles.claude.private` and `profiles.codex.private` as shown in [the identity contract](contracts/profile-identity.md). Run `cc-deck config validate` (an alias of `config check`). It should accept both profiles. Add a duplicate `private` key under Claude; validation should fail rather than keep one copy. Try an old flat entry; the error should show the nested form.

## 2. Validate selection

Run `cc-deck config profile list`; the human-readable NAME column should show `claude-private` and `codex-private`, matching the synced wrapper names. JSON and YAML output retain separate short-name and harness fields. Then run `cc-deck config profile show private`. It should fail and offer `claude/private` and `codex/private`. `cc-deck config profile show claude/private` should show only the Claude profile. `cc-deck config profile use codex/private` should save `default_profile: codex/private`.

## 3. Validate wrappers and sessions

Run `cc-deck config profile sync` in a local workspace with both harnesses installed. Confirm `claude-private` and `codex-private` are available. Launch both through cc-deck workspace panes, inspect the sidebar legend, then save and restore a workspace snapshot. Each pane should retain its original wrapper and account. Repeat profile provisioning in an SSH or OpenShell workspace when those backends are available.

## 4. Validate the current installation migration

After the implementation passes tests, carry out this one-time procedure for the current installation:

1. Inspect only the live profile names, effective harnesses, default selector, and credential file paths. Confirm each old key maps to exactly one `<harness>/<name>` pair. Inventory every credential source and target under `~/.config/cc-deck/profiles/<harness>/<name>/`. Stop before changing live files if a source is unreadable, a target file or directory conflicts, or a profile/default mapping is unclear.
2. Make a timestamped, mode-restricted, recoverable copy of `~/.config/cc-deck/config.yaml` before changing either configuration or credentials. Stage a transformed copy, preserving every non-profile setting. Move each flat definition under its effective harness, remove the redundant inner `harness`, and qualify `default_profile` with the original profile's harness. Verify the staged file with `cc-deck --config <staged-path> config validate`.
3. Copy credential files into the harness/name targets using exclusive creation, so an existing target cannot be overwritten even if it appears after preflight. Keep original files. Install the staged config atomically only after all credential copies succeed. Leave `~/.local/share/cc-deck/profiles/<name>/<harness>/` and its login state unchanged.
4. Run `cc-deck config validate`, `cc-deck config profile list`, and `cc-deck config profile sync`. Verify the existing `claude-private` and `claude-work` wrappers and login/data directory paths. Record the backup location and the names of copied credential paths in `VERIFY.md`; never print credential values.

If validation fails, restore the backed-up config and retain the original credential files. The newly copied targets can be removed after confirming they were created by this migration. Never remove the backup until the installation has been verified.

## 5. Project verification

Run the repository's `make test` and `make lint` targets and the documentation prose checks required by the project constitution. Confirm the README, CLI reference, configuration reference, and harness profiles guide use the nested schema and qualified selectors.

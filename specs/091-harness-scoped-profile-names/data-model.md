# Data Model: Harness-Scoped Profiles

## Profile identity

| Field | Meaning | Rule |
|---|---|---|
| `harness` | Agent harness namespace | One of `claude`, `codex`, `opencode` |
| `name` | Short user-chosen profile name | Existing lowercase name pattern |

The ordered pair `(harness, name)` is unique. Its canonical selector is `harness/name`. A bare name is only a lookup shorthand while exactly one pair has that name; it is never the stored identity.

## Configuration

`profiles` is a map of harness groups to maps of short names to existing profile definitions. The inner definition retains auth, backend, model, color, icon, env, and legacy Kubernetes/git fields, but has no `harness` field. `default_profile` is a qualified selector when written by `profile use` or the one-time migration.

Validation rejects unknown harness groups, malformed names, redundant inner harness fields, duplicate keys/pairs, a missing default, and ambiguous bare references. Legacy flat profile entries are rejected with migration guidance.

## Runtime profile

A resolved profile combines its identity and definition. It produces a wrapper command named `<harness command>-<name>`. Its data/login directory remains `~/.local/share/cc-deck/profiles/<name>/<harness>`. Credential files use `~/.config/cc-deck/profiles/<harness>/<name>` on both host and workspaces. Color and icon remain properties of the definition.

## Session identity

A session carries `agent` and `profile` separately. `agent` supplies the harness; `profile` is the short name. The pair is used for profile lookup, background attribution, snapshot restore, sidebar legend grouping, and qualified legend labels. Existing unprofiled sessions have an empty profile and retain their current behavior.

## Workspace provider identity

An OpenShell credential provider created for a profile includes workspace, harness, and short name in its identity. Deduplication uses the same pair so one harness cannot suppress another's provider.

## One-time migration state

The migration maps each old flat key and effective harness to the new nested entry, removes the redundant inner harness field, rewrites the default to its qualified identity, and copies credential files to the harness/name path. Existing data/login directories stay in place. Before live changes, a recoverable config copy is created. Conflicting target files stop migration and are not overwritten.

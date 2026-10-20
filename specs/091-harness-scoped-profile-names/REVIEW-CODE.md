# Code Review

## Outcome

**Gate: PASS — 100% functional requirement compliance.**

All FR-001 through FR-014 have implementation and test coverage. No unresolved Critical or Important findings remain.

## Review coverage

The review examined spec compliance and the implementation from behavior, security/runtime, architecture, and test coverage perspectives. The review checked the changed Go configuration, CLI, profile, workspace, hook, session and credential paths, the Rust sidebar renderer, and user documentation.

The reviewers identified and verified fixes for these issues:

1. **Nested schema strictness:** YAML decoding ignored unknown fields in nested auth objects. Recursive validation now rejects unknown and repeated keys, including misspellings under `auth`, while allowing arbitrary user-defined keys in `env`.
2. **Default stability when adding a same-name profile:** Adding a second harness profile could make a previously valid bare default ambiguous. The add path now resolves the old default to a qualified identity before writing the new profile.
3. **Default display and diagnostics:** List/show now recognize a uniquely resolvable bare default, and synchronization/provisioning warnings include the qualified harness/name identity.
4. **Snapshot identity coverage:** A round-trip test now verifies same-named Claude and Codex sessions retain their separate wrapper commands through snapshot parsing, entry construction, serialization, and restore command generation.

Each issue was re-reviewed after its fix. No other confirmed defects were reported.

## Requirement compliance

| Requirement | Status | Evidence |
|---|---|---|
| FR-001 | Compliant | Nested `Config.Profiles` and strict profile body validation in `cc-deck/internal/config`; nested schema examples in configuration reference. |
| FR-002 | Compliant | Duplicate pair and repeated YAML key validation in config tests; cross-harness repeated names tested. |
| FR-003 | Compliant | Legacy flat entry, unknown harness, malformed name, and nested typo validation tests in `internal/config`. |
| FR-004 | Compliant | Qualified selectors parsed and exercised by profile CLI tests. |
| FR-005 | Compliant | Bare-name unique resolution and ambiguity errors with qualified candidates covered in profile CLI tests. |
| FR-006 | Compliant | `profile use` stores qualified defaults; default validation and ambiguity cases covered in config and CLI tests. |
| FR-007 | Compliant | Add/list/show/use/delete/sync resolve profile pairs; duplicate pairs are rejected, human list names match wrapper commands, and JSON/YAML retain separate fields, covered in profile CLI and sync tests. |
| FR-008 | Compliant | Harness-specific wrapper names verified in wrapper generation and execution tests. |
| FR-009 | Compliant | Credentials, login, sync, and deletion paths use the pair identity; provisioning, credential, and isolation tests cover both harnesses. |
| FR-010 | Compliant | Hook/session attribution and Rust sidebar qualified labels, colors, and icons covered by Go hook tests and Rust integration tests. |
| FR-011 | Compliant | Same-named Claude/Codex snapshot round trip verifies each restore command retains its original harness wrapper. |
| FR-012 | Compliant | Workspace provisioning and credential handling tests cover local, SSH, OpenShell, Kubernetes, container, and Compose paths. |
| FR-013 | Compliant | Live single-user migration backup, pair-preserving defaults, byte-for-byte credential copies, and post-migration validation are recorded in `VERIFY.md`. |
| FR-014 | Compliant with tooling limitation | Tests cover schema, selectors, ambiguity, isolation, migration, wrappers, and session identity; README and required references were updated. Documentation was manually checked because the configured prose plugin is unavailable in this Codex session. |

No out-of-scope feature behavior was found.

## Verification

- `make test` passed, including Go tests, Compose smoke tests, and all 472 Rust tests.
- `make lint` passed (`go vet ./...` and `cargo clippy -- -D warnings`).
- `git diff --check` passed.
- The current user's single-user profile configuration and credential files were migrated and verified as recorded in [VERIFY.md](VERIFY.md).
- Documentation was manually checked against the nested schema and selector behavior. The configured prose plugin is unavailable in this Codex session, so its automated prose check could not be run.

## Remaining issues

None.

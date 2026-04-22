# Deep Review Report: Centralize Workspace Definitions

**Branch**: `041-centralize-workspace-definitions`
**Date**: 2026-04-22
**Reviewers**: 5 specialized agents (Correctness, Security, Architecture, Testing, Spec Compliance)
**Spec Compliance**: 25/25 requirements PASS

## Verdict

The implementation is **functionally complete** and **spec-compliant** (all 18 FRs, 6 SCs, 3 clarifications, 4 edge cases pass). The code removes more lines than it adds, and the core design is sound. However, there are actionable issues in correctness, security hardening, and test coverage that should be addressed before merge.

## Build Status

- `go vet`: PASS
- `go test`: 2 failures in `compose_smoke_test.go` (pre-existing, not related to this branch)
- All unit tests for changed code: PASS

---

## Critical / Important Findings

### C1. Phase-1 Resolution Fallthrough Selects Wrong Project

**Source**: Correctness
**File**: `internal/cmd/ws.go:1641-1649`

When `FindByProjectDir` returns multiple project-dir matches but none have `LastAttached` set, `selectByRecency` returns empty and the code falls through to phase 2 (global pool). This could silently connect a user to a workspace from a completely different project.

**Fix**: When phase 1 finds multiple project-dir matches but `selectByRecency` returns empty, return an error ("multiple workspaces match this project; specify a name") instead of falling through to phase 2.

---

### C2. Stale Definition After Failed Create Blocks Retry

**Source**: Correctness
**File**: `internal/cmd/ws.go:366-515`

The definition is added to the central store (line 366) before `e.Create()` (line 515). If Create fails (podman unavailable, SSH failure, etc.), the orphaned definition remains and blocks retrying with the same name ("already exists" error). The user must `ws delete` first.

**Fix**: Either defer the definition store write until after `Create` succeeds, or clean up the definition on `Create` failure.

---

### C3. YAML Injection via Template Placeholder Values

**Source**: Security (MEDIUM)
**File**: `internal/ws/template.go:131-143`, consumed at `internal/cmd/ws.go:337-341`

`ResolvePlaceholders` performs raw byte substitution on serialized YAML, then re-parses. A placeholder value can break out of the YAML string context and inject arbitrary fields. For example, responding to a `{{ssh_user}}` prompt with `roland"\nimage: "evil:latest` would override the image field. While the attack surface is limited (user must type the malicious value at their own prompt, or a cloned repo must contain a malicious template), it violates the principle of least surprise.

**Fix**: Either (a) YAML-escape placeholder values before substitution (reject or escape newlines, colons, quotes), or (b) perform substitution on parsed struct fields rather than raw YAML bytes.

---

### C4. TemplateVariant Duplicates WorkspaceDefinition Fields

**Source**: Architecture (IMPORTANT)
**File**: `internal/ws/template.go:33-56`

`TemplateVariant` duplicates 20+ fields from `WorkspaceDefinition`. The `VariantToDefinition` function is a tedious field-by-field copy. Adding a new field to `WorkspaceDefinition` without also adding it to `TemplateVariant` and `VariantToDefinition` will be a silent bug (no compiler error).

**Fix**: Have `TemplateVariant` embed a shared spec struct, or use YAML unmarshaling into `WorkspaceDefinition` directly (skipping `Name`/`Type` which are set externally).

---

### C5. Missing Test: Template Placeholder Integration Through ws new

**Source**: Testing (CRITICAL)
**File**: `internal/cmd/ws_new_test.go`

US1 scenario 1 (template with `{{placeholder}}`, prompting, resolution, storage) has no integration test. The existing template test uses no placeholders. The interactive prompting path through `runWsNew` (lines 329-342) is untested.

---

### C6. Missing Test: Multiple Workspaces, No Recency, Error Path

**Source**: Testing (CRITICAL)
**File**: `internal/cmd/ws_resolve_test.go`

US2 scenario 3 ("multiple workspaces exist, none attached") should return an error. No test exists for this path.

---

### C7. Missing Test: ws update --sync-repos

**Source**: Testing (CRITICAL)
**File**: No test file exists

US4 has two acceptance scenarios. Neither is tested. The `runWsUpdate` repos lookup and the "no repos defined" error path are both uncovered.

---

### C8. Multi-Variant Template Error Message is Misleading

**Source**: Correctness (IMPORTANT)
**File**: `internal/cmd/ws.go:308`

When a template has multiple variants and `--type` is omitted, the error message is: `template has no variant for type ""; available: ssh, container`. The actual problem is that the user must choose, not that type `""` is invalid.

**Fix**: Change to: `"template defines multiple variants; use --type to select one: %s"`.

---

### C9. AddWithCollisionHandling Mutates Caller's Input

**Source**: Correctness (IMPORTANT)
**File**: `internal/ws/definition.go:262`

The method mutates `def.Name` on the passed-in pointer when auto-suffixing. The current call site handles this, but the API is a foot-gun for future callers.

**Fix**: Work on a local copy of the name and assign `def.Name` only at the end, or accept the definition by value.

---

## Minor Findings

### M1. Predictable Temporary File Path

**Source**: Security (MEDIUM)
**File**: `internal/ws/definition.go:123-124`

Uses `s.path + ".tmp"` instead of `os.CreateTemp`. On multi-user systems, a symlink at the `.tmp` path could redirect writes.

**Fix**: Use `os.CreateTemp(dir, filepath.Base(s.path)+".*.tmp")`.

---

### M2. Stale Help Text References `.cc-deck/workspace.yaml`

**Source**: Spec Compliance
**File**: `internal/cmd/ws.go` lines 45, 676, 743, 817, 1140, 1300, 1341

Multiple help/long-description strings still reference the old `.cc-deck/workspace.yaml` pattern. Functionally correct, but confusing for users reading `--help`.

---

### M3. Auto-Suffixed Name Not Re-Validated

**Source**: Correctness + Security
**File**: `internal/ws/definition.go:262`, `internal/cmd/ws.go:285`

`ValidateWsName` is called before `AddWithCollisionHandling`, but the auto-suffixed name (e.g., `foo-ssh`) is not re-validated. A name at the 40-character limit would exceed it after suffixing.

---

### M4. File Permissions: 0o644 vs 0o600

**Source**: Security (LOW)
**File**: `internal/ws/definition.go:124`

Workspace definitions file is world-readable. Contains SSH hosts, identity file paths, kubeconfig paths. Should use `0o600` for consistency with compose env file.

---

### M5. Missing Integration Tests for Auto-Suffix, Subdirectory Resolution, Prune

**Source**: Testing (IMPORTANT)
- Same-name + different-type auto-suffix through `runWsNew` (US3 scenario 3)
- Subdirectory ancestor matching through `resolveWorkspaceName` (FR-006)
- `ws prune` command (no-op, zero tests)

---

### M6. Stale Comment in gitignore.go

**Source**: Spec Compliance
**File**: `internal/ws/gitignore.go:13`

Comment says "required entries (status.yaml and run/)" but `status.yaml` is no longer in the entries list.

---

### M7. ws.go is a 1837-Line Monolith

**Source**: Architecture (IMPORTANT)
**File**: `internal/cmd/ws.go`

15+ subcommands, resolution helpers, formatting, and output types all in one file. Not blocking for this PR, but worth splitting in a follow-up.

---

## Info / Observations

- **I1**: `DefinitionStore` reloads from disk on every operation. Fine for CLI tool, but documents a single-writer assumption. Add a comment about concurrency model.
- **I2**: `WorkspaceDefinition` has two `yaml:"-"` transient fields (`ExtraRemotes`, `AutoDetectedURL`) that belong to creation-time options, not the definition.
- **I3**: Template validation excludes `local` as a variant type. Probably intentional (local workspaces need no config), but not documented.
- **I4**: Placeholder in numeric fields (e.g., `port: {{my_port}}`) works via YAML coercion but produces confusing errors if user enters non-numeric values.
- **I5**: Symlink resolution in `FindByProjectDir` is correctly implemented. No vulnerability.
- **I6**: Command injection surface (exec.Command with separate args) is properly handled.
- **I7**: `os.Chdir` in tests creates latent flaky risk if `t.Parallel()` is ever added.

---

## Recommended Fix Priority

| Priority | ID | Description | Effort |
|----------|----|-------------|--------|
| **Must fix** | C1 | Phase-1 fallthrough to phase 2 | S |
| **Must fix** | C2 | Stale definition on failed Create | S |
| **Must fix** | C8 | Misleading multi-variant error message | XS |
| **Must fix** | M2 | Stale help text references | XS |
| **Should fix** | C3 | YAML injection via placeholders | M |
| **Should fix** | C4 | TemplateVariant field duplication | M |
| **Should fix** | C5 | Missing placeholder integration test | S |
| **Should fix** | C6 | Missing multi-ws no-recency test | S |
| **Should fix** | C7 | Missing sync-repos test | S |
| **Should fix** | C9 | AddWithCollisionHandling mutation | S |
| **Nice to have** | M1 | Predictable temp file path | XS |
| **Nice to have** | M3 | Re-validate auto-suffixed name | XS |
| **Nice to have** | M4 | File permissions 0o600 | XS |
| **Nice to have** | M5 | Additional integration tests | M |
| **Nice to have** | M6 | Stale gitignore comment | XS |
| **Follow-up** | M7 | Split ws.go monolith | L |

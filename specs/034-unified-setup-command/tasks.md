# Tasks: Unified Setup Command

**Branch**: `034-unified-setup-command` | **Plan**: [plan.md](plan.md)

## Phase 1: Package Rename and Manifest Evolution

### Task 1.1: Rename `internal/build` to `internal/setup`

**Priority**: P1 | **Estimate**: S | **Risk**: Low
**Files**: `cc-deck/internal/build/*` -> `cc-deck/internal/setup/*`, all import references
**Spec**: FR-017

Rename the `internal/build` package to `internal/setup`. Update all import paths in `cmd/build.go` (renamed to `cmd/setup.go`), `main.go`, and any other files that reference the build package. Verify with `make test` and `make lint`.

**Acceptance**: `make test` passes, `make lint` passes, no references to `internal/build` remain.

---

### Task 1.2: Evolve manifest struct with Targets

**Priority**: P1 | **Estimate**: M | **Risk**: Low
**Files**: `cc-deck/internal/setup/manifest.go`
**Spec**: FR-012, data-model.md

Replace `Image ImageConfig` with `Targets *TargetsConfig`. Add `ContainerTarget` (preserving all `ImageConfig` fields plus `Registry`) and `SSHTarget` structs. Update `Validate()` to check target-specific required fields. Update `ImageRef()` and `BaseImage()` to read from `Targets.Container`. Rename manifest filename constant from `cc-deck-image.yaml` to `cc-deck-setup.yaml`.

**Acceptance**: Manifest loading, validation, and image ref methods work with the new struct. Existing tests updated. `make test` passes.

---

### Task 1.3: Rename CLI command from `image` to `setup`

**Priority**: P1 | **Estimate**: S | **Risk**: Low
**Files**: `cc-deck/internal/cmd/build.go` -> `cc-deck/internal/cmd/setup.go`, `cc-deck/cmd/cc-deck/main.go`
**Spec**: FR-017

Rename the cobra command from `image` to `setup`. Update command registration in `main.go`. Update all help text and usage strings. The subcommands (init, verify, diff) remain the same.

**Acceptance**: `cc-deck setup init`, `cc-deck setup verify`, `cc-deck setup diff` work. `cc-deck image` no longer exists. `make test` passes.

---

## Phase 2: Init Command Extension

### Task 2.1: Add `--target` flag to init command

**Priority**: P1 | **Estimate**: M | **Risk**: Low
**Files**: `cc-deck/internal/cmd/setup.go`
**Spec**: FR-001, FR-002

Add `--target` flag accepting `container`, `ssh`, or comma-separated combination. When omitted, generate full template with target sections commented out. When `container` specified, uncomment container section. When `ssh` specified, uncomment SSH section and scaffold Ansible role skeletons.

**Acceptance**: All four acceptance scenarios from User Story 1 pass.

---

### Task 2.2: Scaffold Ansible role skeletons

**Priority**: P1 | **Estimate**: M | **Risk**: Low
**Files**: `cc-deck/internal/setup/init.go`
**Spec**: FR-001, FR-007

When `--target ssh` is specified during init, create the Ansible directory structure: `roles/{base,tools,zellij,claude,cc_deck,shell_config,mcp}/tasks/main.yml`, `roles/{base,tools,zellij,claude,cc_deck,shell_config,mcp}/defaults/main.yml`, `group_vars/`, `site.yml` skeleton, `inventory.ini` template. Use `go:embed` for skeleton templates.

**Acceptance**: `cc-deck setup init --target ssh` creates complete directory structure. `cc-deck setup init --target container,ssh` creates both container and SSH artifacts.

---

### Task 2.3: Update manifest template for dual targets

**Priority**: P1 | **Estimate**: S | **Risk**: Low
**Files**: `cc-deck/internal/setup/init.go`, embedded template files
**Spec**: FR-002, FR-012

Update the embedded manifest template to include both `targets.container` and `targets.ssh` sections. Sections are commented/uncommented based on the `--target` flag. Update manifest filename from `cc-deck-image.yaml` to `cc-deck-setup.yaml`.

**Acceptance**: Generated manifest template matches the schema in `data-model.md`.

---

## Phase 3: Claude Command Updates

### Task 3.1: Update `/cc-deck.capture` command

**Priority**: P1 | **Estimate**: M | **Risk**: Low
**Files**: `cc-deck/internal/setup/commands/cc-deck.capture.md`
**Spec**: FR-004

Update the capture command to read from `cc-deck-setup.yaml` instead of `cc-deck-image.yaml`. Ensure it only modifies shared sections (tools, sources, plugins, mcp, github_tools, settings, network) and never touches the `targets` section. The capture behavior is otherwise identical.

**Acceptance**: Capture populates shared sections. Target sections remain unchanged after capture. Re-running capture shows existing selections.

---

### Task 3.2: Update `/cc-deck.build` for container target

**Priority**: P1 | **Estimate**: M | **Risk**: Low
**Files**: `cc-deck/internal/setup/commands/cc-deck.build.md`
**Spec**: FR-005, FR-006

Update the build command to require `--target container` or `--target ssh`. For `--target container`, preserve all existing Containerfile generation behavior but read from `targets.container` instead of `image`. Add `--push` support using `targets.container.registry`. Remove the separate `/cc-deck.push` command.

**Acceptance**: `/cc-deck.build --target container` generates and builds a Containerfile. `/cc-deck.build --target container --push` builds and pushes. Same self-correction loop as before.

---

### Task 3.3: Add `/cc-deck.build` SSH target section

**Priority**: P1 | **Estimate**: L | **Risk**: Medium
**Files**: `cc-deck/internal/setup/commands/cc-deck.build.md`
**Spec**: FR-005, FR-007, FR-008, FR-009, FR-013, FR-014, FR-015, FR-016

Add the `--target ssh` section to the build command. This section generates Ansible playbooks from the manifest (inventory.ini, group_vars/all.yml, site.yml, role task files), then runs `ansible-playbook`. Includes self-correction loop (up to 3 retries) for Ansible task failures. Must check for Ansible availability first (FR-015).

The generated playbooks must be:
- Idempotent (re-runnable without side effects)
- Standalone (runnable without Claude Code after convergence)
- Role-per-concern (base, tools, zellij, claude, cc_deck, shell_config, mcp)

**Acceptance**: All six acceptance scenarios from User Story 3 pass (including AS-6: role conflict diff-and-ask). Converged playbooks run standalone.

---

### Task 3.4: Remove `/cc-deck.push` command

**Priority**: P1 | **Estimate**: S | **Risk**: Low
**Files**: `cc-deck/internal/setup/commands/cc-deck.push.md` (delete), `cc-deck/internal/setup/embed.go`
**Spec**: FR-003

Delete the push command file. Update embed.go to remove the push command reference. Push functionality is now part of `/cc-deck.build --target container --push`.

**Acceptance**: Only two commands installed: `cc-deck.capture.md` and `cc-deck.build.md`. Push works via `--push` flag.

---

## Phase 4: SSH Bootstrap Simplification

### Task 4.1: Create lightweight probe

**Priority**: P1 | **Estimate**: S | **Risk**: Low
**Files**: `cc-deck/internal/ssh/probe.go` (new)
**Spec**: FR-018

Create a `Probe()` function in `internal/ssh` that runs `which zellij && which cc-deck && which claude` via SSH. Returns nil if all tools are found, or an error with "Host appears unprovisioned. Run 'cc-deck setup' first."

**Acceptance**: Probe passes on a provisioned host. Probe fails with clear message on an unprovisioned host.

---

### Task 4.2: Simplify SSH environment Create()

**Priority**: P1 | **Estimate**: M | **Risk**: Medium
**Files**: `cc-deck/internal/env/ssh.go`, `cc-deck/internal/ssh/bootstrap.go` (delete)
**Spec**: FR-018, FR-019

Replace the pre-flight bootstrap in `SSHEnvironment.Create()` with:
1. Validate SSH connectivity
2. Run lightweight probe
3. If probe fails, error with provisioning instructions
4. If probe passes, create workspace directory and register environment

Delete `internal/ssh/bootstrap.go` entirely. Allow multiple environments targeting the same SSH host (FR-019) by removing any single-host uniqueness checks if they exist.

**Acceptance**: `cc-deck env create` with SSH type uses probe instead of bootstrap. Multiple envs can target same host with different names/workspaces.

---

## Phase 5: Verify and Diff Commands

### Task 5.1: Add `--target` flag to verify command

**Priority**: P2 | **Estimate**: M | **Risk**: Low
**Files**: `cc-deck/internal/cmd/setup.go`
**Spec**: FR-010

Add `--target` flag to `cc-deck setup verify`. For `--target container`, preserve existing behavior (run checks inside container). For `--target ssh`, connect via SSH and run the same checks (cc-deck version, Claude Code, Zellij, tools from manifest).

**Acceptance**: Verify reports pass/fail per tool for both container and SSH targets.

---

### Task 5.2: Add `--target` flag to diff command

**Priority**: P3 | **Estimate**: M | **Risk**: Low
**Files**: `cc-deck/internal/cmd/setup.go`
**Spec**: FR-011

Add `--target` flag to `cc-deck setup diff`. For `--target container`, preserve existing Containerfile diff behavior. For `--target ssh`, compare manifest against Ansible role task files. Auto-detect target if `--target` not specified (check for Containerfile/roles existence).

**Acceptance**: Diff detects and reports drift for both target types.

---

## Phase 6: Documentation

### Task 6.1: Update README.md

**Priority**: P1 | **Estimate**: M | **Risk**: Low
**Files**: `README.md`
**Spec**: Constitution IX, X

Add spec 034 to the feature specifications table. Update the CLI reference section to replace `cc-deck image` with `cc-deck setup`. Document the `--target` flag and SSH provisioning workflow. Use prose plugin with cc-deck voice.

**Acceptance**: README reflects the new command structure. Spec table updated.

---

### Task 6.2: Update CLI reference documentation

**Priority**: P1 | **Estimate**: M | **Risk**: Low
**Files**: `docs/modules/reference/pages/cli.adoc`
**Spec**: Constitution IX

Add `cc-deck setup` command group with init, verify, diff subcommands. Document all flags including `--target`. Remove `cc-deck image` references. Use prose plugin.

**Acceptance**: CLI reference covers all new commands and flags.

---

### Task 6.3: Create Antora guide page for setup command

**Priority**: P2 | **Estimate**: L | **Risk**: Low
**Files**: `docs/modules/*/pages/setup.adoc` (new), `docs/modules/*/nav.adoc`
**Spec**: Constitution IX

Create a dedicated guide page covering: overview, prerequisites, container workflow, SSH workflow, dual-target workflow, troubleshooting. Add to nav.adoc. Use prose plugin.

**Acceptance**: Guide page provides complete user documentation for the setup command.

---

### Task 6.4: Update landing page

**Priority**: P2 | **Estimate**: S | **Risk**: Low
**Files**: Landing page repo (`cc-deck/cc-deck.github.io`)
**Spec**: Constitution IX

Add a feature card for the unified setup command on the Astro landing page. Use prose plugin.

**Acceptance**: Landing page shows the new feature.

---

## Phase 7: Testing

### Task 7.1: Unit tests for manifest evolution

**Priority**: P1 | **Estimate**: M | **Risk**: Low
**Files**: `cc-deck/internal/setup/manifest_test.go`

Test manifest loading, validation, and helper methods with the new Targets struct. Test both v1 (should fail gracefully) and v2 manifests. Test target-specific validation rules.

**Acceptance**: All manifest tests pass. Coverage for ContainerTarget and SSHTarget validation.

---

### Task 7.2: Unit tests for probe

**Priority**: P1 | **Estimate**: S | **Risk**: Low
**Files**: `cc-deck/internal/ssh/probe_test.go`

Test the probe function with mock SSH client. Test success case (all tools found) and failure case (tools missing).

**Acceptance**: Probe tests pass.

---

### Task 7.3: Integration test against SSH target

**Priority**: P2 | **Estimate**: L | **Risk**: High
**Files**: Manual testing

End-to-end test against a real SSH target (Hetzner VM). Run the full workflow: init, capture, build --target ssh, verify. Verify all 10 testing findings (F-001 through F-010) are resolved.

**Acceptance**: Full workflow completes. Playbooks converge. Standalone re-run succeeds. All F-001 through F-010 findings resolved.

---

## Dependency Graph

```
Phase 1 (1.1, 1.2, 1.3) - Package rename, manifest evolution, CLI rename
    ↓
Phase 2 (2.1, 2.2, 2.3) - Init command extension
    ↓
Phase 3 (3.1, 3.2, 3.3, 3.4) - Claude command updates
    ↓
Phase 4 (4.1, 4.2) - SSH bootstrap simplification
    ↓
Phase 5 (5.1, 5.2) - Verify and diff updates
    ↓
Phase 6 (6.1, 6.2, 6.3, 6.4) - Documentation
    ↓
Phase 7 (7.1, 7.2, 7.3) - Testing
```

Phases 1-3 are sequential (each depends on the previous). Phase 4 can run in parallel with Phase 3 (independent code paths). Phases 5-7 depend on Phases 1-4 being complete. Within phases, tasks can run in parallel.

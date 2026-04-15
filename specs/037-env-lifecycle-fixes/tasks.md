# Tasks: Environment Lifecycle Fixes

**Branch**: `037-env-lifecycle-fixes` | **Date**: 2026-04-14 | **Plan**: [plan.md](plan.md)

## Task List

### Phase 1: SSH Delete Cleanup

#### Task 1.1: Add definition removal to SSHEnvironment.Delete() [X]

**FR**: FR-005, FR-006
**File**: `cc-deck/internal/env/ssh.go`
**What**: Add `defs.Remove()` call after `store.RemoveInstance()` in the `Delete()` method, following the exact pattern from `ContainerEnvironment.Delete()` (container.go:362-366). Wrap in `if e.defs != nil` guard, log warning on failure.
**Tests**: Add test in `cc-deck/internal/env/ssh_test.go` that creates an SSH environment with a definition, deletes it, and verifies the definition is removed from the store. Also test that delete succeeds even when definition removal fails (best-effort).
**Acceptance**: `SSHEnvironment.Delete()` removes the definition. Ghost entries no longer appear in `cc-deck ls` after SSH delete.

---

### Phase 2: Type Resolution Fix

#### Task 2.1: Refactor type resolution to support global definition lookup [X]

**FR**: FR-001, FR-002, FR-002a, FR-003, FR-004
**File**: `cc-deck/internal/cmd/env.go` (function `runEnvCreate()`)
**What**: After name resolution (line 216) and before type resolution (line 222), insert a new decision branch:
- When an explicit name is provided AND `projDef != nil` AND `name != projDef.Name`: set `projDef = nil`, look up global definition via `defs.FindByName(name)`. If found, use its type and settings. If not found, fall back to `local` type.
- Introduce a `usedGlobalDef` flag. When true, skip scaffolding at line 241.
- Refactor settings application (lines 259-301) to apply from whichever definition was resolved (global or project-local), not exclusively from `projDef`.
**Tests**: In `cc-deck/internal/cmd/env_create_test.go`:
1. Explicit name matching global def creates environment with global def's type
2. Explicit name not in any store falls back to `local` type
3. Explicit name matching project-local def uses project-local (regression test)
4. `--type` flag overrides global definition's type
5. No `.cc-deck/environment.yaml` scaffolded when using global definition
**Acceptance**: `cc-deck env create marovo-test` from a project with a different local definition creates an SSH environment (matching global definition), not a compose environment (from project-local).

---

### Phase 3: --global and --local Flags

#### Task 3.1: Add --global and --local flags to env create [X]

**FR**: FR-012, FR-013, FR-014, FR-015
**File**: `cc-deck/internal/cmd/env.go`
**What**:
1. Add `global bool` and `local bool` fields to `createFlags` struct
2. Register `--global` and `--local` as boolean flags in `newEnvCreateCmd()`
3. Call `cmd.MarkFlagsMutuallyExclusive("global", "local")` for validation
4. In `runEnvCreate()`, before the resolution chain from Task 2.1:
   - If `--global`: require explicit name, call `defs.FindByName(name)`, error if not found (FR-013), set `projDef = nil`, use global definition, set `usedGlobalDef = true`
   - If `--local`: require `projDef != nil`, error if not found (FR-014), ignore global definitions
**Tests**:
1. `--global` selects global definition over project-local with same name
2. `--local` selects project-local definition over global with same name
3. `--global` with non-existent global definition returns error
4. `--local` without project-local definition returns error
5. `--global --local` rejected as mutually exclusive
6. `--global --type container` uses global definition settings but overrides type
**Acceptance**: Users can force definition resolution with `--global` or `--local` flags. Appropriate errors on missing definitions.

---

### Phase 4: List SOURCE Column

#### Task 4.1: Add SOURCE column and remove PATH from cc-deck ls [X]

**FR**: FR-007, FR-008, FR-010
**File**: `cc-deck/internal/cmd/env.go`
**What**:
1. Add `Source string \`json:"source" yaml:"source"\`` to `envListEntry` struct
2. In `writeEnvStructured()`: populate Source field: `"global"` for entries from global definitions, `"project"` for project-local entries, empty string for orphan instances (definition removed)
3. For state instances: determine source by checking `defs.FindByName(inst.Name)` (global) or matching against loaded project definitions (project)
4. In `writeEnvTableWithProjects()`: replace PATH column header and formatting with SOURCE column
5. Remove all PATH-related rendering logic
**Tests**:
1. Table output has SOURCE column header, no PATH column header
2. Global definition entry shows `global` in SOURCE
3. Project-local entry shows `project` in SOURCE
4. Orphan instance shows empty SOURCE
5. JSON output includes `"source"` key with correct values
6. YAML output includes `source` key with correct values
**Acceptance**: `cc-deck ls` shows SOURCE instead of PATH. Structured output includes `"source"` field.

---

### Phase 5: Status Project Path

#### Task 5.1: Add project path to cc-deck status output [X]

**FR**: FR-009
**File**: `cc-deck/internal/cmd/env.go`
**What**:
1. Add `ProjectPath string \`json:"project_path,omitempty" yaml:"project_path,omitempty"\`` to `envStatusOutput` struct
2. In `runEnvStatus()`: iterate `store.ListProjects()`, load each project's definition via `env.LoadProjectDefinition(p.Path)`, match by name to find the project path
3. In `writeEnvStatusText()`: add `Project:` line after existing fields when `ProjectPath` is non-empty
**Tests**:
1. Status output for project-local environment includes project path
2. Status output for global environment omits project path
3. JSON/YAML output includes `project_path` field for project-local environments
**Acceptance**: `cc-deck status smoke-full` shows the project directory path. Global environments show no project path.

---

### Phase 6: Documentation

#### Task 6.1: Update documentation and run final validation [X]

**FR**: All (documentation aspect)
**Files**: `README.md`, `docs/modules/reference/pages/cli.adoc`
**What**:
1. Add spec 037 entry to Feature Specifications table in `README.md`
2. Update CLI reference with `--global` and `--local` flag documentation for `env create`
3. Update `env list` documentation to reflect SOURCE column (replacing PATH)
4. Update `env status` documentation to mention project path output
5. Run `make test` and `make lint` for final validation
**Tests**: `make test` passes, `make lint` passes
**Acceptance**: Documentation is current. All tests pass. No lint errors.

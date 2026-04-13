# Tasks: cc-deck setup run

**Input**: Design documents from `/specs/036-setup-run-command/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

## Phase 1: Core Command (US1 + US2 - Container Build & SSH Provisioning, P1)

**Goal**: Add `cc-deck setup run` with auto-detection, container build execution, and SSH playbook execution

**Independent Test**: Run `cc-deck setup run` in a directory with a Containerfile or Ansible artifacts

### Tests

- [X] T001 [P] [US1,US2] Unit tests for target auto-detection logic in `cc-deck/internal/cmd/setup_test.go`: test Containerfile-only, site.yml+inventory.ini-only, both present (error), neither present (error)
- [X] T002 [P] [US1,US2] Unit tests for flag validation in `cc-deck/internal/cmd/setup_test.go`: test `--push` rejected with SSH target, `--push` without registry in manifest, `--target` with invalid value

### Implementation

- [X] T003 [US1,US2] Add `newSetupRunCmd()` function in `cc-deck/internal/cmd/setup.go`: cobra command with `[dir]` positional arg, `--target` and `--push` flags, target auto-detection from artifact presence, dispatch to `runContainerBuild()` or `runSSHProvision()`
- [X] T004 [US1] Add `runContainerBuild()` function in `cc-deck/internal/cmd/setup.go`: load manifest, detect runtime via `setup.DetectRuntime()`, get image ref via `Manifest.ImageRef()`, execute `<runtime> build -t <imageRef> -f Containerfile .` with stdout/stderr/stdin piped to terminal, set `Cmd.Dir` to setup directory, passthrough exit code
- [X] T005 [US2] Add `runSSHProvision()` function in `cc-deck/internal/cmd/setup.go`: check `ansible-playbook` on PATH via `exec.LookPath()`, execute `ansible-playbook -i inventory.ini site.yml` with stdout/stderr piped to terminal, set `Cmd.Dir` to setup directory, passthrough exit code
- [X] T006 [US1,US2] Wire `newSetupRunCmd()` into `NewSetupCmd()` via `cmd.AddCommand()` in `cc-deck/internal/cmd/setup.go`

**Checkpoint**: `cc-deck setup run` works for single-target setups (container or SSH)

---

## Phase 2: Push Support (US3 - Push Container Image, P2)

**Goal**: Add `--push` flag to build and push container images

**Independent Test**: Run `cc-deck setup run --push` with a manifest that has `targets.container.registry` set

### Implementation

- [X] T007 [US3] Add push logic to `runContainerBuild()` in `cc-deck/internal/cmd/setup.go`: validate `targets.container.registry` is set in manifest, construct push reference `<registry>/<name>:<tag>`, execute `<runtime> push <pushRef>` after successful build, passthrough exit code from push

**Checkpoint**: `cc-deck setup run --push` builds and pushes container images

---

## Phase 3: Documentation & Polish

**Purpose**: Documentation updates required by constitution Principle IX

- [X] T008 [P] Update README.md: add spec 036 to feature specifications table, update setup workflow section to include `setup run`
- [X] T009 [P] Update CLI reference in `docs/modules/reference/pages/cli.adoc`: add `setup run` command with synopsis, flags table, usage examples, and exit codes
- [X] T010 Run `make test` and `make lint` to verify all tests pass and code is clean

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1**: No dependencies, can start immediately
- **Phase 2**: Depends on T004 (container build function exists)
- **Phase 3**: Depends on Phase 1 + 2 completion

### Within Phase 1

- T001, T002 (tests) can run in parallel with each other
- T003 (command skeleton) must come first
- T004, T005 can run in parallel after T003
- T006 (wiring) after T003

### Parallel Opportunities

- T001 and T002 are independent test files, can be written in parallel
- T004 and T005 are independent functions, can be written in parallel
- T008 and T009 are independent doc files, can be written in parallel

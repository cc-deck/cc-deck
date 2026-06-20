# Tasks: Build Skill Iteration Reduction

**Input**: Design documents from `/specs/071-build-skill-fixes/`
**Prerequisites**: plan.md, spec.md, research.md, quickstart.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: No project setup needed. All changes are to existing files.

- [x] T001 Read current state of all 4 target files to understand exact content and line numbers in cc-deck/internal/build/commands/cc-deck.build.md, cc-deck/internal/build/commands/cc-deck.capture.md, cc-deck/internal/build/templates/containerfile/03-mandatory-stack.tmpl, cc-deck/internal/build/templates/containerfile/05-shell-finalize.tmpl

---

## Phase 2: User Story 1 - First-Try OpenShell Build (Priority: P1)

**Goal**: Eliminate all build-time failures by fixing instruction gaps in the build skill and templates

**Independent Test**: Run `/cc-deck.capture --all` + `/cc-deck.build --target openshell` and verify first-try success

### Template Changes (do first, they are referenced by skill instructions)

- [x] T002 [P] [US1] Fix cache directory ownership in cc-deck/internal/build/templates/containerfile/03-mandatory-stack.tmpl: change `chown -R {{.User}}:{{.User}} {{.HomeDir}}/.config/zellij {{.HomeDir}}/.cache/zellij {{.HomeDir}}/.claude` to include `{{.HomeDir}}/.cache` as parent chown target (FR-006, FR-018)
- [x] T003 [P] [US1] Add marketplace setup to cc-deck/internal/build/templates/containerfile/03-mandatory-stack.tmpl: add `claude plugins marketplace add anthropics/claude-plugins-official` after Claude Code install and before any plugin install commands (FR-007, FR-019)
- [x] T004 [P] [US1] Add Claude Code npm fallback to cc-deck/internal/build/templates/containerfile/03-mandatory-stack.tmpl: add commented fallback section after native installer showing npm install with prefix override for non-root users, triggered on exit 137 (FR-005)

### Build Skill Section C2 Changes

- [x] T005 [US1] Add USER root instruction to C2 assembly order in cc-deck/internal/build/commands/cc-deck.build.md: insert between step 1 (01-header.txt) and step 3 (system packages layer) stating that `USER root` must be added before any generated RUN layers when the base image runs as non-root user (FR-001)
- [x] T006 [US1] Add GitHub release asset verification protocol to C2 tool resolution in cc-deck/internal/build/commands/cc-deck.build.md: add subsection under "Tool resolution" for `install: github-release` requiring GitHub API query for actual asset names and tarball structure probe before generating download commands (FR-002, FR-003)
- [x] T007 [US1] Change snippet handling rule in C2 in cc-deck/internal/build/commands/cc-deck.build.md: replace "Copy content EXACTLY as-is" with "Copy as-is unless download command produces 404 or extraction error; verify against GitHub API and fix with comment" (FR-004)
- [x] T008 [US1] Add exact jq merge command to C2 settings handling in cc-deck/internal/build/commands/cc-deck.build.md: add the exact command `jq -s '.[0] as $orig | $orig * .[1] | .hooks = $orig.hooks'` under the settings merge strategy documentation (FR-009)
- [x] T009 [US1] Add post_install sandboxing protocol to C2 GitHub release tools section in cc-deck/internal/build/commands/cc-deck.build.md: document the 5-step protocol (create dirs as root, switch to sandbox, append || true, switch back to root, guard interactive prompts) (FR-010)

### Build Skill Section A2 Changes (mirror C2 changes for container builds)

- [x] T010 [P] [US1] Add GitHub release asset verification protocol to A2 tool resolution in cc-deck/internal/build/commands/cc-deck.build.md: same content as T006 but for Section A2 (FR-002, FR-003)
- [x] T011 [P] [US1] Add exact jq merge command to A2 settings handling in cc-deck/internal/build/commands/cc-deck.build.md: same content as T008 but for Section A2 settings merge (FR-009)
- [x] T012 [P] [US1] Add marketplace setup to A2 plugin handling in cc-deck/internal/build/commands/cc-deck.build.md: add marketplace add command before first plugin install (FR-007)
- [x] T013 [P] [US1] Add post_install sandboxing protocol to A2 GitHub release tools section in cc-deck/internal/build/commands/cc-deck.build.md: same content as T009 but for Section A2 (FR-010)

### Build Skill Key Rules Changes

- [x] T014 [US1] Add OpenShell base image documentation to Key Rules section in cc-deck/internal/build/commands/cc-deck.build.md: document that OpenShell base is Ubuntu (not Fedora), runs as sandbox (not root), and lacks lsd/starship/zsh/bat/ripgrep (FR-011)

**Checkpoint**: Build skill and templates now have all 11 build-time fixes. A fresh OpenShell build should handle USER root, asset verification, cache ownership, marketplace, jq merge, post_install protocol, snippet escape hatch, and Claude Code fallback correctly.

---

## Phase 3: User Story 2 - Shell Config Dependency Resolution (Priority: P1)

**Goal**: Detect and install implicit tool dependencies from shell config; guard runtime init scripts

**Independent Test**: Build an image with shell config referencing starship, lsd, fzf, compinit and verify all work at runtime

### Build Skill Changes

- [x] T015 [US2] Add shell config dependency scanning to C2 base image probing in cc-deck/internal/build/commands/cc-deck.build.md: add third probing step that scans curated shell config for commands in aliases/eval/source, cross-references with base image probe, and flags missing tools for installation (FR-008)

### Template Changes

- [x] T016 [US2] Guard starship init with TERM check in cc-deck/internal/build/templates/containerfile/05-shell-finalize.tmpl: change `echo 'eval "$(starship init '"$SHELL_NAME"')"' >> "$RC"` to `echo '[[ "$TERM" != "dumb" ]] && eval "$(starship init '"$SHELL_NAME"')"' >> "$RC"` (FR-017)

### Capture Skill Changes

- [x] T017 [P] [US2] Add shell config dependency scanning to Step 5c in cc-deck/internal/build/commands/cc-deck.capture.md: after the existing "Guard unresolved commands" section, add a new subsection that scans for implicit tool dependencies (starship, lsd, fzf, zoxide, bat in aliases/eval/source) and flags them in the manifest (FR-012)
- [x] T018 [P] [US2] Add fzf GitHub release detection to Step 5c in cc-deck/internal/build/commands/cc-deck.capture.md: when `source <(fzf --zsh)` is detected, flag fzf for GitHub release install instead of package manager (FR-013)
- [x] T019 [P] [US2] Add compinit preamble rule to Step 5c strip-out rules in cc-deck/internal/build/commands/cc-deck.capture.md: when stripping plugin managers, check for compdef/zstyle and add `autoload -Uz compinit && compinit -C` preamble (FR-014)

**Checkpoint**: Shell config dependencies are now detected at capture time and resolved at build time. Starship, fzf, lsd, and zsh completions all work in built images.

---

## Phase 4: User Story 3 - Capture-Time Verification (Priority: P2)

**Goal**: Verify asset patterns and post_install commands during capture for early feedback

**Independent Test**: Run `/cc-deck.capture` with a tool that has a wrong asset_pattern and verify the wizard corrects it

- [x] T020 [P] [US3] Add GitHub release asset verification to capture Step 11 in cc-deck/internal/build/commands/cc-deck.capture.md: after processing tools with `install: github-release`, query GitHub API for actual asset names and update manifest with verified patterns (FR-015)
- [x] T021 [P] [US3] Add post_install dry-run validation to capture Step 11 in cc-deck/internal/build/commands/cc-deck.capture.md: for each tool with a post_install command, run with --dry-run or --help to detect interactive prompts and warn (FR-016)
- [x] T022 [US3] Add build refresh verification note to cc-deck/internal/build/commands/cc-deck.build.md: document that `build refresh` must verify download URLs in regenerated snippets against GitHub APIs (FR-020)

**Checkpoint**: Capture wizard now validates asset patterns and post_install commands. build refresh verifies snippet URLs.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Validation and cleanup

- [x] T023 Run `make verify` to ensure no Go compilation or lint errors from template changes in cc-deck/internal/build/templates/containerfile/
- [x] T024 Verify all 13 changes are correctly placed by re-reading each modified section and cross-referencing against the brainstorm 072 change list

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies, start immediately
- **Phase 2 (US1)**: Templates (T002-T004) first, then C2 changes (T005-T009), then A2 mirrors (T010-T013), then Key Rules (T014)
- **Phase 3 (US2)**: Can start after Phase 2 (US2 edits different sections than US1)
- **Phase 4 (US3)**: Can start after Phase 2 (US3 edits capture.md which is independent)
- **Phase 5 (Polish)**: After all phases complete

### User Story Dependencies

- **US1 (P1)**: No dependencies on other stories. Template changes come first, then skill edits.
- **US2 (P1)**: Independent of US1. Edits different sections (C2 base image probing, Step 5c, template).
- **US3 (P2)**: Independent of US1 and US2. Edits capture Step 11 (separate from Step 5c).

### Within Each User Story

- Template edits before skill instruction edits (templates are referenced by skill instructions)
- C2 changes before A2 mirrors (C2 is the primary, A2 copies the pattern)

### Parallel Opportunities

- T002, T003, T004 can all run in parallel (different template sections)
- T010, T011, T012, T013 can run in parallel (different A2 subsections)
- T017, T018, T019 can run in parallel (different Step 5c subsections)
- T020, T021 can run in parallel (different Step 11 additions)
- US2 and US3 can run in parallel with each other (different files/sections)

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Read current files
2. Complete Phase 2: All build-time fixes
3. **STOP and VALIDATE**: Test with `/cc-deck.build --target openshell`
4. If first-try success for build-time issues: proceed to US2

### Incremental Delivery

1. US1: Build-time fixes (eliminates 7 of 13 failures)
2. US2: Shell config + runtime guards (eliminates 4 of 13 failures)
3. US3: Capture-time verification (eliminates 2 of 13 failures, early feedback)
4. Polish: Verify all changes compile and lint clean

---

## Notes

- All changes are markdown/template edits. No Go code compilation needed.
- `make verify` at the end catches template syntax errors.
- The 4 target files are: cc-deck.build.md (834 lines), cc-deck.capture.md (1032 lines), 03-mandatory-stack.tmpl (30 lines), 05-shell-finalize.tmpl (32 lines).

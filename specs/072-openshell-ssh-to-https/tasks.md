# Tasks: OpenShell SSH-to-HTTPS Git Clone Conversion

**Input**: Design documents from `/specs/072-openshell-ssh-to-https/`

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Go Code Changes (US1)

**Goal**: Convert SSH URLs to HTTPS in buildCloneCommand() for OpenShell workspaces

- [ ] T001 [US1] Add `sshToHTTPS()` function in cc-deck/internal/ws/repos.go that converts `git@<host>:<path>` to `https://<host>/<path>` and `ssh://git@<host>/<path>` to `https://<host>/<path>`
- [ ] T002 [US1] Add `convertSSH bool` parameter to `buildCloneCommand()` in cc-deck/internal/ws/repos.go and call `sshToHTTPS()` on the URL when true
- [ ] T003 [US1] Update all callers of `buildCloneCommand()` to pass `convertSSH: true` for OpenShell workspace type and `false` for other types
- [ ] T004 [US1] Add INFO log message when URL is converted: `"Converting SSH URL to HTTPS for OpenShell sandbox: <old> -> <new>"`

**Checkpoint**: `cc-deck ws new --type openshell --repo git@github.com:org/repo.git` clones successfully

---

## Phase 2: Unit Tests (US1)

- [ ] T005 [P] [US1] Add unit tests for `sshToHTTPS()` in cc-deck/internal/ws/repos_test.go: github.com, gitlab.com, bitbucket.org SSH URLs, custom hosts, ssh:// scheme, HTTPS passthrough, edge cases (no .git suffix, nested paths)
- [ ] T006 [P] [US1] Add unit tests for `buildCloneCommand()` with `convertSSH=true` and `convertSSH=false` in cc-deck/internal/ws/repos_test.go

---

## Phase 3: Build Skill Changes (US2)

**Goal**: Add insteadOf git config to OpenShell images

- [ ] T007 [US2] Add git insteadOf configuration to Section C2 assembly order in cc-deck/internal/build/commands/cc-deck.build.md: add `git config --global` commands for github.com, gitlab.com, and bitbucket.org SSH-to-HTTPS mappings in the user configuration layer

---

## Phase 4: Verify

- [ ] T008 Run `make verify` to ensure all tests pass and no lint errors in cc-deck/internal/ws/

---

## Dependencies

- T001 before T002 (function must exist before it's called)
- T002 before T003 (signature change before callers update)
- T005, T006 can run in parallel (different test functions)
- T007 is independent of T001-T006 (different file)
- T008 after all other tasks

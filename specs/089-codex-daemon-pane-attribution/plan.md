# Implementation Plan: Codex Shared Daemon Pane Attribution

**Branch**: `089-codex-daemon-pane-attribution` | **Date**: 2026-10-15 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/089-codex-daemon-pane-attribution/spec.md`

## Summary

Codex 0.157+ runs hooks inside a shared app-server daemon that carries the first client's Zellij identity, so `cc-deck hook --agent codex` forwards events to the wrong (often nonexistent) pane. The hook will detect that it runs under the daemon by its process ancestry, then attribute each event to the interactive Codex client that owns the session: first through a cached mapping keyed by client pid and start time, else by discovering the single interactive Codex client of the same user and Codex home whose working directory matches the payload `cwd`. Ambiguous or foreign matches are dropped, never misattributed. Delivery targets the client's own Zellij session. A new cgo-free `internal/procinfo` package provides process data on macOS (sysctl) and Linux (`/proc`); the pane-map cache becomes lock-protected and atomically written. Details and alternatives are in [research.md](research.md).

## Technical Context

**Language/Version**: Go 1.25 (CLI, `cc-deck/`); no Rust plugin change

**Primary Dependencies**: cobra (existing), `golang.org/x/sys/unix` v0.47.0 (already in the module graph, promoted from indirect to direct), `internal/xdg`, `internal/agent`, `internal/profile`

**Storage**: `~/.local/state/cc-deck/pane-map.json` (extended entries), new `~/.local/state/cc-deck/pane-map.lock`

**Testing**: `make test` (Go stdlib `testing`, testify); table tests over a fake `procinfo.Source`; parser tests for `/proc` fixtures and Darwin `procargs2` byte fixtures; a concurrency test for the cache; a benchmark for SC-004 and SC-005

**Target Platform**: macOS and Linux hosts (also Linux SSH and OpenShell workspaces); other platforms degrade to "no client found"

**Project Type**: CLI hook subcommand

**Performance Goals**: cache-hit path adds at most 20 ms per event (SC-004); discovery path completes within 500 ms with 10 Codex clients (SC-005)

**Constraints**: `CGO_ENABLED=0`; no subprocess per event on the hot path; hook must never disrupt Codex (silent exit on all errors, as today); no misattribution under any input (SC-003)

**Scale/Scope**: up to about 10 concurrent Codex clients per user; roughly 6 Go files touched or added, 3 documentation pages

## Global Constraints

Every task inherits these values from the spec and the repository rules:

- Codex 0.157 or later is the release line with the shared daemon; earlier releases and `--no-daemon` keep today's attribution (FR-003).
- Platforms: macOS and Linux (release targets `linux`/`darwin`, `amd64`/`arm64`), built with `CGO_ENABLED=0`.
- Users do not change how they start Codex, the Codex configuration, or the hook installation (FR-015).
- Never a wrong row: when attribution is uncertain, the event is dropped (SC-003). Several matching clients always drop the event, bound or not (spec review decision).
- Claude Code and OpenCode hook behavior and the existing hook test cases stay unchanged (FR-013, SC-007).
- Build only with `make test`, `make lint`, `make install`; state paths through `internal/xdg`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
|-----------|--------|----------|
| I. Tests and documentation | PASS (planned) | Unit tests for `procinfo` parsers, attribution table (contract tests 1 to 8), cache concurrency, benchmark; docs: `cli.adoc` (hook attribution and debug reasons), `configuration.adoc` (pane-map fields, lock file), `multi-agent.adoc` guide section on the Codex daemon; README note under agent support |
| II. Interface contracts | PASS | `procinfo.Source` is new; [contracts/codex-attribution.md](contracts/codex-attribution.md) defines hook behavior before implementation; existing `agent.Agent` interface is unchanged (`TranslateEvent` output gains no new required field) |
| III. Build and tool rules | PASS | Only `make test`, `make lint`, `make install`; `internal/xdg` for state paths; no container work |
| IV. Plugin debug logging | N/A | No plugin change; hook debug log extended per FR-014 |
| V. Command files are code | N/A | No `internal/build/commands/*.md` change |

No violations.

## Project Structure

### Documentation (this feature)

```text
specs/089-codex-daemon-pane-attribution/
├── plan.md              # This file
├── research.md          # R1 to R11
├── data-model.md        # procinfo.Process, codexClient, paneMapEntry, reasons
├── quickstart.md        # manual validation walkthrough
├── contracts/
│   └── codex-attribution.md
└── tasks.md             # /speckit-tasks
```

### Source Code (repository root)

```text
cc-deck/internal/
├── procinfo/                       # NEW package
│   ├── procinfo.go                 # Process, Source, ErrUnsupported, Default()
│   ├── procinfo_darwin.go          # sysctl kern.proc.uid / kern.proc.pid / kern.procargs2
│   ├── procinfo_linux.go           # /proc/<pid>/{stat,cmdline,environ,cwd}
│   ├── procinfo_other.go           # ErrUnsupported
│   ├── procargs.go                 # procargs2 parser (pure function, no build tag, testable on any OS)
│   └── *_test.go
├── cmd/
│   ├── hook.go                     # resolution order, delivery session, pane-map locking + atomic writes
│   ├── codex_attribution.go        # NEW: daemon detection, client derivation, discovery, cache validation
│   ├── codex_attribution_test.go   # NEW: contract tests 1 to 8 over a fake Source
│   ├── hook_debug.go               # session=, pane=, target_session= fields
│   └── hook_test.go                # concurrency test, unchanged existing table
└── agent/
    └── codex.go                    # parse optional client_env from the payload (exposed to cmd)

docs/modules/
├── reference/pages/cli.adoc            # hook: Codex daemon attribution, new debug reasons
├── reference/pages/configuration.adoc  # pane-map.json fields, pane-map.lock
└── using/pages/multi-agent.adoc        # Codex shared daemon section, same-directory rule, --no-daemon
README.md                               # one sentence under Codex support
```

**Structure Decision**: Existing monorepo layout. Process inspection lives in its own package so `cmd` stays testable through an interface and platform code is isolated by build tags. Codex-specific attribution sits next to the hook in `internal/cmd`, because it decides delivery, not event translation.

## Phase 0: Research

Complete. See [research.md](research.md). All Technical Context items are resolved; no NEEDS CLARIFICATION remains.

## Phase 1: Design

Complete:

- [data-model.md](data-model.md): process and client shapes, extended `paneMapEntry`, resolution reasons, mapping state transitions, files.
- [contracts/codex-attribution.md](contracts/codex-attribution.md): inputs, resolution order, delivery, cache rules, debug log, contract tests.
- [quickstart.md](quickstart.md): manual validation for US1 to US3, no-daemon control, Claude Code regression check.

### Constitution re-check after design

Unchanged: PASS. The new interface (`procinfo.Source`) and the hook contract are written before implementation; tests and documentation are enumerated per file above.

## Implementation Order (for /speckit-tasks)

1. `internal/procinfo`: types, Linux and Darwin sources, parsers with fixtures.
2. Pane-map locking (`syscall.Flock`, as in `internal/share/lock.go`) and atomic writes in `hook.go`, with the concurrency test (benefits all agents, FR-012).
3. `codex.go`: optional `client_env` in the payload.
4. `codex_attribution.go`: daemon detection, client derivation (dir, Codex home, Zellij identity, profile), cache validation, discovery, reasons; contract tests over a fake source.
5. Wire into `runHook`: resolution order, skip `not-in-zellij` for daemon-hosted events, `--session` delivery, profile enrichment from the client, extended debug log.
6. Benchmark for SC-004 and SC-005.
7. Documentation and `/prose:check`; manual quickstart run.

## Complexity Tracking

No constitution violations to justify.

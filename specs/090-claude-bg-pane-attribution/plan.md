# Implementation Plan: Claude Code Background Session Pane Attribution

**Branch**: `090-claude-bg-pane-attribution` | **Date**: 2026-10-18 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/090-claude-bg-pane-attribution/spec.md`

## Summary

Hooks of a Claude Code background job run inside the worker process that the daemon's PTY host started, so they carry the daemon's inherited pane identity (without `ZELLIJ`) and every event is dropped or would land in an unrelated row. `cc-deck hook --agent claude` will detect a background job by its ancestors (a worker with a verified `kind: "bg"` session record, or a `--bg-pty-host` process), read the job id from the worker's record, and find the job's viewers exactly: interactive TUIs whose record names the job in `parkedJobId`, and `claude attach <jobId>` processes, both with the worker's configuration directory and verified by pid plus start time. A single viewer terminal inside Zellij receives the event in its own Zellij session with its own profile; no viewer, several viewers, or a viewer outside Zellij drop the event. A per-job "last viewer pane" file drives a new `cc-deck:release-row` pipe that the plugin applies only when the row still belongs to the job's session. Delivered background payloads carry `background: true`, which the plugin shows as a dim `⇣` before the row name. Details and alternatives are in [research.md](research.md).

## Technical Context

**Language/Version**: Go 1.25 (CLI, `cc-deck/`); Rust (Zellij plugin, `cc-zellij-plugin/`, WASM target)

**Primary Dependencies**: cobra, `golang.org/x/sys/unix` (existing), `internal/procinfo` (spec 089, extended), `internal/xdg`, `internal/agent`, `internal/config`; Rust: serde, `unicode-width` 0.1, zellij-tile (existing)

**Storage**: new `~/.local/state/cc-deck/bg-job-panes.json`, short merge lock `bg-job-panes.lock`, and per-job `bg-job-<hash>.lock`; durable pending cleanup without age-only expiry (deep-review correction); `pane-map.json` unchanged in shape; plugin session state gains `background`

**Testing**: `make test` (Go `testing` plus testify, fake `procinfo.Source` and temporary records directories; Rust `cargo test` through the Makefile); `make lint`; benchmark for SC-004

**Target Platform**: macOS and Linux hosts and Linux workspaces (SSH, OpenShell); other platforms keep today's path (no ancestors readable means foreground)

**Project Type**: CLI hook subcommand plus Zellij sidebar plugin

**Performance Goals**: background resolution adds at most 150 ms with 30 Claude Code processes (SC-004); foreground events add at most the ancestor walk (a few sysctl or `/proc` reads and one small file read)

**Constraints**: `CGO_ENABLED=0`; no subprocess on the hook path except the existing `zellij pipe`; hook never disrupts Claude Code (silent on all errors); never a wrong row (SC-003); foreground payloads byte-identical to today (SC-005)

**Scale/Scope**: up to about 30 Claude Code processes per user; about 5 Go files and 6 Rust files touched, 4 documentation pages plus README

## Global Constraints

Every task inherits these values from the spec and the repository rules:

- Claude Code 2.1.296 or later is the release line with background sessions; record fields as in research R2.
- Platforms: macOS and Linux (`linux`/`darwin`, `amd64`/`arm64`), `CGO_ENABLED=0`.
- Users change nothing in how they start, park or attach sessions, the Claude Code configuration, or the hook installation (FR-015).
- Never a wrong row: missing, unverifiable or inconsistent data drops the event (FR-011, SC-003). Several viewer terminals always drop.
- Background events never read or write `pane-map.json` for attribution (FR-002); `SessionEnd` still removes a session's entry.
- Codex, OpenCode, `--raw`, and foreground Claude Code behavior stay unchanged; existing Go and Rust tests pass without modification (FR-013, SC-005).
- Build only with `make test`, `make lint`, `make install`; state paths through `internal/xdg`.
- Documentation uses the prose plugin with the `cc-deck` voice, no em-dashes or en-dashes, one sentence per line in AsciiDoc.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
|-----------|--------|----------|
| I. Tests and documentation | PASS (planned) | Go contract tests 1 to 18 and Rust tests 19 to 22 ([contract](contracts/claude-bg-attribution.md) section 8); docs: `cli.adoc` (Claude Code background attribution, reasons, release pipe), `configuration.adoc` (`bg-job-panes.json`, lock), `multi-agent.adoc` (new "Claude Code Background Sessions" section with rules, upstream issue, foreground workaround), `sidebar.adoc` (background marker), README sentence under Claude Code support |
| II. Interface contracts | PASS | `procinfo.Source` gains `ListUserMatching`; the existing fake in `codex_attribution_test.go` and both platform sources implement it, and `ListUser` keeps its contract. The hook and plugin behavior is fixed in [contracts/claude-bg-attribution.md](contracts/claude-bg-attribution.md) before implementation. `agent.Agent` is unchanged. |
| III. Build and tool rules | PASS | Only Makefile targets; `internal/xdg` for state paths; no container work |
| IV. Plugin debug logging | PASS | Plugin change (release pipe, marker) adds no new logging mechanism; existing debug logger may log release decisions |
| V. Command files are code | N/A | No `internal/build/commands/*.md` change |
| VI. Zellij is an implementation detail | PASS | Users run `claude`, `claude attach`, and `cc-deck` commands only; quickstart uses no `zellij` command; the `zellij pipe` call stays internal to the hook |

No violations.

## Project Structure

### Documentation (this feature)

```text
specs/090-claude-bg-pane-attribution/
├── plan.md              # This file
├── research.md          # R1 to R12
├── data-model.md        # records, host, viewers, reasons, bg-job-panes, payload field
├── quickstart.md        # manual validation Q1 to Q5
├── contracts/
│   └── claude-bg-attribution.md
└── tasks.md             # /speckit-tasks
```

### Source Code (repository root)

```text
cc-deck/internal/
├── procinfo/
│   ├── procinfo.go                    # StartUnix field, ListUserMatching in Source
│   ├── procinfo_darwin.go             # StartUnix, ListUserMatching (ListUser delegates)
│   ├── procinfo_linux.go              # btime from /proc/stat, StartUnix, ListUserMatching
│   ├── procinfo_other.go              # ListUserMatching returns ErrUnsupported
│   └── *_test.go                      # stat/btime fixtures, comm filter
├── agent/
│   ├── agent.go                       # NormalizedPayload.Background (omitempty)
│   └── claude.go                      # parse optional viewer_env / client_env
└── cmd/
    ├── claude_bg_attribution.go       # NEW: records, host walk, viewers, terminals, reasons, bg-job-panes, release
    ├── claude_bg_attribution_test.go  # NEW: contract tests 1 to 18, benchmark
    ├── hook.go                        # branch for --agent claude before the not-in-zellij exit; generic locked state file helper
    ├── hook_debug.go                  # logClaudeBackground, logRowRelease
    └── codex_attribution_test.go      # fake source gains ListUserMatching

cc-zellij-plugin/src/
├── pipe_handler.rs                    # HookPayload.background; PipeAction for cc-deck:release-row
├── session.rs                         # Session.background (serde default)
├── lib.rs                             # RenderSession.background (serde default)
├── controller/hooks.rs                # set background on every event (incl. paused rows)
├── controller/mod.rs                  # dispatch release-row to a conditional evict
├── controller/render_broadcast.rs     # map background into RenderSession
└── sidebar_plugin/render.rs           # dim ⇣ marker on line 1, prefix_len, legend entry

docs/modules/
├── reference/pages/cli.adoc
├── reference/pages/configuration.adoc
├── using/pages/multi-agent.adoc
└── using/pages/sidebar.adoc
README.md
```

**Structure Decision**: Existing monorepo layout. Background attribution sits next to the Codex attribution in `internal/cmd` (it decides delivery, not translation) and reuses its helpers (`validZellijSessionName`, `resolvePath`, `profileEnv`, `clientEnvTarget`, `deliverHookPayload` with a target session). Process data stays behind `procinfo.Source` so the resolver is testable with the existing fake. The plugin change follows the existing `badges`/`paused` path from payload to render.

## Phase 0: Research

Complete. See [research.md](research.md). No NEEDS CLARIFICATION remains. One open risk is tracked there (R2): whether the installed Claude Code release keeps `parkedJobId` while the TUI shows the job; quickstart Q1 decides it, and the design is safe either way.

## Phase 1: Design

Complete:

- [data-model.md](data-model.md): extended `procinfo.Process`, `claudeRecord`, host outcomes, worker and viewer shapes, reasons, `bg-job-panes.json` transitions, `background` payload field.
- [contracts/claude-bg-attribution.md](contracts/claude-bg-attribution.md): inputs, host detection, resolution order, delivery, row release pipe, marker, debug log, contract tests.
- [quickstart.md](quickstart.md): manual validation for US1 to US3, unchanged agents, latency.

### Constitution re-check after design

Unchanged: PASS. The extended interface and both sides of the new pipe are specified before implementation; tests and documentation are listed per file above.

## Implementation Order (for /speckit-tasks)

1. `internal/procinfo`: `StartUnix` (Darwin, Linux with `btime`), `ListUserMatching`, `ListUser` delegating; fixtures; fake source update.
2. `internal/agent`: `NormalizedPayload.Background`; `viewer_env`/`client_env` parsing for Claude Code.
3. `claude_bg_attribution.go`: record parsing and verification, ancestor walk, worker, viewer collection, terminal grouping, reasons; contract tests 1 to 13 and 17 over the fake source.
4. Generalize the locked, atomic state-file helpers in `hook.go` (pane-map unchanged in behavior); `bg-job-panes.json` transitions and the release send; contract tests 14 to 16.
5. Wire into `runHook` for `--agent claude` before the `not-in-zellij` exit; debug log lines (FR-014).
6. Plugin: `background` through payload, session, render payload, render; `cc-deck:release-row` handler with session check; Rust tests 19 to 22.
7. Benchmark for SC-004, results in `VERIFY.md`.
8. Documentation and `/prose:check`; quickstart run including Q1, updating research R2 and the docs with the observed `parkedJobId` behavior.

## Complexity Tracking

No constitution violations to justify.

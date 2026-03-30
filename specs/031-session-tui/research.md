# Research: Session TUI (Control Plane Dashboard)

**Feature**: 031-session-tui
**Date**: 2026-03-30

## R1: TUI Framework Selection

**Decision**: bubbletea v0.25+ with lipgloss v1.x and bubbles components

**Rationale**: bubbletea is the only Go TUI framework with native suspend/resume support (`tea.Suspend`/`tea.Resume`), which is critical for the attach model. The Charm stack (bubbletea + lipgloss + bubbles) provides ready-made table, text input, spinner, viewport, and help components that cover all UI needs.

**Alternatives considered**:
- tview: No suspend/resume; would require manual terminal state management
- tcell: Too low-level; would need custom widget development
- termui: Abandoned, no active maintenance

**New dependencies to add to go.mod**:
- `github.com/charmbracelet/bubbletea` v0.25+
- `github.com/charmbracelet/lipgloss` v1.x
- `github.com/charmbracelet/bubbles` (table, textinput, spinner, viewport, help, key)

## R2: Session Data Access for Local Environments

**Decision**: Read Zellij plugin's WASI cache file from the host filesystem

**Rationale**: The Zellij plugin writes session state to `/cache/sessions.json` (WASI path), which maps to `~/.config/zellij/plugins/cc_deck.wasm/cache/sessions.json` on the host. The file contains a `BTreeMap<u32, Session>` serialized as JSON. This is a read-only operation from the TUI's perspective.

**Session data format** (from `cc-zellij-plugin/src/session.rs`):
```json
{
  "<pane_id>": {
    "pane_id": 1,
    "session_id": "session-1",
    "display_name": "api-refactor",
    "activity": "Working",
    "tab_index": 0,
    "tab_name": "api",
    "working_dir": "/home/user/project",
    "git_branch": "feat/api-v2",
    "last_event_ts": 1711800000,
    "manually_renamed": false,
    "paused": false,
    "meta_ts": 0,
    "done_attended": false,
    "pending_tab_rename": false
  }
}
```

**Activity enum values**: `"Init"`, `"Working"`, `{"Waiting":"Permission"}`, `{"Waiting":"Notification"}`, `"Idle"`, `"Done"`, `"AgentDone"`

**Alternatives considered**:
- HTTP status endpoint inside container: Only available for container/K8s envs, not local. Phase 2+.
- Zellij pipe messages: Only works inside Zellij sessions, not from an external process.

## R3: Zellij Session Naming Convention

**Decision**: Use `cc-deck-<envname>` prefix (matching existing `zellijSessionPrefix = "cc-deck-"` in `local.go`)

**Rationale**: The existing codebase already uses this convention consistently. The TUI must match it for attach/detach operations. The spec clarification says "1:1 mapping using environment name" which is achieved through this deterministic prefix.

## R4: P1 Architecture (Direct Polling)

**Decision**: Direct polling in the TUI process, no daemon

**Rationale**: P1 ships without the daemon architecture. Each TUI instance polls independently using the existing `env.Environment.Status()` method for container environments and direct file reads for local session data. The daemon (shared polling over Unix socket) is Phase 2.

**Polling implementation**:
- Use bubbletea's `tea.Tick` for periodic status updates
- Local environments: call `ReconcileLocalEnvs()` + read sessions.json (2s interval)
- Container environments: call `env.Status()` which runs `podman inspect` (5s interval)
- Run reconciliation in a goroutine to avoid blocking the UI

## R5: Attach Model (Suspend/Resume)

**Decision**: Use `tea.Suspend` to release terminal, `syscall.Exec` for local, `os/exec.Command` for container

**Rationale**: The existing `LocalEnvironment.Attach()` uses `syscall.Exec` which replaces the process entirely. For the TUI's suspend/resume model, we need to spawn a child process instead so the TUI can resume after exit.

**Implementation pattern**:
```go
// In the bubbletea Update():
case attachMsg:
    cmd := tea.Sequence(
        tea.Suspend,
        tea.ExecProcess(exec.Command("zellij", "attach", sessionName), nil),
    )
    return m, cmd
```

`tea.ExecProcess` (bubbletea v0.25+) handles suspend, child process execution, and resume automatically.

**Alternatives considered**:
- Manual terminal restore: Error-prone, bubbletea handles this natively
- `syscall.Exec` (process replacement): Cannot resume TUI after exit

## R6: State Store Integration

**Decision**: Reuse `FileStateStore` and `DefinitionStore` directly from `internal/env`

**Rationale**: The TUI is part of the same binary. It imports and uses the same state management code as the CLI commands. No adapter layer needed.

**Key types**:
- `FileStateStore`: v1 records (local envs) + v2 instances (container/compose) + project registry
- `DefinitionStore`: environment definitions from `~/.config/cc-deck/environments.yaml`
- `resolveEnvironment()`: Factory function that finds env by name across both v1 and v2

## R7: Bubbletea Model Architecture

**Decision**: Single root model with view-specific sub-models and a shared state struct

**Rationale**: bubbletea uses a single `tea.Model` with `Init()`, `Update()`, and `View()`. For multiple views (list, detail, create wizard, help overlay), use a view enum and delegate to sub-models. Shared state (environment list, polling results) lives in the root model.

**Pattern**:
```go
type model struct {
    view      viewType          // list, detail, create, help
    envs      []envRow          // cached environment data
    list      listModel         // list view sub-model
    create    createModel       // create wizard sub-model
    help      helpModel         // help overlay
    confirm   *confirmModel     // optional confirmation dialog
    width     int               // terminal width
    height    int               // terminal height
    store     *env.FileStateStore
    defs      *env.DefinitionStore
}
```

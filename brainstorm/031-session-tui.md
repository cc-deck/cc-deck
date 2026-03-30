# 031: Session TUI (Control Plane Dashboard)

**Date:** 2026-03-30
**Status:** Brainstorm
**Depends on:** 023-env-interface, 024-tui-environment-manager (supersedes), 025-container-environment
**Builds on:** 09-cross-session-visibility, 030-single-instance-architecture

## Problem

Managing cc-deck environments today requires the CLI (`cc-deck env list`, `cc-deck env status <name>`, `cc-deck env attach <name>`). When running multiple environments across local, podman, and Kubernetes, the user has no unified, interactive view of what is happening. They must run multiple commands, remember environment names, and manually check which agents need attention.

The user also needs to start Zellij manually or run `cc-deck env attach` from a plain terminal. There is no single entry point that provides both visibility and action.

## Solution

A full-screen terminal UI that acts as the **control plane** for all cc-deck environments. The TUI runs outside of Zellij, in any terminal emulator (Ghostty, iTerm, Alacritty, etc.). It provides:

- A live overview of all environments and their contained agent sessions
- Full lifecycle management (create, start, stop, delete, rename, tag)
- Attach capability that suspends the TUI and hands the terminal to Zellij
- Data transfer operations (push, pull, harvest)
- Full-text search across environments, sessions, branches, and tags
- OS-level notifications when agents need attention
- On-demand session summaries via the Claude API (stretch goal)

The TUI replaces the previous brainstorm 024 (TUI Environment Manager), which was limited to a simple list/detail view without lifecycle management or the daemon architecture.

## Architecture

### Client-Daemon Model

The TUI uses a client-daemon architecture, similar to Zellij and tmux. This avoids redundant polling when multiple TUI instances run simultaneously.

```
Terminal 1              Terminal 2              Terminal 3
┌─────────────┐        ┌─────────────┐        ┌─────────────┐
│  TUI client │        │  TUI client │        │  TUI client │
│  (renders)  │        │  (renders)  │        │  (attached)  │
└──────┬──────┘        └──────┬──────┘        └──────┬──────┘
       │                      │                      │
       └──────────┬───────────┴──────────┬───────────┘
                  │  Unix domain socket  │
                  │  $XDG_RUNTIME_DIR/cc-deck/tui.sock
                  │  (or ~/.local/state/cc-deck/tui.sock)
                  │                      │
           ┌──────┴──────────────────────┴──────┐
           │         cc-deck daemon              │
           │                                     │
           │  Responsibilities:                  │
           │  - Poll environment status          │
           │  - Cache session data               │
           │  - Aggregate HTTP status endpoints  │
           │  - Fire OS notifications (once)     │
           │  - Broadcast state to TUI clients   │
           │  - Track which envs are attached    │
           └─────────────────────────────────────┘
```

**Auto-lifecycle:**
- The daemon starts automatically when the first TUI client connects
- Exits after the last client disconnects plus a grace period (30 seconds)
- PID file at `$XDG_RUNTIME_DIR/cc-deck/daemon.pid`
- No manual daemon management needed

**Communication protocol:**
- Unix domain socket with JSON-over-newline messages
- Client sends: commands (create, delete, start, stop, refresh, subscribe)
- Daemon sends: state updates (full or delta), notifications, command results
- Heartbeat every 5 seconds to detect stale clients

### Status Polling Strategy

The daemon polls environment status at different intervals based on environment type:

| Environment | Mechanism | Default Interval | Notes |
|-------------|-----------|-----------------|-------|
| Local | Check Zellij process + read sessions.json | 2s | Fast, no overhead |
| Container | `podman inspect` + HTTP status endpoint | 5s | Moderate overhead |
| Compose | `podman-compose ps` + HTTP status endpoint | 5s | Same as container |
| K8s Deploy | K8s API (Pod status) + HTTP status endpoint | 10s | Network latency |
| K8s Sandbox | K8s API (Pod status) | 10s | Ephemeral, less data |

**HTTP status endpoint** (for container and K8s environments):
- Runs inside the environment container alongside Zellij
- Listens on a configurable port (default: 8083, avoids conflict with Zellij WebUI on 8082)
- Reads `/cache/sessions.json` (Zellij plugin state) and optional status files
- Serves `GET /status` with JSON response (see Status File Design below)
- Lightweight: single-binary Go HTTP server, started by the container entrypoint

For K8s environments, the HTTP endpoint is accessible via:
- `kubectl port-forward` (the daemon manages the tunnel)
- OpenShift Route (if exposed)
- The same networking path used for Zellij WebUI access

### Attach Model (Suspend/Resume)

When the user selects an environment and presses Enter:

1. The TUI sends `tea.Suspend` (bubbletea v0.25+)
2. bubbletea releases the terminal (restores original terminal state)
3. The TUI spawns the attach command as a child process:
   - Local: `zellij attach --create <session-name>`
   - Container: `podman exec -it <name> zellij attach --create <session-name>`
   - K8s: `kubectl exec -it <pod> -- zellij attach --create <session-name>`
4. The child process takes over the terminal (user is now inside Zellij with sidebar)
5. When the user exits Zellij (`Ctrl+Q`) or detaches, the child process exits
6. bubbletea receives `tea.Resume`, reclaims the terminal
7. TUI refreshes status and redraws

The daemon is notified of attach/detach events so it can:
- Mark the environment as "attached" in the UI across all TUI instances
- Skip redundant polling for the attached environment (local status is visible in the sidebar)

### Environment Creation from TUI

The TUI can create new environments without the user ever touching the CLI:

**Local environment:**
1. TUI calls `zellij --layout cc-deck --session <name>` to start a Zellij session
2. Registers it in state.yaml
3. Immediately attaches (suspend/resume)

**Container/Compose environment:**
1. TUI presents create wizard (name, image, storage, path, tags)
2. Daemon executes `cc-deck env create` logic (reusing the existing Go code)
3. On success, optionally auto-attaches

**K8s environment:**
1. Same wizard with K8s-specific fields (namespace, profile, storage size)
2. Daemon creates the StatefulSet/Pod
3. Waits for Pod to be Ready, then optionally auto-attaches

## UI Design

### Technology Stack

- **bubbletea** (v0.25+): TUI framework with suspend/resume support
- **lipgloss**: Styling, colors, borders
- **bubbles**: Ready-made components (table, text input, spinner, viewport)
- **bubbletea/key**: Key binding definitions

### Main View: Environment List

```
 cc-deck                                    3 running  1 stopped  1 creating  $34.20
 ──────────────────────────────────────────────────────────────────────────────────────

   NAME              TYPE        STATUS       SESSIONS   STORAGE    ATTACHED     TAGS
 ▸ my-project        container   ● running    3/3 ●      volume     5m ago       dev, api
   backend-work      k8s         ● running    2/5 ⚠      pvc/10Gi   30m ago      prod
   eval-run-42       k8s         ● running    1/1 ●      emptyDir   never        eval
   old-project       container   ○ stopped    -          volume     3d ago       legacy
   new-setup         local       ◎ creating   -          host       -            -

 ──────────────────────────────────────────────────────────────────────────────────────
  ↑↓/jk navigate  Enter attach  n new  s status  / search  ? help  q quit
```

Design elements:
- **Header**: environment counts by state, total estimated cost across all environments
- **Sessions column**: `3/3 ●` = 3 sessions, all healthy. `2/5 ⚠` = 2 of 5 need attention
- **Tags**: user-defined labels for filtering and organization
- **Status indicators**: `●` running, `○` stopped, `◎` creating, `✕` error, `◌` unknown
- **Footer**: context-sensitive key hints, change based on selected row and view

### Detail View: Environment Status

Full-screen replacement when pressing `s` or `Tab` on a selected environment:

```
 cc-deck ❯ backend-work                                               k8s  prod  $12.45
 ──────────────────────────────────────────────────────────────────────────────────────

  Status     Running (5d 3h)            Namespace   cc-deck
  Storage    PVC (10Gi, gp3)            Profile     anthropic-prod
  Sync       git-harvest                Image       quay.io/cc-deck/demo:latest
  Last push  2h ago                     Harvested   30m ago

  SESSIONS                                                       2 working  1 ⚠  $12.45
  ────────────────────────────────────────────────────────────────────────────────────
    NAME              STATUS          BRANCH              LAST EVENT    COST     TOOL
  ▸ api-refactor      ⚠ Permission    feat/api-v2         2m ago        $4.80    Edit
    docs-update       ● Working       docs/quickstart     1m ago        $3.20    Bash
    bugfix-123        ✓ Done          fix/null-ptr        15m ago       $2.10    -
    test-runner       ● Working       test/integration    30s ago       $1.85    Grep
    data-migration    ○ Idle          feat/migrate        1h ago        $0.50    -

 ──────────────────────────────────────────────────────────────────────────────────────
  Enter attach  h harvest  p push  P pull  r rename  t tag  d delete  Σ report  Esc back
```

### Status Report (on-demand, press `Σ`)

From any view, pressing `Σ` generates a human-readable status report for the selected environment. This is a flowing prose summary of all sessions, not a per-session drill-down. The report highlights action items, completed work, and current progress.

```
 cc-deck ❯ backend-work ❯ Status Report                          generated 14:30:12
 ──────────────────────────────────────────────────────────────────────────────────────

  ⚠ ACTION REQUIRED: api-refactor is waiting for permission to edit
  src/auth/oauth2.go (feat/api-v2 branch). It has been implementing an OAuth2
  PKCE flow for the API gateway and needs approval to modify the auth module.

  docs-update is actively working on the quickstart guide (docs/quickstart
  branch), currently running shell commands to validate code examples. It has
  been updating installation steps and adding a new "Getting Started" section.
  No action needed.

  test-runner is running integration tests on the test/integration branch,
  currently searching through test fixtures. This session started 45 minutes
  ago and has been iterating on flaky test fixes.

  ✓ bugfix-123 completed its work on fix/null-ptr 15 minutes ago. It fixed a
  nil pointer dereference in the request handler middleware. Ready for harvest.

  data-migration has been idle for 1 hour on feat/migrate. It finished
  analyzing the schema diff but has not started writing migration code yet.

  Total spend: $12.45 across 5 sessions (4.5M input, 1.2M output tokens).
  The api-refactor session accounts for the largest share at $4.80.

 ──────────────────────────────────────────────────────────────────────────────────────
  r regenerate  y copy to clipboard  Esc back
```

**How it works:**

1. The TUI collects session state from the daemon cache (activities, branches, timestamps, costs)
2. For richer context, the daemon fetches recent activity from the HTTP status endpoint (or reads local session files for local environments)
3. The TUI calls the Claude API with a structured prompt containing all session data, requesting a concise status report in flowing prose
4. The report is displayed in a scrollable viewport
5. The user can copy it to the clipboard (`y`) for pasting into Slack, a standup, or a PR description

**The prompt instructs Claude to:**
- Lead with action items (permissions, errors) that need immediate attention
- Summarize each session's current activity and recent progress
- Mention completed sessions and what they accomplished
- Include cost breakdown
- Use a professional, concise tone (no filler, no emojis)
- Highlight anything unusual (long idle times, high cost sessions, errors)

This replaces the per-session info panel concept. Instead of drilling into individual sessions, the user gets a single, actionable overview of everything happening in an environment. This is more useful in practice because the user typically wants to know "what needs my attention and what is everyone doing" rather than diving into one session's tool call history.

The report can also be generated for **all environments at once** by pressing `Σ` from the list view (no environment selected), producing a cross-environment status report.

### Create Wizard

```
 cc-deck ❯ New Environment
 ──────────────────────────────────────────────────────────────────────────────────────

  Name        █my-new-project▊
  Type        ○ local  ● container  ○ compose  ○ k8s

  ── Container Settings ──────────────────────────────────────────────────────────────
  Image       quay.io/cc-deck/cc-deck-demo:latest
  Storage     ○ bind mount  ● named volume
  Source      ~/Development/my-project
  Tags        dev, frontend

  ── Advanced ────────────────────────────────────────────────────────────────────────
  Ports       8082:8082, 8083:8083
  Auth        anthropic (from profile)
  Domains     default (ai-providers + package-registries)

  ──────────────────────────────────────────────────────────────────────────────────
  Tab/↓ next  Shift+Tab/↑ prev  Enter create  Esc cancel
```

The wizard adapts fields based on the selected type:
- **local**: Only name and tags
- **container**: Image, storage, source path, ports, auth
- **compose**: Same as container + domain filtering, sidecars
- **k8s**: Namespace, profile, storage size/class, sync strategy

### Search Mode

```
 cc-deck                                                              Search: api▊
 ──────────────────────────────────────────────────────────────────────────────────────

   NAME              TYPE        STATUS       SESSIONS   STORAGE    ATTACHED     TAGS
 ▸ my-project        container   ● running    3/3 ●      volume     5m ago       dev, api
   backend-work      k8s         ● running    2/5 ⚠      pvc/10Gi   30m ago      prod

 ──────────────────────────────────────────────────────────────────────────────────────
  2 of 5 matching "api"  (searches: name, tags, sessions, branches)  Esc clear
```

Full-text search across all fields. The search runs client-side against the cached state, so it is instant. Fuzzy matching (like fzf) would be ideal.

### Confirmation Dialogs

Destructive operations show inline confirmations:

```
  Delete environment "old-project"?
  This will remove the container and named volume.
  Data cannot be recovered.

  Type the environment name to confirm: █old-project▊

  Enter confirm  Esc cancel
```

### Help Overlay

```
 ┌─ cc-deck Help ──────────────────────────────────────────────────────────────────┐
 │                                                                                  │
 │  NAVIGATION              LIFECYCLE              DATA                            │
 │  ↑↓ / j k  Move          n  New environment     h  Harvest (git)                │
 │  Enter     Attach         S  Start               p  Push files                  │
 │  s / Tab   Detail view    X  Stop                P  Pull files                  │
 │  Esc       Back           d  Delete              e  Exec command                │
 │  g / G     Top / Bottom                                                         │
 │                           EDIT                   DISPLAY                         │
 │  SEARCH                   r  Rename              1  All types                   │
 │  /         Start search   t  Add/edit tags       2  Local only                  │
 │  Esc       Clear search                          3  Container only              │
 │                           REPORT                 4  Compose only                │
 │  GLOBAL                   Σ  Status report       5  K8s only                    │
 │  ?         This help                             R  Refresh                     │
 │  q         Quit                                                                 │
 │                                                                                  │
 └──────────────────────────────────────────────────────────────────────────────────┘
```

## Key Bindings

### Global (all views)

| Key | Action |
|-----|--------|
| `q` / `Ctrl+C` | Quit TUI |
| `?` / `F1` | Help overlay |
| `/` | Search / filter mode |
| `Esc` | Back / clear search / close dialog |
| `R` | Force refresh all status |
| `1` | Show all environment types |
| `2` | Filter: local only |
| `3` | Filter: container only |
| `4` | Filter: compose only |
| `5` | Filter: k8s only |

### List View

| Key | Action |
|-----|--------|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `g` | Go to first row |
| `G` | Go to last row |
| `Enter` | Attach to selected environment |
| `s` / `Tab` | Open detail view |
| `n` | Create new environment |
| `d` | Delete environment (with confirmation) |
| `S` | Start stopped environment |
| `X` | Stop running environment |
| `r` | Rename environment |
| `t` | Edit tags |
| `h` | Harvest (git) |
| `p` | Push files |
| `P` | Pull files |
| `e` | Exec command (opens input) |

### Detail View

| Key | Action |
|-----|--------|
| `↑` / `k` | Move cursor up (sessions) |
| `↓` / `j` | Move cursor down (sessions) |
| `Enter` | Attach to environment |
| `Σ` (Shift+S) | Generate status report |
| `h` | Harvest |
| `p` | Push |
| `P` | Pull |
| `r` | Rename environment |
| `t` | Edit tags |
| `d` | Delete |
| `Esc` | Back to list view |

### Status Report View

| Key | Action |
|-----|--------|
| `r` | Regenerate report |
| `y` | Copy report to clipboard |
| `Esc` | Back to previous view |

## Notifications

### In-TUI Indicators

- **Header bar**: Shows aggregate attention count: `⚠ 3 agents need attention`
- **List rows**: Environments with attention-needing sessions show `⚠` in the sessions column
- **Detail rows**: Individual sessions show their status with appropriate indicators

### OS Notifications

The daemon fires OS-level notifications when:
- An agent transitions to "Permission" state (needs human input)
- An environment stops unexpectedly (error state)

Implementation:
- **macOS**: `osascript -e 'display notification'` or `terminal-notifier`
- **Linux**: `notify-send` (libnotify)
- Configurable: can be disabled in config (`notifications.os: false`)
- Deduplicated: one notification per state transition, not per poll cycle

## Status File Design

### In-Container HTTP Endpoint

A lightweight Go HTTP server (`cc-deck-status`) runs inside the environment container:

```
GET /status          Full status JSON
GET /status/sessions Sessions only
GET /status/costs    Cost aggregation only
GET /sessions/{name}/context    Recent activity context for a session (on demand)
```

### Status Response Schema

```json
{
  "version": 1,
  "environment": "backend-work",
  "hostname": "cc-deck-backend-work-0",
  "uptime_seconds": 18720,
  "zellij_session": "cc-deck",
  "sessions": [
    {
      "name": "api-refactor",
      "pane_id": 1,
      "tab_index": 0,
      "activity": "Permission",
      "activity_detail": "Edit src/auth/oauth2.go",
      "tool": "Edit",
      "branch": "feat/api-v2",
      "last_event": "2026-03-30T14:28:00Z",
      "started_at": "2026-03-30T09:15:00Z",
      "costs": {
        "input_tokens": 1250000,
        "output_tokens": 380000,
        "cache_read_tokens": 5200000,
        "cache_write_tokens": 150000,
        "estimated_cost_usd": 4.80
      }
    }
  ],
  "totals": {
    "session_count": 5,
    "active_count": 2,
    "attention_count": 1,
    "estimated_cost_usd": 12.45,
    "input_tokens": 4500000,
    "output_tokens": 1200000
  }
}
```

### Data Sources Inside the Container

The status server reads from:

1. **`/cache/sessions.json`**: Zellij plugin state (session names, activities, pane IDs, branches)
2. **Claude session directories** (`~/.claude/projects/.../sessions/`): For cost data, token counts, and message history. The cc-spex plugin already writes status files to these directories with structured data.
3. **Process inspection**: `pgrep` for Zellij/agent health checks

Cost calculation:
- Read JSONL conversation files from Claude session directories
- Sum input/output/cache tokens per session
- Apply model pricing (configurable, with defaults for Claude Opus/Sonnet/Haiku)

### On-Demand Session Context

The `/sessions/{name}/context` endpoint reads recent activity from the Claude session directory (conversation JSONL, tool calls, timestamps) and returns a structured summary suitable for feeding into a status report prompt. This avoids shipping full conversation files over the network.

The actual AI-generated status report is produced by the TUI client, not the in-container server. The TUI:
1. Fetches session state and costs from `GET /status`
2. Fetches activity context from `GET /sessions/{name}/context` for each session
3. Calls the Claude API with all context, requesting a flowing prose status report
4. Displays the report in a scrollable viewport

This keeps the in-container server simple (data only, no AI calls) and gives the TUI control over the prompt and output formatting.

## Implementation Phases

### Phase 1: Core TUI (MVP)

- Environment list view with status polling
- Attach (suspend/resume) for local and container environments
- Create wizard (local + container types)
- Start/stop/delete operations
- Tags and rename
- Basic search
- Direct polling (no daemon yet, each TUI instance polls independently)

### Phase 2: Daemon + Notifications

- Client-daemon architecture with Unix socket
- Shared polling across TUI instances
- OS-level notifications
- K8s environment support in TUI
- Compose environment support in TUI

### Phase 3: Rich Status

- HTTP status endpoint in container image
- Cost tracking and display
- Session detail view
- Recent activity feed

### Phase 4: AI-Powered Features (Stretch)

- On-demand status reports via Claude API (flowing prose summary of all sessions)
- Cross-environment status reports from list view
- Copy-to-clipboard for standup notes, Slack, PR descriptions
- Smart suggestions (e.g., "3 agents done, ready to harvest")

## Technology Decisions

### Go + Charm Stack

- **bubbletea** (v0.25+): Main TUI framework. Suspend/resume support is critical for the attach model.
- **lipgloss** (v1.x): Styling, adaptive colors (responds to terminal light/dark mode)
- **bubbles**: Table, text input, spinner, viewport, help components
- **bubbletea/key**: Declarative key binding definitions with help text generation

### Integration with Existing Code

The TUI reuses the existing cc-deck Go packages:
- `internal/env`: Environment interface, factory, state management
- `internal/podman`: Podman container operations
- `internal/compose`: Compose file generation and runtime
- `internal/config`: Profile and configuration management
- `internal/xdg`: XDG path resolution

The TUI does not duplicate any environment management logic. It calls the same code paths as the CLI commands.

### Binary Integration

The TUI is part of the `cc-deck` binary, launched via `cc-deck tui` (subcommand, not a separate binary). This keeps installation simple and ensures the TUI always matches the CLI version.

For the daemon, the same binary runs in daemon mode: `cc-deck tui --daemon` (started automatically by the TUI client, not manually).

## Open Questions

1. **Daemon protocol**: JSON-over-newline on a Unix socket is simple. Should we consider gRPC for type safety and streaming? Probably overkill for v1, but worth noting.

2. **Cost model accuracy**: Claude API pricing changes. Should the cost calculation be configurable per-model, or hardcode current prices with a version check?

3. **Multi-cluster K8s**: If the user has environments across multiple K8s clusters (different kubeconfigs), the daemon needs to manage multiple API clients. This adds complexity. Should v1 support only one kubeconfig at a time?

4. **Zellij session naming**: When the TUI creates a local Zellij session, should it use the environment name as the Zellij session name? This creates a 1:1 mapping that simplifies attach/detach.

5. **Attach to specific session**: Should Enter in the detail view (on a specific agent session) attach and also focus that tab in Zellij? This would require passing the tab index to the `zellij attach` command or sending a pipe message after attach.

6. **Config for key bindings**: Should key bindings be user-configurable (like Zellij itself)? This adds complexity but follows the terminal tool convention.

7. **Theme support**: Should the TUI support user-defined color themes? lipgloss makes this easy, but it is more work to design a theme system.

8. **Status endpoint security**: The HTTP status endpoint inside containers has no authentication. For K8s environments accessible via Route, should there be a bearer token or mTLS? Or is the assumption that the Route is not publicly exposed?

9. **Daemon state persistence**: Should the daemon persist its cache to disk so that a TUI client can show stale-but-informative data immediately on connect, even before the first poll completes?

10. **Terminal multiplexer agnosticism**: The design is tightly coupled to Zellij. Should there be an abstraction layer that could support tmux in the future? Probably not for v1, but worth considering for the interface design.

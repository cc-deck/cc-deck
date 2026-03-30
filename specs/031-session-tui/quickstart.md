# Quickstart: Session TUI Implementation

**Feature**: 031-session-tui
**Date**: 2026-03-30

## Prerequisites

- Go 1.25+ (from go.mod)
- Zellij installed (for local environment testing)
- Podman installed (for container environment testing)
- At least one cc-deck environment created (`cc-deck env create test-env`)

## Setup

### 1. Add bubbletea dependencies

```bash
cd cc-deck
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/bubbles@latest
```

### 2. Create TUI package structure

```
cc-deck/internal/tui/
├── model.go        # Root model, Init/Update/View, view routing
├── keys.go         # Key binding definitions
├── styles.go       # lipgloss style definitions
├── list.go         # Environment list view
├── create.go       # Create wizard view
├── help.go         # Help overlay
├── confirm.go      # Confirmation dialog
├── polling.go      # Status polling logic (tea.Tick commands)
├── session.go      # Plugin session data reader (sessions.json parser)
└── envrow.go       # envRow builder (merges records + instances + defs)
```

### 3. Add the `tui` subcommand

In `cc-deck/internal/cmd/`, add `tui.go`:

```go
func NewTuiCmd(gf *GlobalFlags) *cobra.Command {
    return &cobra.Command{
        Use:   "tui",
        Short: "Launch the interactive environment dashboard",
        RunE: func(cmd *cobra.Command, args []string) error {
            return tui.Run()
        },
    }
}
```

Register it in the root command alongside `env`, `plugin`, etc.

### 4. Run and test

```bash
make install
cc-deck tui
```

## Key Implementation Notes

1. **Never import bubbletea in `internal/env`**. The TUI package imports env, not the other way around.

2. **Attach uses `tea.ExecProcess`**, not `syscall.Exec`. The existing `LocalEnvironment.Attach()` replaces the process. The TUI needs to spawn a child process so it can resume.

3. **Polling runs reconciliation functions** (`ReconcileLocalEnvs`, `ReconcileContainerEnvs`, `ReconcileComposeEnvs`) before listing. These are the same functions the CLI `env list` command uses.

4. **Session data parsing** must handle Rust serde enum format for the `activity` field (both string and object forms).

5. **The create wizard reuses existing `env.NewEnvironment()` + `Create()`** logic. Do not duplicate environment creation logic.

6. **Build via Makefile only** (Constitution Principle VI). Never run `go build` directly.

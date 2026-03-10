# Quickstart: Session Save and Restore

## Prerequisites

- cc-deck installed (`cc-deck plugin install`)
- Running inside a Zellij session with cc-deck layout
- Claude Code sessions active in one or more tabs

## Basic Usage

### Save before restart

```bash
# Save current workspace (auto-generated name)
cc-deck session save

# Save with a name
cc-deck session save my-projects
```

### Restart and restore

```bash
# Kill Zellij and restart
zellij kill-all-sessions -y
zellij --layout cc-deck

# In the new session, restore
cc-deck session restore

# Or restore a specific named snapshot
cc-deck session restore my-projects
```

### Manage snapshots

```bash
# List all snapshots
cc-deck session list

# Remove a specific snapshot
cc-deck session remove old-setup

# Remove all snapshots
cc-deck session remove --all
```

## Auto-save

Auto-save happens automatically via the hook system. Every 5 minutes (when Claude events are firing), the current state is saved. Up to 5 rolling auto-saves are kept. No configuration needed.

## What Gets Saved

- Tab names and order
- Working directories
- Claude Code session IDs (for resume)
- Display names (including manual renames)
- Pause state
- Git branch (informational)

## What Doesn't Get Saved

- Claude Code conversation history (managed by Claude itself)
- Terminal scrollback
- Zellij pane splits within tabs
- Running processes

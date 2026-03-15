#!/usr/bin/env bash
# quick-demo.sh - 20-second cc-deck sidebar demo driver
#
# Drives a tight automated demo using pipe commands + injected fake session
# states. No real Claude Code sessions required.
#
# SETUP:
#   1. Open a recording terminal and start Zellij inside asciinema:
#
#        asciinema rec --cols 160 --rows 40 --idle-time-limit 0.5 \
#          demos/recordings/quick-demo.cast \
#          -- zellij --session cc-quick --layout cc-deck
#
#   2. From a SECOND terminal, run this script:
#
#        bash demos/scripts/quick-demo.sh
#
#   3. Convert to GIF when done (exit Zellij to stop recording first):
#
#        agg --cols 160 --rows 40 \
#          demos/recordings/quick-demo.cast \
#          docs/modules/ROOT/images/quick-demo.gif
#
# The script injects 3 fake sessions into the sidebar (no Claude needed),
# then drives: attend cycling, navigation mode, pause, and the help overlay.
#
# Total runtime: ~22 seconds. Exit Zellij manually after the script completes.
#
# Zellij session name can be overridden: ZELLIJ_SESSION=myname bash quick-demo.sh

ZELLIJ_SESSION="${ZELLIJ_SESSION:-cc-quick}"

pipe() {
    local action="$1"
    local payload="${2:-}"
    if [[ -n "$payload" ]]; then
        zellij --session "$ZELLIJ_SESSION" pipe --name "cc-deck:${action}" -- "$payload"
    else
        zellij --session "$ZELLIJ_SESSION" pipe --name "cc-deck:${action}"
    fi
}

# ─── Session state JSON ────────────────────────────────────────────────────────
# Three fake sessions in different states, ordered by tab_index.
# Activity variants (serde): "Init" | "Working" | "Idle" | "Done" | "AgentDone"
#                             {"Waiting": "Permission"} | {"Waiting": "Notification"}

SESSIONS=$(cat <<'JSON'
{
  "101": {
    "pane_id": 101,
    "session_id": "demo-1",
    "display_name": "todo-api",
    "activity": {"Waiting": "Permission"},
    "tab_index": 0,
    "tab_name": "todo-api",
    "working_dir": "/tmp/cc-deck-demo/todo-api",
    "git_branch": "feat/search-endpoint",
    "last_event_ts": 3000,
    "manually_renamed": false,
    "paused": false,
    "meta_ts": 0,
    "done_attended": false
  },
  "102": {
    "pane_id": 102,
    "session_id": "demo-2",
    "display_name": "weather-cli",
    "activity": "Working",
    "tab_index": 1,
    "tab_name": "weather-cli",
    "working_dir": "/tmp/cc-deck-demo/weather-cli",
    "git_branch": "feat/json-flag",
    "last_event_ts": 2000,
    "manually_renamed": false,
    "paused": false,
    "meta_ts": 0,
    "done_attended": false
  },
  "103": {
    "pane_id": 103,
    "session_id": "demo-3",
    "display_name": "portfolio",
    "activity": "Done",
    "tab_index": 2,
    "tab_name": "portfolio",
    "working_dir": "/tmp/cc-deck-demo/portfolio",
    "git_branch": "feat/dark-mode",
    "last_event_ts": 1000,
    "manually_renamed": false,
    "paused": false,
    "meta_ts": 0,
    "done_attended": false
  }
}
JSON
)

# ─── Demo sequence (~22s) ──────────────────────────────────────────────────────

echo "Waiting 3s for Zellij to be ready..."
sleep 3

# Inject all three fake sessions into the sidebar
echo "[1/6] Seeding sidebar with 3 sessions..."
pipe "sync" "$SESSIONS"
sleep 2.5   # viewer sees: mixed states overview (permission, working, done)

# Smart attend: jumps to highest priority (todo-api: permission-waiting)
echo "[2/6] Alt+a -> permission session..."
pipe "attend"
sleep 2.5   # viewer sees: focus switches to todo-api

# Smart attend again: cycles to next (portfolio: done)
echo "[3/6] Alt+a -> done session..."
pipe "attend"
sleep 2     # viewer sees: focus switches to portfolio

# Enter navigation mode (amber cursor appears in sidebar)
echo "[4/6] Navigation mode..."
pipe "nav-toggle"
sleep 0.8
pipe "nav-down"
sleep 0.5
pipe "nav-down"    # cursor on portfolio
sleep 0.8

# Pause the session at cursor
echo "[5/6] Pause session..."
pipe "pause"
sleep 1.5   # viewer sees: portfolio dimmed with pause icon

# Exit navigation mode
pipe "nav-toggle"
sleep 0.8

# Show help overlay, then close
echo "[6/6] Help overlay..."
pipe "help"
sleep 2.5   # viewer sees: keyboard shortcut overlay
pipe "help"
sleep 0.8

echo "Done! Exit Zellij now to finish the asciinema recording."

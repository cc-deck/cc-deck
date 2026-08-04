# Follow-Up Tasks: Multiplayer Focus Modes Bug Fixes

**Context**: Implementation of spec 083 is code-complete (415 tests pass, clippy clean) but empirical testing with two terminals revealed 5 integration bugs. These notes are for the next session.

**Branch**: `083-multiplayer-focus-modes` (worktree at `.claude/worktrees/083-multiplayer-focus-modes`)
**Key files**: `specs/083-multiplayer-focus-modes/empirical-testing-notes.md` has full bug descriptions.

> 2026-07-24: F006, F009, F012-F017 were superseded by the client-view
> state-model refactor described in `empirical-testing-notes.md`. Automated
> coverage is complete; the remaining follow-up is the live two-terminal
> verification in F008, F011, F015, F018, and F019.

## Prerequisites (start of new session)

1. Read `specs/083-multiplayer-focus-modes/empirical-testing-notes.md` for all bug details
2. Enable debug logging:
   ```bash
   touch ~/Library/Caches/org.Zellij-Contributors.Zellij/file:~/.config/zellij/plugins/cc_deck.wasm/plugin_cache/debug_enabled
   ```
3. Truncate log before each test:
   ```bash
   : > ~/Library/Caches/org.Zellij-Contributors.Zellij/file:~/.config/zellij/plugins/cc_deck.wasm/plugin_cache/debug.log
   ```

## Bug Fix Tasks (priority order)

### Phase 1: Debug and Diagnose

- [ ] F001 Reproduce BUG-1 (stale indicators after disconnect) with debug logging enabled. Check the log for "CTRL FOCUS-REPORT" and "CTRL SIDEBAR cleaned" entries. Verify whether `cleanup_dead_sidebars` fires on disconnect and whether zombie sidebars prevent client_focus cleanup.

- [ ] F002 Reproduce BUG-4 (ModeUpdate timing). Check the log for "CTRL MODE_UPDATE: multiplayer colors extracted". Verify: does ModeUpdate arrive before or after the first render? Does it arrive at all on first client attach? If not, presence indicators should not render (the `multiplayer_colors.is_some()` guard should prevent it).

- [ ] F003 Reproduce BUG-5 (single color). With debug logging, check what color values are in `multiplayer_colors`. If None, indicators shouldn't show at all. If Some, check the RGB values for each player. If the indicator IS showing without palette, find the code path that bypasses the guard.

- [ ] F004 Reproduce BUG-3 (sidebar highlight). Check `local_focus_override` behavior: does it persist across render payload updates? Search for any code that clears `local_focus_override` (grep for "local_focus_override = None" or "local_focus_override.take()").

- [ ] F005 Reproduce BUG-2 (auto-sort rearranging). Check if `auto_sort_tail` is being modified by `handle_focus_report`. With debug logging, watch for "CTRL FOCUS-REPORT: auto-unpaused" entries that shouldn't be firing for non-paused sessions.

### Phase 2: Fix Stale Cleanup (BUG-1, highest priority)

- [ ] F006 Add `connected_clients` tracking to ControllerState. Zellij's `TabInfo` has `other_focused_clients: Vec<ClientId>`. Use `handle_tab_update` to track the set of active client_ids. When a client_id disappears from all tabs' `other_focused_clients`, remove its `client_focus` entry.

- [ ] F007 Alternative approach: use a timer-based sweep. In `handle_timer`, check `client_focus` entries. If a client_id has no sidebar in the registry that sent a `sidebar-hello` within the last 60 seconds, remove its focus entry.

- [ ] F008 Test the fix: attach second terminal, verify indicator appears, detach, verify indicator disappears within one render cycle.

### Phase 3: Fix Auto-Sort (BUG-2)

- [ ] F009 In `handle_focus_report` (controller/actions.rs), do NOT modify `auto_sort_tail` when unpausing. The auto-sort tail is for the primary client's sort view. Focus-reports from secondary clients should unpause the session but not affect the sort tail. Remove: `state.auto_sort_tail.retain(|&p| p != report.pane_id); state.auto_sort_tail.push(report.pane_id);`

- [ ] F010 Consider whether `handle_focus_report` should call `mark_render_dirty()` at all, or just update the `client_focus` map and let the next natural render cycle (timer tick) pick up the change. This would debounce rapid clicks.

- [ ] F011 Test: click sessions rapidly on client 2, verify client 1's sidebar order stays stable.

### Phase 4: Fix Sidebar Highlight (BUG-3)

- [ ] F012 Investigate `local_focus_override` lifecycle. The override should persist until the next explicit user action (click or keyboard select). Check if any code clears it on render payload update. If it gets cleared, the sidebar falls back to the payload's `focused_pane_id` (which is client 1's focus, not client 2's).

- [ ] F013 If `local_focus_override` IS being cleared: stop clearing it. For multiplayer, the sidebar must retain its local focus state permanently (until the next click).

- [ ] F014 If `local_focus_override` is NOT being cleared but highlighting still fails: check `effective_focused_pane_id()` to ensure it's used in all rendering paths, not just some.

- [ ] F015 Test: click session on client 2, verify it stays highlighted through multiple render cycles.

### Phase 5: Fix Color Issues (BUG-4, BUG-5)

- [ ] F016 If `ModeUpdate` never fires on first connection: add fallback colors. Use Zellij's default multiplayer palette (magenta, blue, purple, yellow, cyan, green, orange, gray, pink, brown) as hardcoded fallback when `multiplayer_colors` is None after a grace period (e.g., 5 seconds after first client_focus entry).

- [ ] F017 If indicators render without palette (guard bypass): find and fix the code path. The `build_other_client_focus` returns empty when `multiplayer_colors` is None. Check if there's another path that renders blocks.

- [ ] F018 Test: attach second terminal, verify TWO DIFFERENT colors appear for the two clients' indicators.

### Phase 6: Verify and Clean Up

- [ ] F019 Run `make install` and test all fixes with two terminals. Walk through quickstart.md V1-V7 scenarios.
- [ ] F020 Run `cargo test` and `cargo clippy -- -D warnings` to ensure no regressions.
- [ ] F021 Update `empirical-testing-notes.md` with fix results.
- [ ] F022 Run `make verify` if WASM binary is available.

## Key Code Locations

| Area | File | Line | What |
|------|------|------|------|
| Focus report handler | `cc-zellij-plugin/src/controller/actions.rs` | ~580 | `handle_focus_report()` |
| Client focus cleanup | `cc-zellij-plugin/src/controller/sidebar_registry.rs` | ~84 | `cleanup_dead_sidebars()` |
| Presence data builder | `cc-zellij-plugin/src/controller/render_broadcast.rs` | ~165 | `build_other_client_focus()` |
| Render broadcast | `cc-zellij-plugin/src/controller/render_broadcast.rs` | ~200 | `broadcast_render()` |
| ModeUpdate handler | `cc-zellij-plugin/src/controller/events.rs` | ~580 | `handle_mode_update()` |
| Local focus switch | `cc-zellij-plugin/src/sidebar_plugin/input.rs` | ~802 | `switch_focus_locally()` |
| Indicator rendering | `cc-zellij-plugin/src/sidebar_plugin/render.rs` | ~485 | presence indicator overlay |
| Local focus override | `cc-zellij-plugin/src/sidebar_plugin/state.rs` | ~67 | `local_focus_override` field |
| Effective focus | `cc-zellij-plugin/src/sidebar_plugin/state.rs` | ~190 | `effective_focused_pane_id()` |
| Local activation order | `cc-zellij-plugin/src/sidebar_plugin/state.rs` | ~160 | `apply_local_activation_order()` |

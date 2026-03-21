# SDD Handoff: 025-sidebar-state-refresh

## Feature
Sidebar State Refresh on Reattach

## Status
- [x] Brainstorm (`brainstorm/027-sidebar-state-refresh.md`)
- [x] Specification (`specs/025-sidebar-state-refresh/spec.md`)
- [x] Spec Review (passed with revisions applied)
- [x] Clarify (no critical ambiguities)
- [x] Plan (`specs/025-sidebar-state-refresh/plan.md`)
- [x] Tasks (`specs/025-sidebar-state-refresh/tasks.md`)
- [x] Implementation (10/11 tasks complete, T010 manual test deferred)

## Key Context

**Problem**: After detaching and reattaching to a Zellij session, the sidebar shows "No Claude sessions" even though panes are running.

**Root Cause**: Startup race condition. The plugin restores cached sessions on load (`sync::restore_sessions()` in `main.rs:249`), but then `PaneUpdate` fires with an incomplete manifest and `remove_dead_sessions()` (`state.rs:146`) wipes all restored entries before panes finish initializing.

**Existing Infrastructure** (already implemented, do not rebuild):
- Session cache persistence: `/cache/sessions.json` (save on every change, restore on load)
- Multi-instance sync: `cc-deck:request` / `cc-deck:sync` pipe protocol
- Dead session cleanup: `remove_dead_sessions()` via `PaneUpdate` manifest
- Stale Done/AgentDone cleanup: `cleanup_stale_sessions()` via timer

**Solution Approach**: Add a startup grace period after plugin load during which `remove_dead_sessions` is deferred. Once the pane manifest stabilizes, reconcile cached sessions against it.

## Key Files
- `cc-zellij-plugin/src/main.rs:717-720` - PaneUpdate handler calls remove_dead_sessions
- `cc-zellij-plugin/src/state.rs:146-179` - remove_dead_sessions implementation
- `cc-zellij-plugin/src/sync.rs` - Session cache persistence + multi-instance sync
- `cc-zellij-plugin/src/main.rs:249-253` - Session restore on permission grant
- `brainstorm/027-sidebar-state-refresh.md` - Original brainstorm

## Next Step
Run `/speckit.clarify` or `/speckit.plan` to proceed.

## SDD State
sdd-initialized: true

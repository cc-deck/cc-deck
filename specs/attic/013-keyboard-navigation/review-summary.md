# Review Summary: 013-keyboard-navigation

## Overview

- **Feature**: Keyboard Navigation & Global Shortcuts for cc-deck sidebar
- **Spec**: 6 user stories, 20 functional requirements, 6 success criteria
- **Plan**: 8 phases, 38 tasks, 27 dependencies
- **Coverage**: 20/20 FRs mapped to tasks (100%)

## Spec-to-Task Coverage

| Requirement | Task(s) | Status |
|-------------|---------|--------|
| FR-001 Two modes (passive/navigation) | T002, T008, T010 | Covered |
| FR-002 Navigate shortcut (Alt+s) | T003, T006, T007, T008 | Covered |
| FR-003 Triangle cursor (▶) | T010 | Covered |
| FR-004 j/k/arrow navigation | T011 | Covered |
| FR-005 Enter switches session | T012 | Covered |
| FR-006 Esc exits navigation | T013 | Covered |
| FR-007 r starts rename | T022 | Covered |
| FR-008 d shows delete confirmation | T024, T025 | Covered |
| FR-009 y confirms delete | T026, T027 | Covered |
| FR-010 / enters search mode | T028 | Covered |
| FR-011 Case-insensitive search | T030 | Covered |
| FR-012 n creates new session | T034 | Covered |
| FR-013 g/Home and G/End | T014 | Covered |
| FR-014 Attend shortcut (Alt+a) | T003, T006 | Covered |
| FR-015 Smart attend priority tiers | T001, T004, T017 | Covered |
| FR-016 Skip current session | T018 | Covered |
| FR-017 Dynamic shortcut registration | T006 | Covered |
| FR-018 No override user config | T006 (reconfigure with false) | Covered |
| FR-019 Active tab instance only | T009 | Covered |
| FR-020 Cursor persistence | T016 | Covered |

## Research Quality

4 parallel research agents investigated:
1. `rebind_keys()` vs `reconfigure()` API (decided: `reconfigure()` with KDL)
2. Hook event distinction (PermissionRequest vs Notification distinguishable)
3. Focus/selectability independence (need both `set_selectable` + `focus_plugin_pane`)
4. Navigation patterns from harpoon/room/zbuffers (cursor wrapping with modulo)

All unknowns resolved. No remaining NEEDS CLARIFICATION markers.

## Task Quality Assessment

| Criterion | Assessment |
|-----------|------------|
| All tasks have IDs | Yes (T001-T038) |
| All story tasks have [US] labels | Yes (29 story tasks) |
| All tasks have file paths | Yes |
| Setup tasks are non-story | Yes (T001-T005) |
| Polish tasks are non-story | Yes (T035-T038) |
| Parallel tasks marked [P] | Yes (T001-T003, T020) |
| Each story independently testable | Yes (checkpoints defined) |
| MVP scope clear | Yes (Setup + US2 + US1) |

## Risks and Notes

1. **`reconfigure()` with `MessagePlugin`**: This is the recommended approach from research, but needs live testing. If `MessagePlugin` routing doesn't work as expected in 0.43.1, fallback to `rebind_keys()` with `Action::KeybindPipe`.

2. **`focus_plugin_pane()` availability**: Confirmed available in zellij-tile 0.43.1. Requires the plugin's own pane ID, which can be obtained via `get_plugin_ids()`.

3. **Activity::Waiting(WaitReason) breaking change**: T001 changes the enum, T005 fixes all callers, T038 updates tests. These must be done together in Phase 1.

4. **Notification hook mapping change**: Currently `Notification` does NOT set `Activity::Waiting`. The spec requires it to. T004 changes this, which is a behavioral change for existing users.

## Reviewer Guidance

When reviewing the spec PR:

1. **Check FR-015** (smart attend priority): The tiered algorithm is the most complex new logic. Verify the priority order makes sense for your workflow.
2. **Check default keybindings**: `Alt+s` and `Alt+a`. Verify these don't conflict with your terminal programs.
3. **Check Notification → Waiting change**: Previously, Notification events only updated timestamps. Now they'll set `Activity::Waiting(Notification)`. This changes sidebar behavior.
4. **Check search scope** (US5 is P3): Search/filter may not be needed in MVP. Consider deferring if timeline is tight.

# Tasks: Multiplayer Plugin Resilience

**Input**: Design documents from `specs/082-multiplayer-plugin-resilience/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Protocol & State Changes (Foundational)

**Purpose**: Add client_id to protocol messages and controller state. MUST complete before any user story work.

- [x] T001 Add `client_id: u16` field with `#[serde(default)]` to `SidebarHello` struct in `cc-zellij-plugin/src/lib.rs`. Add unit test verifying deserialization with and without `client_id` field (backward compatibility SC-006)
- [x] T002 [P] Add `client_id: u16` field to `ControllerState` in `cc-zellij-plugin/src/controller/state.rs`. Initialize to 0 in Default impl
- [x] T003 Store controller's own `client_id` from `get_plugin_ids().client_id` during permission grant in `cc-zellij-plugin/src/controller/mod.rs` (set `self.state.client_id` alongside existing `self.state.plugin_id` assignment)
- [x] T004 [P] Include `client_id` from `get_plugin_ids().client_id` in `SidebarHello` payload sent by the sidebar in `cc-zellij-plugin/src/sidebar_plugin/mod.rs` (update the hello construction)

**Checkpoint**: Protocol and state carry client_id. No behavioral changes yet.

---

## Phase 2: User Story 2 - Sidebar Registration Deduplication (Priority: P1)

**Goal**: Deduplicate sidebar registrations by (tab_index, client_id) to prevent unbounded growth.

**Independent Test**: Attach a second client, verify registry has one entry per (tab, client_id). No duplicate (tab, client_id) pairs.

- [x] T005 Change `sidebar_registry` type from `HashMap<u32, usize>` to `HashMap<u32, (usize, u16)>` in `cc-zellij-plugin/src/controller/state.rs`. Update all compile errors from the type change in: `sidebar_registry.rs` (19 refs), `events.rs` (6 refs), `render_broadcast.rs` (4 refs), `mod.rs` (3 refs), `integration_tests.rs` (2 refs). Value access changes from `&usize` to `&(usize, u16)`
- [x] T006 [US2] Update `handle_sidebar_hello` in `cc-zellij-plugin/src/controller/sidebar_registry.rs`: extract `client_id` from `SidebarHello`, insert `(tab_index, client_id)` into registry. Before inserting, check for an existing entry with the same `(tab_index, client_id)` and remove it (dedup per contract B2)
- [x] T007 [US2] Update `discover_sidebars_from_manifest` in `cc-zellij-plugin/src/controller/sidebar_registry.rs`: assign controller's `state.client_id` to auto-discovered sidebar entries (insert `(tab_pos, state.client_id)` instead of `tab_pos`)
- [x] T008 [US2] Update `cleanup_dead_sidebars` in `cc-zellij-plugin/src/controller/sidebar_registry.rs`: adjust the retain closure to handle the new `(usize, u16)` value type (logic unchanged, just destructure the tuple)
- [x] T009 [US2] Add unit tests in `cc-zellij-plugin/src/controller/sidebar_registry.rs`: test dedup replaces old entry with same (tab, client_id), test different client_ids on same tab coexist, test backward-compatible hello (client_id=0)

**Checkpoint**: Sidebar registry deduplicates by (tab, client_id). Existing tests pass.

---

## Phase 3: User Story 1 - Stable Sidebar During Multiplayer (Priority: P1)

**Goal**: Filter render broadcasts to only target the controller's own client sidebars. Zombie sidebars stop receiving updates.

**Independent Test**: Attach a second client. Verify debug log shows broadcast count equals tab count (not tab count * client count). No flickering.

- [x] T010 [US1] Update `broadcast_render` in `cc-zellij-plugin/src/controller/render_broadcast.rs`: filter `sidebar_registry` iteration to only include entries where `entry.1 == state.client_id` (the client_id component of the tuple). Log the filtered count
- [x] T011 [US1] Guard `broadcast_render_all()` untargeted fallback in `cc-zellij-plugin/src/controller/render_broadcast.rs`: only call it when `sidebar_registry` is empty (contract B7). Skip when registry has entries
- [x] T012 [US1] Add unit test in `cc-zellij-plugin/src/controller/render_broadcast.rs`: verify `broadcast_render` only iterates entries matching the controller's client_id. Verify broadcast_render_all is skipped when registry is non-empty

**Checkpoint**: Render broadcasts are filtered. Multiplayer sessions don't cause render storms.

---

## Phase 4: User Story 3 - Controller Election Stability (Priority: P2)

**Goal**: Use (client_id, plugin_id) tuple for election priority so primary client always wins.

**Independent Test**: Attach a second client. Verify original leader retains leadership. Detach. Leader unchanged.

- [x] T013 [US3] Update `broadcast_controller_ping` in `cc-zellij-plugin/src/controller/events.rs`: change function signature from `(plugin_id: u32)` to `(client_id: u16, plugin_id: u32)`. Change payload from `plugin_id.to_string()` to `format!("{}:{}", client_id, plugin_id)`. Update the non-wasm stub to match. Update all 5 call sites: 2 in `events.rs` (lines ~198, ~213) and 3 in `mod.rs` (lines ~121, ~464, ~691) to pass `state.client_id` as the first argument
- [x] T014 [US3] Update `PipeAction::ControllerPing` handler in `cc-zellij-plugin/src/controller/mod.rs`: parse `client_id:plugin_id` from payload. Compare as `(sender_client_id, sender_plugin_id) < (self.state.client_id, self.state.plugin_id)`. Fall back to plugin_id-only comparison if no `:` found (backward compatibility)
- [x] T015 [US3] Add unit tests for election comparison: test (1,0) beats (2,0), test (1,5) beats (2,0), test fallback parsing of old-format ping payload without client_id

**Checkpoint**: Election uses (client_id, plugin_id). Primary client always wins.

---

## Phase 5: User Story 4 - Zero Regression (Priority: P1)

**Goal**: Verify all existing functionality works identically in single-client mode.

- [x] T016 [US4] Run `make test` and verify all existing tests pass without modification (SC-001)
- [x] T017 [US4] Run `make lint` and verify no new warnings from the changes

**Checkpoint**: Zero regressions confirmed.

---

## Phase 6: Polish & Documentation

**Purpose**: Documentation and final verification.

- [x] T018 Update README.md with a note about multiplayer session support: behavior is stable but zombie plugin instances persist until session kill (Zellij #4064). Use the prose plugin with the cc-deck voice profile
- [x] T019 Run `make install` and manually test with a second attached terminal: verify sidebar stability, no flickering, correct session count in debug log

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Protocol & State)**: No dependencies, start immediately
- **Phase 2 (Registration Dedup)**: Depends on T001, T002 (protocol and state must carry client_id)
- **Phase 3 (Broadcast Filter)**: Depends on T005 (registry type change)
- **Phase 4 (Election)**: Depends on T002, T003 (controller must know its client_id)
- **Phase 5 (Regression)**: Depends on all code phases (1-4)
- **Phase 6 (Polish)**: Depends on Phase 5

### Parallel Opportunities

- T002 and T004 can run in parallel (different files)
- T010 and T013 can run in parallel after Phase 2 (different files, independent features)
- T018 is independent and can run at any time

---

## Implementation Strategy

### Sequential Delivery

1. Phase 1: Protocol + State (T001-T004)
2. Phase 2: Registry dedup (T005-T009)
3. Phase 3: Broadcast filter (T010-T012)
4. Phase 4: Election hardening (T013-T015)
5. Phase 5: Regression check (T016-T017)
6. Phase 6: Documentation (T018-T019)

---

## Notes

- Total tasks: 19
- Tasks per story: US1=3, US2=5, US3=3, US4=2, setup=4, polish=2
- All changes are in `cc-zellij-plugin/src/` (Rust only, no Go changes)
- Build via `make install`/`make test`/`make lint` only

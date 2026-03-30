# Feature Specification: Session TUI (Control Plane Dashboard)

**Feature Branch**: `031-session-tui`
**Created**: 2026-03-30
**Status**: Draft
**Input**: Brainstorm `brainstorm/031-session-tui.md`

## Clarifications

### Session 2026-03-30

- Q: Are P1/P2/P3 priorities release milestones or implementation ordering? → A: Phased releases. P1 is the MVP release, P2 and P3 are follow-on features in separate branches.
- Q: How does the TUI acquire session data for local environments (no HTTP endpoint)? → A: Read plugin state from the host filesystem. The Zellij plugin writes session state to the WASI cache directory, which maps to a known path on the host.
- Q: Are long-running operations (create container, data transfers) blocking or non-blocking? → A: Non-blocking. Operations run in the background with a progress indicator in the status bar. The user can continue navigating.
- Q: Where do status report API credentials come from? → A: Reuse existing cc-deck auth profile. The profile system already stores API keys for Claude.
- Q: Does P1 use direct polling or the daemon architecture? → A: Direct polling. Each TUI instance polls independently. The daemon architecture is Phase 2.
- Q: What is the Zellij session naming convention for local environments? → A: Use the environment name as the Zellij session name (1:1 mapping).
- Q: Which environment types are available in the P1 create wizard? → A: Local and container only. Compose and Kubernetes types are added in Phase 2.
- Q: Is cost display part of P1 or deferred? → A: Deferred to Phase 2 (detail view). P1 list omits the cost column.
- Q: What host path maps to the WASI /cache/ for reading local session data? → A: ~/.config/zellij/plugins/cc_deck.wasm/cache/sessions.json (standard Zellij WASI cache mapping).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View All Environments at a Glance (Priority: P1)

A developer managing multiple cc-deck environments across local, container, and Kubernetes backends opens a single full-screen terminal interface to see what is running, what needs attention, and what is idle. Today this requires running `cc-deck env list`, `cc-deck env status <name>` for each environment, and mentally tracking which agents need input. The TUI replaces this with a live, auto-refreshing table showing every environment, its type, status, session health, and last-attached time.

**Why this priority**: Without visibility, the user cannot make informed decisions about which environment to interact with. This is the foundation for every other TUI capability.

**Independent Test**: Launch the TUI with at least two environments (one local, one container). Verify the list displays environment name, type, status, session count, and last-attached timestamp. Verify the list auto-refreshes when an environment's status changes.

**Acceptance Scenarios**:

1. **Given** two running environments (one local, one container), **When** the user launches the TUI, **Then** both environments appear in the list with correct names, types, and "running" status.
2. **Given** an environment transitions from running to stopped externally, **When** the TUI polls status, **Then** the list updates to show the new status within the configured polling interval.
3. **Given** an environment has sessions needing attention (e.g., permission requests), **When** the user views the list, **Then** a visual indicator shows how many sessions need attention.
4. **Given** no environments exist, **When** the user launches the TUI, **Then** an empty state message appears with guidance on creating the first environment.

---

### User Story 2 - Attach to an Environment (Priority: P1)

The user selects an environment from the list and presses Enter. The TUI suspends itself, hands the terminal over to Zellij (or the appropriate attach mechanism for the environment type), and the user works inside Zellij with the cc-deck sidebar plugin. When the user exits Zellij, the TUI resumes and refreshes its view. The user never needs to remember environment names or type attach commands.

**Why this priority**: Attach is the primary action users take after viewing the list. Without it, the TUI is read-only and delivers minimal value.

**Independent Test**: From the TUI list, select a running local environment, press Enter. Verify Zellij opens. Exit Zellij. Verify the TUI reappears with an updated list.

**Acceptance Scenarios**:

1. **Given** a running local environment is selected, **When** the user presses Enter, **Then** the TUI suspends, Zellij attaches, and the user sees the Zellij session with the cc-deck sidebar.
2. **Given** the user exits Zellij after attaching, **When** Zellij terminates, **Then** the TUI resumes and displays the refreshed environment list.
3. **Given** a running container environment is selected, **When** the user presses Enter, **Then** the TUI suspends and attaches via the container runtime.
4. **Given** a stopped environment is selected, **When** the user presses Enter, **Then** the TUI displays a message that the environment must be started first (or offers to start it).
5. **Given** the user attaches from one TUI instance, **When** another TUI instance is open, **Then** the other instance shows the environment as "attached." *(Phase 2: requires daemon for shared state; P1 instances are independent.)*

---

### User Story 3 - Create a New Environment from the TUI (Priority: P1)

The user presses a key to open a creation wizard. The wizard presents fields appropriate to the selected environment type (local, container, compose, Kubernetes). After filling in the fields and confirming, the environment is created and optionally auto-attached. The user never needs to switch to the CLI to create environments.

**Why this priority**: The TUI should be the single entry point for all environment operations. Forcing users back to the CLI for creation breaks the workflow.

**Independent Test**: From the TUI, create a local environment by providing just a name. Verify it appears in the list as "running." Verify the user can immediately attach to it.

**Acceptance Scenarios**:

1. **Given** the user presses the "new" key, **When** the create wizard opens, **Then** environment type options are shown (local and container in P1; compose and Kubernetes added in Phase 2).
2. **Given** "local" is selected as type, **When** the user enters a name and confirms, **Then** a local Zellij session is created and registered.
3. **Given** "container" is selected, **When** the user fills in image, storage, and source path and confirms, **Then** a container environment is created using the same logic as `cc-deck env create`.
4. **Given** creation succeeds, **When** auto-attach is enabled (default), **Then** the TUI suspends and attaches to the newly created environment.
5. **Given** environment creation fails (e.g., invalid name, container pull error), **When** the error occurs, **Then** the wizard displays the error and allows the user to correct the input.

---

### User Story 4 - Manage Environment Lifecycle (Priority: P1)

The user can start stopped environments, stop running environments, and delete environments directly from the TUI. Destructive operations (delete) require confirmation by typing the environment name. The user manages the full lifecycle without leaving the TUI.

**Why this priority**: Core lifecycle operations complete the TUI as a control plane. Without them, users still depend on the CLI for basic management.

**Independent Test**: Stop a running environment from the TUI. Verify its status changes to "stopped." Start it again. Verify it returns to "running." Delete a stopped environment, confirm with name entry. Verify it disappears from the list.

**Acceptance Scenarios**:

1. **Given** a running environment is selected, **When** the user presses the stop key, **Then** the environment stops and the status updates to "stopped."
2. **Given** a stopped environment is selected, **When** the user presses the start key, **Then** the environment starts and the status updates to "running."
3. **Given** the user presses the delete key on an environment, **When** a confirmation dialog appears, **Then** the user must type the environment name to confirm deletion.
4. **Given** the user confirms deletion, **When** the environment is removed, **Then** it disappears from the list and associated resources are cleaned up.
5. **Given** the user presses Esc during a confirmation dialog, **When** the dialog closes, **Then** no action is taken.

---

### User Story 5 - Environment Detail View with Session Status (Priority: P2)

The user navigates to a detail view for a specific environment to see all agent sessions, their current activity (working, idle, permission-needed, done), branches, and cost estimates. This provides a drill-down from the high-level list into what each agent is doing within an environment.

**Why this priority**: The list view gives a summary, but effective management requires knowing what each session is doing. This is the bridge between "I see my environments" and "I understand what my agents are doing."

**Independent Test**: Open the detail view for an environment with multiple sessions. Verify each session shows its name, activity status, branch, and last event time. Verify the view auto-refreshes.

**Acceptance Scenarios**:

1. **Given** the user presses the detail key on a running environment, **When** the detail view opens, **Then** all sessions within that environment are listed with name, status, branch, last event, and cost.
2. **Given** a session transitions to "permission needed," **When** the detail view refreshes, **Then** the session row shows a visual attention indicator.
3. **Given** the user presses Enter on a session in the detail view, **When** the TUI attaches, **Then** the Zellij session opens focused on the appropriate tab for that session.
4. **Given** the user presses Esc in the detail view, **When** the view closes, **Then** the user returns to the environment list.

---

### User Story 6 - Search and Filter Environments (Priority: P2)

The user presses a key to enter search mode and types a query. The environment list filters in real time, matching against environment names, tags, session names, and branches. The user can also filter by environment type using number keys (1 = all, 2 = local, 3 = container, 4 = compose, 5 = Kubernetes). This is essential when the user has many environments.

**Why this priority**: With growing numbers of environments, scanning a list becomes slow. Search and filter provide instant access to the right environment.

**Independent Test**: With 5+ environments, type a search query matching one environment's tag. Verify only matching environments appear. Press the type filter key. Verify only environments of that type appear.

**Acceptance Scenarios**:

1. **Given** the user enters search mode, **When** they type a query, **Then** the list filters in real time showing only matching environments.
2. **Given** a search query matches an environment's tag but not its name, **When** the filter is applied, **Then** the environment still appears in the results.
3. **Given** the user presses a type filter key, **When** the filter is active, **Then** only environments of that type are shown.
4. **Given** the user presses Esc in search mode, **When** the search clears, **Then** the full list is restored.

---

### User Story 7 - Data Transfer Operations (Priority: P2)

The user can push files to, pull files from, and harvest git changes from an environment, all from the TUI. These operations use the same logic as `cc-deck env push`, `cc-deck env pull`, and `cc-deck env harvest`. The user sees progress indicators and results without switching to a separate terminal.

**Why this priority**: Data transfer is a frequent operation during development. Embedding it in the TUI eliminates context switching.

**Independent Test**: From the TUI, trigger a harvest operation on a container environment. Verify progress is shown. Verify the operation completes successfully. Check that the harvested changes appear on the host.

**Acceptance Scenarios**:

1. **Given** a running environment is selected, **When** the user presses the harvest key, **Then** a harvest operation begins with a progress indicator.
2. **Given** a push operation completes successfully, **When** the result is returned, **Then** the TUI displays a success message with summary details.
3. **Given** a data transfer fails (e.g., network error), **When** the error occurs, **Then** the TUI displays the error message and the environment remains in its previous state.

---

### User Story 8 - OS-Level Notifications (Priority: P3)

When an agent session transitions to a state requiring human attention (e.g., permission request) or an environment stops unexpectedly, the operating system displays a notification. This ensures the user is alerted even when the TUI is not in the foreground or not running. Notifications are deduplicated so the same state transition does not produce multiple alerts.

**Why this priority**: Notifications are valuable but not essential for core functionality. Users can work effectively by periodically checking the TUI.

**Independent Test**: Configure notifications to be enabled. Trigger a permission request in a session. Verify an OS notification appears. Verify a second poll cycle does not produce a duplicate notification for the same event.

**Acceptance Scenarios**:

1. **Given** notifications are enabled, **When** a session transitions to "permission needed," **Then** an OS notification appears with the environment name and session details.
2. **Given** a notification was already sent for a specific state transition, **When** the next poll detects the same state, **Then** no duplicate notification is sent.
3. **Given** the user disables notifications in configuration, **When** a state transition occurs, **Then** no OS notification is sent.
4. **Given** an environment stops due to an error, **When** the error is detected, **Then** an OS notification alerts the user.

---

### User Story 9 - On-Demand Status Report (Priority: P3)

The user presses a key to generate a human-readable status report for the selected environment (or all environments from the list view). The report is a flowing prose summary of all sessions: what needs attention, what is in progress, what is completed, and cost breakdown. The report can be copied to the clipboard for use in standups, Slack messages, or PR descriptions.

**Why this priority**: This is a convenience feature that adds significant value but is not required for core environment management. It depends on session detail data and external API access.

**Independent Test**: Open the detail view for an environment with multiple sessions. Press the report key. Verify a prose summary appears covering all sessions. Copy it to the clipboard and verify the content is well-formatted.

**Acceptance Scenarios**:

1. **Given** the user presses the report key in the detail view, **When** session data is available, **Then** a flowing prose status report is generated and displayed in a scrollable view.
2. **Given** the report is displayed, **When** the user presses the copy key, **Then** the report text is copied to the system clipboard.
3. **Given** the user presses the report key in the list view with no environment selected, **When** multiple environments exist, **Then** a cross-environment status report is generated.
4. **Given** the report generation encounters an error (e.g., API unavailable), **When** the error occurs, **Then** the TUI displays an error message and falls back to a structured (non-AI) summary of session states.

---

### User Story 10 - Tags and Rename (Priority: P2)

The user can add, edit, or remove tags on any environment and rename environments. Tags enable filtering and organization. Renamed environments update everywhere (state files, display, references).

**Why this priority**: Organization features become important as the number of environments grows, but individual environments work fine without them.

**Independent Test**: Rename an environment from the TUI. Verify the new name appears in the list. Add a tag. Verify it appears in the tags column. Search for the tag. Verify the environment appears in results.

**Acceptance Scenarios**:

1. **Given** the user presses the rename key, **When** a text input appears with the current name, **Then** the user can edit the name and confirm.
2. **Given** the user presses the tag key, **When** a text input appears, **Then** the user can add or remove comma-separated tags.
3. **Given** a renamed environment, **When** the list refreshes, **Then** the new name is displayed consistently.

---

### Edge Cases

- What happens when the terminal window is too small to display the full list? The TUI should show a minimum viable layout or an informative message.
- How does the TUI handle rapid environment state changes during attach/detach cycles? State should be refreshed on resume.
- What happens if multiple TUI instances attempt the same destructive operation simultaneously? The first operation should succeed, subsequent ones should see an updated state.
- What happens when a container image pull times out during creation? The wizard should display the error and allow retry.
- How does the TUI behave when the user's terminal does not support colors? It should degrade gracefully to a no-color mode.
- What happens when an environment's status endpoint is unreachable? The session column should show "unknown" with an appropriate indicator.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST display a live, auto-refreshing list of all registered cc-deck environments with name, type, status, session health summary (local environments only in P1; container/K8s session health requires Phase 2 HTTP status endpoint), storage type, last-attached time, and tags.
- **FR-002**: System MUST allow the user to attach to a running environment by selecting it and pressing a key, suspending the TUI, handing the terminal to the environment's attach mechanism, and resuming the TUI when the user exits.
- **FR-003**: System MUST provide an environment creation wizard that adapts its fields based on the selected environment type. P1 supports local and container types only; compose and Kubernetes types are added in Phase 2.
- **FR-004**: System MUST support starting, stopping, and deleting environments from the TUI.
- **FR-005**: System MUST require explicit confirmation (typing the environment name) before deleting an environment.
- **FR-006**: System MUST provide a detail view for each environment showing individual session statuses, branches, last event times, and cost estimates (Phase 2). For local environments, session data MUST be read from the Zellij plugin's WASI cache directory on the host filesystem at `~/.config/zellij/plugins/cc_deck.wasm/cache/sessions.json`.
- **FR-007**: System MUST support full-text search across environment names, tags, session names, and branch names.
- **FR-008**: System MUST support filtering the environment list by environment type.
- **FR-009**: System MUST support push, pull, and harvest data transfer operations from the TUI, running non-blocking in the background with a progress indicator. The user MUST be able to continue navigating while operations run.
- **FR-010**: System MUST support renaming environments and editing tags.
- **FR-011**: System MUST fire OS-level notifications when a session transitions to a state requiring attention, with deduplication.
- **FR-012**: System MUST allow disabling notifications via configuration.
- **FR-013**: System MUST provide an on-demand status report generator that produces a flowing prose summary of all sessions in an environment (or across all environments). The report generator MUST use API credentials from the existing cc-deck auth profile.
- **FR-014**: System MUST allow copying the status report to the system clipboard.
- **FR-015**: System MUST be accessible as a subcommand of the existing cc-deck binary, not as a separate binary.
- **FR-016**: System MUST show context-sensitive key hints at the bottom of each view, changing based on the selected item and current view.
- **FR-017**: System MUST handle terminal resize events gracefully, reflowing the layout to fit the new dimensions.
- **FR-018**: System MUST reuse the existing environment management logic (create, start, stop, delete, attach, push, pull, harvest) from the Go codebase. No duplication of business logic.
- **FR-019**: System MUST show an aggregate header with environment counts by state.
- **FR-020**: System MUST support keyboard navigation using both arrow keys and vim-style keys (j/k for up/down, g/G for top/bottom).
- **FR-021**: System MUST display a help overlay listing all available key bindings, organized by category.

### Key Entities

- **Environment**: A cc-deck environment (local, container, compose, or Kubernetes) with a name, type, status, configuration, tags, and zero or more agent sessions.
- **Agent Session**: An individual Claude Code session running inside an environment, identified by name, with an activity state (working, idle, permission-needed, done, error), branch, cost data, and tool-use information.
- **Status Report**: A generated prose summary of session activity within one or more environments, including action items, progress, and cost breakdown.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can view the status of all environments within 2 seconds of launching the TUI.
- **SC-002**: Users can attach to any running environment in 2 key presses or fewer (navigate + Enter).
- **SC-003**: Users can create a new local environment and begin working in it within 30 seconds using only the TUI.
- **SC-004**: The environment list refreshes automatically, reflecting external state changes within the configured polling interval.
- **SC-005**: Users managing 10+ environments can find a specific environment via search in under 5 seconds.
- **SC-006**: OS notifications reach the user within 10 seconds of an agent requesting attention.
- **SC-007**: Status reports provide an actionable summary that a user can paste directly into a standup update or Slack message.
- **SC-008**: All existing CLI environment operations (create, start, stop, delete, attach, push, pull, harvest) are accessible from the TUI with no loss of functionality.
- **SC-009**: The TUI gracefully handles terminal widths from 80 columns to full-screen displays.

## Assumptions

- The existing `internal/env` interface and its implementations (local, container, compose) provide all necessary operations. The TUI calls these directly.
- P1 uses direct polling from each TUI instance. The daemon architecture (shared polling, Unix socket communication) is Phase 2.
- Local environments use the environment name as the Zellij session name (1:1 mapping), simplifying attach/detach.
- The Zellij sidebar plugin already writes session state data that can be read by the TUI or a status endpoint.
- The user's terminal emulator supports 256-color or true-color output. Graceful degradation to basic colors is acceptable but not a primary target.
- Kubernetes environment support in the TUI depends on the K8s backend implementations in the existing codebase. If those are incomplete, the TUI will show K8s as an option but operations may be limited.
- The Claude API is available for status report generation. When unavailable, the system falls back to a structured (non-AI) summary.
- Polling intervals are configurable but ship with sensible defaults (2s local, 5s container, 10s Kubernetes).

## Dependencies

- **023-env-interface**: The Environment interface and its behavioral contracts.
- **024-tui-environment-manager**: Superseded by this feature.
- **025-container-environment**: Container and compose environment implementations.
- **030-single-instance-architecture**: The single-instance controller pattern for daemon coordination.

## Scope Boundaries

### Release Phasing

This feature ships in phased releases aligned to priority tiers:

- **MVP (P1)**: Environment list view with direct polling (no daemon), attach (suspend/resume), create wizard (local + container only), lifecycle management (start/stop/delete), help overlay. Cost display is deferred. This is a complete, shippable product.
- **Phase 2 (P2)**: Detail view with session status, search and filter, data transfer operations (push/pull/harvest), tags and rename.
- **Phase 3 (P3)**: OS-level notifications with deduplication, on-demand AI-generated status reports.

Each phase ships in its own branch/release. P2 and P3 depend on the MVP being merged first.

### In Scope

- Full-screen terminal UI with list view, detail view, create wizard, search, help overlay, status report, and confirmation dialogs.
- All environment lifecycle operations (create, start, stop, delete, rename, tag).
- Attach via suspend/resume model.
- Data transfer operations (push, pull, harvest).
- OS-level notifications with deduplication.
- On-demand AI-generated status reports (with non-AI fallback).

### Out of Scope

- Tmux support (Zellij only for v1).
- User-configurable key bindings.
- User-defined color themes.
- Per-session drill-down panels (replaced by status report).
- Authenticated status endpoints (assumes private network for K8s Routes).
- Daemon state persistence to disk (daemon starts fresh on each launch).
- Client-daemon architecture (Phase 2; P1 uses direct polling).
- Cost column in environment list (Phase 2; requires session detail data).
- Compose and Kubernetes types in create wizard (Phase 2).

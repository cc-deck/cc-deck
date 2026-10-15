# Feature Specification: Codex Shared Daemon Pane Attribution

**Feature Branch**: `089-codex-daemon-pane-attribution`

**Created**: 2026-10-15

**Status**: Draft

**Input**: User description: "Attribute Codex hook events to the correct Zellij pane when Codex runs on its shared app-server daemon. Source brainstorm: brainstorm/097-codex-daemon-pane-attribution.md (decision: hook-side discovery of the originating Codex terminal client with a session cache, plus the pane-map write race fix)."

## Background

Since Codex 0.157, an interactive `codex` terminal client (the TUI) hands its work to a shared background app-server daemon, one per Codex home directory, and Codex lifecycle hooks run inside that daemon.
The daemon inherits the environment of whichever client started it first, so every hook event from every later Codex client carries that first client's Zellij pane and session identity.
cc-deck's Codex hooks forward that inherited pane to the sidebar, so a Codex session either never appears (when the inherited pane no longer exists) or appears in another pane's row.
Codex maintainers consider hooks client-agnostic by design, and a proposal to pass selected client environment to hooks (openai/codex#44902) is open but not shipped.

## Clarifications

### Session 2026-10-15

- Q: How does cc-deck decide that a Codex hook runs inside the shared daemon? → A: The hook counts as daemon-hosted when one of its ancestor processes is a Codex app-server process; otherwise it trusts its own pane identity.
- Q: Where is an event delivered when its originating client runs in a different Zellij session than the one the daemon's identity names? → A: To the originating client's own Zellij session and pane; events from clients outside Zellij are dropped.
- Q: What happens when several clients share a working directory and no later event disambiguates them? → A: New sessions stay hidden, every dropped event is logged as ambiguous, and attribution resumes as soon as only one matching client remains; the documentation recommends distinct directories or `--no-daemon`.
- Q: What identifies the client behind a remembered mapping, and when is a mapping live? → A: The client's process id together with its process start time; a mapping is live only while that exact process runs, which guards against process id reuse.

### Spec review 2026-10-15

- Decision: One Codex client can host several sessions over its lifetime (`/new`, resuming a session inside the TUI, subagent threads). When exactly one client matches a new session, the session is attributed to that client even if the client already owns other sessions. When several clients match, the event is dropped, whether or not some of them are already bound, because the hook cannot tell "a bound client started another session" apart from "a second client started its first session". This keeps the rule "never a wrong row" without exceptions; the cost is that new sessions in a directory shared by several clients stay hidden until only one client remains there.
- Decision: Candidate clients are limited to those using the same Codex home as the daemon that runs the hook, because each Codex home has its own daemon and cc-deck profiles use separate Codex homes.
- Decision: A daemon-hosted event is attributed even when the daemon's inherited environment carries no Zellij identity at all (daemon first started from a plain terminal).
- Decision: The inherited pane identity of a daemon-hosted event is never written to the session-to-pane cache, and cache entries without a recorded client identity are never used for daemon-hosted events.

### Code review 2026-10-15

- Decision: "Codex app-server process" in FR-002 means the shared daemon: the managed app-server (`codex app-server --listen unix://... --managed-daemon`) and its helper processes (`codex app-server daemon ...`). An app-server that a tool runs for itself (the stdio app-server of an IDE extension) is not the shared daemon; its hooks keep today's attribution, and the process counts as a matching, non-bindable client.
- Decision: When a session's remembered client is no longer live, ordinary events of that session are dropped without discovery, because another client that now matches the directory alone does not own the session (its trailing events would land in a wrong row). Only a `SessionStart` event (startup, resume, clear, compact, fork) runs discovery for such a session. On `SessionStart`, a uniquely discovered client also replaces a live mapping, so a session resumed in another client follows that client; when discovery is not unique, the live mapping stays only if its client is among the matching clients, and is otherwise removed with the event dropped. Known limitations: a session that was never bound runs discovery on every event (FR-007), so its trailing events after its client exited can reach the one remaining client in that directory; and a session resumed in a second client in the same directory while its original client still runs keeps the original client's row, because that `SessionStart` cannot be told apart from a compaction in the original client (US2-2).
- Decision: The daemon check reads only the arguments of ancestor processes (on Linux `status`, `stat` and `cmdline`), so a non-dumpable ancestor is still classified. An unreadable `status` counts as another user's process only when the `/proc/<pid>` directory names an owner that is neither the user nor root; otherwise the event is dropped, because the kernel also denies `status` for a non-dumpable process of the same user under `hidepid=noaccess`. Client inspection still needs the environment; where Linux refuses it (non-dumpable client), daemon-hosted events are dropped.
- Decision: An ancestor process of another user ends the daemon check with "not daemon-hosted", because the user's daemon cannot run above it.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A hand-started Codex session shows up in its own row (Priority: P1)

A user opens a Zellij pane in a cc-deck session and types `codex`.
Codex connects to the shared daemon, which another pane started earlier and which still carries that other pane's identity.
The user's Codex session appears in the sidebar row of the pane where the user typed `codex`, and its status follows the session (working, waiting for input, done).

**Why this priority**: This is the reported defect. Without it, Codex sessions are invisible or misattributed whenever the shared daemon exists, which is the Codex default.

**Independent Test**: Start Codex in pane A, close pane A while the daemon keeps running, start Codex in pane B, submit a prompt that runs a tool, and observe that pane B's row shows the Codex session with changing status.

**Acceptance Scenarios**:

1. **Given** a running shared daemon started from a pane that no longer exists, **When** the user starts `codex` in pane B and submits a prompt, **Then** the sidebar shows a Codex session in pane B's row by the first hook event that can be attributed.
2. **Given** a running shared daemon started from pane A (still open, idle), **When** the user starts `codex` in pane B and works there, **Then** all of pane B's Codex activity appears in pane B's row and none of it in pane A's row.
3. **Given** a running shared daemon started from a terminal outside Zellij, **When** the user starts `codex` in pane B and submits a prompt, **Then** the sidebar shows the Codex session in pane B's row.
4. **Given** a Codex session already shown in pane B's row, **When** the user starts a new conversation in the same client (for example `/new`), **Then** the new session also appears in pane B's row and in no other row.
5. **Given** no shared daemon is running, **When** the user starts `codex` in a pane (so that Codex starts a daemon from this pane), **Then** the session appears in that pane's row, as before this feature.
6. **Given** Codex runs with `--no-daemon`, **When** the user works in it, **Then** the session appears in its own pane's row, as before this feature.

---

### User Story 2 - Several Codex sessions share one daemon (Priority: P1)

A user runs Codex in three panes at the same time, all attached to the same shared daemon.
Each pane's sidebar row reflects only the Codex session running in that pane.

**Why this priority**: Running several agents side by side is cc-deck's core use case; attribution must hold for more than one Codex client at a time.

**Independent Test**: Start Codex in three panes with three different working directories, drive each through a prompt, and confirm each row's status changes only with its own pane's activity.

**Acceptance Scenarios**:

1. **Given** three Codex clients in three panes with distinct working directories sharing one daemon, **When** each runs a prompt, **Then** each sidebar row shows exactly one Codex session, attributed to its own pane.
2. **Given** two Codex clients in two panes with the same working directory, one of which already owns an attributed session, **When** either client starts a new session, **Then** no sidebar row is created or changed for the new session, and the existing attributed session keeps updating its own row.
3. **Given** two Codex clients in two panes with the same working directory and neither yet attributed, **When** a hook event arrives that cannot be tied to exactly one of them, **Then** no sidebar row is created or changed for that event.

---

### User Story 3 - Codex clients outside this Zellij session stay out (Priority: P2)

A user runs Codex in a plain terminal outside Zellij, or in a different Zellij session, on the same machine and Codex home.
Those sessions share the daemon with the Codex clients in the cc-deck session.
They never appear in the cc-deck session's sidebar.

**Why this priority**: The daemon's inherited identity can point into the current Zellij session even for a client that runs elsewhere; without this rule, foreign sessions would show up as ghosts.

**Independent Test**: With a Codex client in the cc-deck session, start a second Codex client in a plain terminal in a different directory, drive it through a prompt, and confirm the sidebar does not change.

**Acceptance Scenarios**:

1. **Given** a Codex client in a terminal outside Zellij sharing the daemon, **When** it runs a prompt, **Then** no sidebar in any Zellij session shows its session.
2. **Given** a Codex client in Zellij session S2 sharing the daemon with clients in session S1, **When** it runs a prompt, **Then** only S2's sidebar may show it, in the client's own pane, and S1's sidebar does not change.

---

### User Story 4 - Diagnosing attribution (Priority: P3)

A user whose Codex session does not show up enables cc-deck's hook debug log and can see, per event, how the pane was determined or why the event was dropped.

**Why this priority**: The original defect was hard to diagnose because the log only showed the inherited pane; explicit resolution reasons make future problems visible.

**Independent Test**: Enable hook debugging, reproduce each attribution path (own pane identity trusted, cached mapping, newly discovered client, ambiguous, no client found), and check that each produces a distinct, readable log entry.

**Acceptance Scenarios**:

1. **Given** hook debugging is enabled, **When** a Codex event is attributed or dropped, **Then** the debug log records the event, the session, the resulting pane (or none), and the reason.

---

### Edge Cases

- The pane that started the daemon still exists and runs a non-Codex program: Codex events from other panes must not be attributed to it.
- The pane that started the daemon still exists and runs its own Codex client: its own events are attributed to it, other clients' events are not.
- A Codex client exits and a new one starts in the same pane: the new session is attributed to that pane; the old session's mapping is not reused for a different pane.
- A Codex client is started with a working directory argument (`-C <dir>` or `--cd <dir>`) that differs from the shell's directory: the argument, not the shell's directory, is matched.
- The event's working directory and the client's working directory name the same directory through different paths (symbolic links, for example `/tmp` and `/private/tmp` on macOS): they match.
- The shared daemon was first started from a terminal outside Zellij, so the hook process has no Zellij identity at all: events are still attributed to their originating client's pane.
- A Codex client that already owns an attributed session starts another session (`/new`, resuming another session inside the TUI, a subagent thread with its own session id): when it is the only matching client, the new session is attributed to it, and its earlier sessions keep their mappings.
- A bound client starts another session while another client in a different pane shares its working directory and Codex home: the new session is not attributed (the event is dropped and logged as ambiguous) until only one matching client remains; the bound client's earlier sessions keep their rows.
- A session is resumed from a directory where no running client matches its recorded working directory: the event is dropped as "no client found".
- Codex clients using different Codex homes (for example a cc-deck profile wrapper and a plain `codex`) run in the same working directory: each daemon only considers clients of its own Codex home.
- A non-interactive Codex run (`codex exec`) whose hooks run inside the shared daemon is never bound to a pane, but it counts as a matching client: while it runs in the same working directory and Codex home as an interactive client, new sessions there are dropped as ambiguous. When its hooks run in its own process, today's attribution applies (FR-003).
- The cache contains entries written before this feature, without a recorded client identity: they are not used for daemon-hosted Codex events.
- The Codex client process cannot be inspected (permission denied, process exited mid-lookup): the event is dropped, not misattributed.
- A Codex client exits and the operating system reuses its process id for another process: the old mapping is not live and is not used.
- Many hook events arrive at once from several sessions: the session-to-pane cache stays consistent and no reader sees a partially written cache.
- A cached mapping names a pane in a Zellij session other than the current one: the mapping is not used for the current session.
- A future Codex release passes the client's identity to hooks: that identity takes precedence over discovery.
- Claude Code and OpenCode hook events: behavior is unchanged.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: For Codex hook events, cc-deck MUST attribute each event to the Zellij pane of the Codex terminal client that owns the event's session, not to the pane identity the hook process inherited from the shared daemon.
- **FR-002**: cc-deck MUST NOT attribute a Codex event to a pane solely because the hook process's inherited pane identity names that pane, when the hook runs inside the shared daemon. A hook runs inside the shared daemon when one of its ancestor processes is a Codex app-server process. For such events cc-deck MUST NOT record the inherited pane identity in the session-to-pane cache, and MUST NOT use cache entries that lack a recorded client identity.
- **FR-003**: When a Codex hook event does not run inside the shared daemon (no Codex app-server ancestor, for example Codex started with `--no-daemon` or Codex releases without the daemon), cc-deck MUST keep today's attribution from the hook's own pane identity.
- **FR-004**: When an event's session is not yet attributed, cc-deck MUST identify the originating Codex client among the user's running interactive Codex terminal clients that use the same Codex home as the daemon running the hook, by matching the event's working directory to the client's working directory. Matching MUST take a client's explicit working directory argument into account and MUST compare directories after resolving symbolic links.
- **FR-005**: Every Codex process of the user in the same Codex home and working directory counts as a matching client, except the shared daemon's own processes; only interactive terminal clients can be bound. When exactly one client matches and it is interactive, it is the only candidate, even if it already owns other live sessions; a single non-interactive match yields no candidate. When several clients match, there is no candidate, whether or not some of them already own live sessions. A mapping is live while the client it names (identified by process id and process start time) is still running.
- **FR-006**: When exactly one candidate client remains, cc-deck MUST attribute the event to that client's pane and Zellij session and remember the session-to-pane mapping. A client MAY own several mappings; earlier mappings of the same client stay usable.
- **FR-007**: When zero or several clients match, cc-deck MUST drop the event for the sidebar (no row created or changed) and MUST try again on the session's next event. Ambiguity that persists keeps the affected sessions hidden; it ends when only one matching client remains.
- **FR-008**: A remembered session-to-pane mapping MUST be used for later events of the same session without repeating the client lookup, as long as the mapped client is still running in that pane.
- **FR-009**: A remembered mapping MUST NOT be used when the mapped client has exited, when the pane now runs a different client, or when the mapping belongs to a different Zellij session than the one receiving the event. Events of such a session are dropped until the session's next `SessionStart` event, which runs discovery again.
- **FR-010**: An attributed event MUST be delivered to the originating client's own Zellij session and pane, regardless of the Zellij session the daemon's inherited identity names. This applies also when the daemon's inherited environment carries no Zellij identity. Events whose originating client runs outside Zellij MUST be dropped.
- **FR-011**: When a Codex hook payload or environment provides the originating client's pane identity explicitly (for example a future client environment allowlist), cc-deck MUST prefer that identity over discovery.
- **FR-012**: The session-to-pane cache MUST be written atomically, and concurrent hook processes MUST NOT lose each other's entries or observe a partially written cache.
- **FR-013**: Claude Code and OpenCode hook handling MUST remain unchanged, apart from benefiting from the cache robustness in FR-012.
- **FR-014**: When hook debugging is enabled, cc-deck MUST log for each Codex event the event name, the session, the resolved pane (or none), and one resolution reason out of: own pane identity, explicit client identity, cached mapping, discovered client, ambiguous, no client found, client outside Zellij.
- **FR-015**: Users MUST NOT need to change how they start Codex, the Codex configuration, or the cc-deck hook installation for attribution to work.
- **FR-016**: The documentation MUST describe the Codex shared daemon behavior, the attribution rules (including that new sessions in a working directory shared by several Codex clients stay hidden while the directory is shared), and the alternatives for that case: distinct working directories or `--no-daemon`.
- **FR-017**: For daemon-hosted Codex events, profile enrichment (profile name and color in the sidebar) MUST use the originating client's profile identity, not the identity the daemon inherited.

### Key Entities

- **Codex terminal client**: An interactive `codex` process running in a terminal; has an owning user, a working directory (or explicit working directory argument), and, when inside Zellij, a pane and Zellij session identity.
- **Shared daemon**: The background Codex app-server that runs hooks for all attached clients; carries the environment of the client that started it.
- **Session-to-pane mapping**: A remembered association from a Codex session to the pane and Zellij session of the client that owns it, plus the client's process id and start time, recorded when an event is attributed.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With the shared daemon running from a closed pane, a newly started Codex session appears in its own pane's sidebar row in 100% of trials, by the first tool call of its first prompt.
- **SC-002**: With three Codex clients in distinct directories sharing one daemon, 100% of hook events are attributed to the correct pane and 0 events to another pane.
- **SC-003**: Across all tested scenarios, 0 events are attributed to a wrong pane; ambiguous cases result in dropped events, never misattribution.
- **SC-004**: After a session's first attributed event, later events of that session add no more than 20 ms to hook processing compared with today's handling.
- **SC-005**: The first attributed event of a session completes hook processing within 500 ms on a machine with up to 10 running Codex clients.
- **SC-006**: Under 20 concurrent hook processes, the session-to-pane cache remains readable and contains every attributed session afterwards.
- **SC-007**: Claude Code and OpenCode sidebar behavior is unchanged, verified by the existing hook test suite passing without modification.

## Assumptions

- Codex terminal clients and the daemon run as the same user as the hook, so the hook can inspect the clients' working directory and environment on macOS and Linux.
- The pane and Zellij session identity of a Codex client are visible in that client's own environment (`ZELLIJ_PANE_ID`, `ZELLIJ_SESSION_NAME`), as Zellij sets them per pane.
- The hook payload's `cwd` is the working directory of the originating client's thread, which equals the client's working directory or its explicit working directory argument.
- Codex hook payloads continue to carry `session_id` and `cwd`.
- Disabling or bypassing the shared daemon is out of scope; cc-deck does not change Codex's configuration.
- Remote workspaces (SSH, OpenShell) behave like the local case when Codex runs there with a shared daemon; no separate remote mechanism is required.

# Feature Specification: Claude Code Background Session Pane Attribution

**Feature Branch**: `090-claude-bg-pane-attribution`

**Created**: 2026-10-18

**Status**: Draft

**Input**: User description: "Attribute Claude Code background-session hook events to the Zellij pane that views the job. Source: the brainstorm discussion of 2026-10-17 (no brainstorm document was written; decisions are recorded under Clarifications), the diagnosis behind anthropics/claude-code#102949, and the Codex precedent in spec 089."

## Background

Since Claude Code 2.1.296 (the earliest version observed), a Claude Code session can move to the background.
A background session runs as a job inside a per-configuration-directory daemon (`claude daemon run`), hosted by a PTY host process (`claude bg-pty-host`) that runs a background worker (a `claude` process whose session record has `kind: "bg"` and a `jobId`).
A session gets there in two ways: an interactive terminal session (TUI) is parked into the daemon (the TUI's session record then carries `parkedJobId`, and the TUI keeps showing the job), or a job is started in the background directly and later viewed with `claude attach <jobId>`.
Hooks of a background job run inside the worker, so they inherit the daemon's environment, not the environment of the terminal that shows the job.
The daemon carries the Zellij identity of whichever process started it, and the PTY host removes `ZELLIJ` but keeps `ZELLIJ_PANE_ID` and `ZELLIJ_SESSION_NAME`.

cc-deck's Claude Code hooks read that inherited pane identity.
Because `ZELLIJ` is missing, every background event is dropped as "not in Zellij", so the row of the pane that shows the job freezes (observed on 2026-10-16: about 1,350 events of job `bac0e04c` dropped while its pane 39 row stayed stale).
If a future Claude Code release kept `ZELLIJ` as well, the same events would land in the row of an unrelated pane.
Viewing a job with `claude attach` has the same problem: the `claude attach` process has no session record of its own; its job id appears only in its arguments and its pane only in its environment.
The upstream report is anthropics/claude-code#102949.

## Clarifications

### Session 2026-10-17 (brainstorm discussion)

- Q: Which pane owns a background job's events? → A: The pane that currently views the job: the TUI that parked it, or a `claude attach` viewer of the job.
- Q: What happens to a job that no terminal views (headless)? → A: It gets no sidebar row; its events are dropped.
- Q: What happens when several terminals view the same job? → A: The event is dropped, following the "never a wrong row" rule of spec 089.
- Q: How is the viewer found? → A: Through exact mapping on Claude Code's own records (the worker's session record and job id, interactive records with `parkedJobId`, `claude attach <jobId>` arguments), never through working directory heuristics. Missing or inconsistent data drops the event.

### Session 2026-10-18

- Q: What happens to a pane's row when the job loses its viewer (detach, or the parked TUI exits)? → A: On the job's first event after the viewer is gone, cc-deck removes the row of the pane that last showed the job, provided that row still belongs to the job and that pane no longer views it.
- Q: Should the sidebar show that a row is fed by a background job? → A: Yes, a small background marker in the row while its latest event came from a background job; no job id is shown.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A session moved to the background keeps its row (Priority: P1)

A user works with Claude Code in a Zellij pane.
The session moves to the background (on request, by keybinding, or by a slash command), and the TUI in the same pane keeps showing the session.
The pane's sidebar row keeps following the session's status (working, waiting for input, done), as before the move, and shows a small background marker so the user can see that the work now runs as a background job.

**Why this priority**: This is the reported defect. A parked session silently freezes its row, and the user loses the main signal cc-deck provides.

**Independent Test**: Start Claude Code in pane A, move the session to the background while the TUI stays open, submit a prompt that runs a tool, and observe that pane A's row changes status with the job.

**Acceptance Scenarios**:

1. **Given** a Claude Code session in pane A whose daemon was started from another pane B, **When** the session is parked and the TUI in pane A keeps showing it, **Then** all of the job's hook events update pane A's row and none update pane B's row.
2. **Given** a parked session whose daemon was started from a pane that no longer exists, **When** the job runs a tool, **Then** pane A's row shows the change by that event.
3. **Given** a parked session, **When** the job continues under a session id different from the original TUI session, **Then** pane A still shows one row for the work, not a second row.
5. **Given** a parked session shown in pane A, **When** the job's first event reaches pane A's row, **Then** the row shows the background marker; **and when** the user later resumes the session in the foreground in pane A, **Then** the marker disappears with the first foreground event.
4. **Given** a Claude Code session that was never moved to the background, **When** it runs, **Then** its row behaves as before this feature.

---

### User Story 2 - Attaching to a job shows it in the attaching pane (Priority: P1)

A user runs `claude attach <jobId>` in a Zellij pane to view a background job (for example a job started earlier from another pane, or one started directly in the background).
The attaching pane's row shows the job and follows its status for as long as the viewer stays attached.

**Why this priority**: Attaching is the other common way users view background jobs, and it fails the same way today (observed on job `2402e991`, whose worker carries pane 3 while its viewer runs in pane 9).

**Independent Test**: Start a job in the background, attach to it with `claude attach <jobId>` in pane C, drive it through a prompt, and observe that pane C's row reflects the job.

**Acceptance Scenarios**:

1. **Given** a background job and no viewer, **When** the user attaches to it in pane C and the job runs a tool, **Then** pane C's row shows the job's status.
2. **Given** a job viewed in pane C, **When** the user detaches and attaches again in pane D, **Then** later events update pane D's row, and pane C's row for the job is removed by the job's first event after the move.
3. **Given** a job viewed in pane C, **When** the user detaches and the job runs on headless, **Then** pane C's row for the job is removed by the job's next event, and no other row changes.
4. **Given** a job last shown in pane C, **When** pane C now runs another agent with its own row and the job sends an event, **Then** pane C's row is left unchanged.
5. **Given** a job whose worker carries the pane identity of pane E, **When** the job is viewed in pane C, **Then** no event of the job updates pane E's row.

---

### User Story 3 - Unviewed or ambiguous jobs never land in a wrong row (Priority: P2)

Background jobs that no terminal shows, jobs viewed by several terminals, and jobs viewed from outside the cc-deck Zellij session never update a row that does not show them.

**Why this priority**: The daemon's inherited identity names some pane for every job; without these rules, jobs would appear in rows of unrelated panes.

**Independent Test**: Run a headless job, a job attached in two panes, and a job attached in a plain terminal outside Zellij, drive each through a prompt, and confirm no sidebar row changes for any of them.

**Acceptance Scenarios**:

1. **Given** a background job with no viewer, **When** it runs a prompt, **Then** no sidebar row is created or changed.
2. **Given** a job viewed in two panes at once, **When** it runs a prompt, **Then** no sidebar row is created or changed, and attribution resumes as soon as exactly one viewer remains.
3. **Given** a job viewed only from a terminal outside Zellij, **When** it runs a prompt, **Then** no sidebar in any Zellij session shows it.
4. **Given** a job viewed only from a pane in Zellij session S2 while the daemon's identity names session S1, **When** it runs a prompt, **Then** only S2's sidebar may show it, in the viewer's pane, and S1's sidebar does not change.

---

### User Story 4 - Diagnosing background attribution (Priority: P3)

A user whose background session does not show up enables cc-deck's hook debug log and can see, per event, whether the event came from a background job, which viewer was chosen, or why the event was dropped.

**Why this priority**: The defect stayed invisible for a day because the log only showed "not in Zellij"; explicit reasons make the next problem visible.

**Independent Test**: Enable hook debugging, reproduce each path (foreground session, parked TUI viewer, attach viewer, headless, several viewers, viewer outside Zellij, missing record), and check that each produces a distinct, readable log entry.

**Acceptance Scenarios**:

1. **Given** hook debugging is enabled, **When** a background event is attributed or dropped, **Then** the debug log records the event, the session, the job, the resulting pane (or none), and the reason.

---

### Edge Cases

- The worker's inherited environment carries `ZELLIJ` and a pane id of a live pane (a future Claude Code release, or a daemon started differently): the inherited identity is still not used.
- The worker's inherited environment carries no Zellij identity at all (daemon started from a plain terminal, or Claude Code strips the identity as asked in the upstream issue): the event is still attributed to the viewer's pane.
- The parked TUI starts a new conversation, exits, or otherwise stops showing the job: later events of the job go to the remaining viewer, or are dropped when none is left.
- A viewer exits and the operating system reuses its process id: the old viewer is not used.
- A job was adopted by a newer daemon after a Claude Code update: attribution works the same, because it does not depend on which daemon instance spawned the worker.
- Several Claude Code configuration directories are in use (cc-deck profiles with `CLAUDE_CONFIG_DIR`): only viewers using the same configuration directory as the worker count, because job ids belong to one daemon.
- Two configuration directories share their session records (a profile that links its sessions directory to the default one): viewers are still matched only to jobs of their own daemon.
- A viewer's record or arguments cannot be read (permission denied, process exited during lookup, malformed record): the event is dropped, not misattributed.
- A Claude Code subagent or a hook-spawned helper runs below the worker: its events count as events of the job.
- The job ends (`SessionEnd` or the daemon retires it): background events never write session-to-pane mappings, but a `SessionEnd` event still removes any mapping left for the session (for example one recorded while the session ran in the foreground).
- A viewer attaches to an idle job: the viewer's row changes only with the job's next hook event, because cc-deck learns about the job through hook events.
- A future Claude Code release passes the viewer's identity to hooks: that identity takes precedence over record lookup.
- Codex and OpenCode hook events: behavior is unchanged.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: For Claude Code hook events that run inside a background job, cc-deck MUST attribute each event to the Zellij pane of the terminal that currently views the job, not to the pane identity the hook process inherited from the daemon.
- **FR-002**: A Claude Code hook runs inside a background job when one of its ancestor processes is a Claude Code background worker (a process with a session record of kind `bg`) or a Claude Code PTY host. For such events cc-deck MUST NOT use the inherited pane identity, the inherited Zellij membership, or any session-to-pane cache entry, and MUST NOT write session-to-pane cache entries, because the viewer can change between any two events; each background event resolves its viewer again (FR-006).
- **FR-003**: When a Claude Code hook does not run inside a background job, cc-deck MUST keep today's attribution, including the "not in Zellij" rule.
- **FR-004**: cc-deck MUST determine the job of a background event from the worker's own session record (its job id), and MUST find the job's viewers among the user's running Claude Code processes that use the same configuration directory as the worker. A viewer is an interactive Claude Code process whose session record names the job as parked (`parkedJobId`), or a `claude attach` process whose arguments name the job.
- **FR-005**: When exactly one viewer exists and it runs inside Zellij, cc-deck MUST deliver the event to that viewer's own Zellij session and pane. Events whose single viewer runs outside Zellij MUST be dropped.
- **FR-006**: When no viewer exists, or several viewers exist, cc-deck MUST drop the event for the sidebar (no row created or changed). The next event of the job MUST resolve the viewer again, so attribution resumes as soon as exactly one viewer remains.
- **FR-007**: Attribution MUST follow viewer changes: after a viewer detaches and another one attaches, the job's next event MUST go to the new viewer's pane and not to the former one.
- **FR-007a**: cc-deck MUST remember, per job, the pane and Zellij session that last received one of the job's events. When a later event of the job finds that pane no longer viewing the job (no viewer, or the viewer is now in another pane), cc-deck MUST remove that pane's row once, but only if the row still belongs to the job's session; a row that now belongs to another agent or session MUST NOT be touched. This memory is used only for removal, never for attribution (FR-002).
- **FR-008**: A viewer MUST be identified by its process id together with its process start time, so that a reused process id is never taken for the former viewer.
- **FR-009**: When the job's events carry a session id different from the session the pane showed before parking, the pane MUST keep showing a single row for the work. A manual rename of that row MUST survive; derived row fields (generated name, working directory, profile) MAY reset and MUST be filled again from the job's following events.
- **FR-010**: When a hook payload or environment provides the viewer's pane identity explicitly (for example a future Claude Code field), cc-deck MUST prefer that identity over record lookup.
- **FR-011**: Missing, unreadable, or inconsistent records (no job id in the worker record, a record of another user, a viewer that exits during lookup) MUST lead to a dropped event, never to attribution by guess.
- **FR-012**: For attributed background events, profile enrichment (profile name and color in the sidebar) MUST use the viewer's profile identity, not the identity the daemon inherited.
- **FR-013**: Codex and OpenCode hook handling MUST remain unchanged.
- **FR-014**: When hook debugging is enabled, cc-deck MUST log for each background Claude Code event the event name, the session, the job, the resolved pane (or none), and one resolution reason out of: explicit viewer identity, parked viewer, attach viewer, headless, several viewers, viewer outside Zellij, record missing. When FR-007a removes a row, the log MUST also record the removed pane and its Zellij session.
- **FR-014a**: Every event cc-deck delivers for a background job MUST mark the payload as coming from a background job, and foreground events (of any agent) MUST NOT carry that mark, so their payloads stay as today. The sidebar MUST show a small background marker in a row whose latest delivered event came from a background job and MUST remove the marker with the row's next foreground event. The marker is drawn flush-right at the end of line 1, and its reserved columns come off the right of the name budget, so session names stay left-aligned across background and non-background rows.
The marker MUST NOT displace the status indicator, the profile bar, the session name's start column, or the second row line, and MUST stay legible with every profile color. Older plugin builds that do not know the field MUST ignore it.
- **FR-015**: Users MUST NOT need to change how they start, park, or attach Claude Code sessions, the Claude Code configuration, or the cc-deck hook installation for attribution to work.
- **FR-016**: The documentation MUST describe background sessions, the attribution rules (viewer pane, headless jobs hidden, several viewers hidden, stale row removal), the background marker, the link to the upstream issue, and the workaround of resuming the session in the foreground.

### Key Entities

- **Background worker**: The Claude Code process that runs a job inside the daemon; has a session record of kind `bg` with a job id and a session id, a configuration directory, and the daemon's inherited environment.
- **Job**: A background unit of work identified by a short job id, owned by one daemon (one configuration directory); its session id can differ from the session it continues.
- **Viewer**: A terminal process that currently shows a job: either the interactive TUI that parked it (its record names the job as parked) or a `claude attach` process (its arguments name the job); has a pane and Zellij session identity when inside Zellij.
- **Last viewer pane**: Per job, the pane and Zellij session that last received one of the job's events, plus the session id shown there; used only to remove a stale row (FR-007a).
- **Daemon**: The per-configuration-directory supervisor that hosts jobs and carries the environment of the process that started it.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After a session is moved to the background while its TUI stays open, 100% of the job's hook events update the TUI's pane row in repeated trials, and 0 update another row.
- **SC-002**: After `claude attach` in a pane, the job's next hook event updates that pane's row in 100% of trials.
- **SC-002a**: After a viewer leaves, the former viewer's row for the job is gone after the job's next hook event in 100% of trials, and 0 rows of other agents or sessions are removed.
- **SC-003**: Across all tested scenarios (headless, several viewers, viewer outside Zellij, missing records, stale inherited identity), 0 events are attributed to a wrong pane.
- **SC-004**: Resolving a background event adds no more than 150 ms to hook processing on a machine with up to 30 running Claude Code processes.
- **SC-004a**: In all tested scenarios, 100% of rows whose latest event came from a background job show the background marker, and 0 rows fed by foreground sessions show it.
- **SC-005**: Foreground Claude Code sessions, Codex, and OpenCode sidebar behavior is unchanged (no background marker appears), verified by the existing hook test suite passing without modification.

## Assumptions

- Claude Code session records live in `<config dir>/sessions/<pid>.json` and keep the fields observed in 2.1.296 to 2.1.302 (`kind`, `jobId`, `sessionId`, `parkedJobId`, `procStart`); the plan verifies the exact lifetime of `parkedJobId` (set while the parking TUI shows the job, cleared when it stops).
- `claude attach` keeps taking the job id as a positional argument, and the attach process carries its own pane's Zellij environment.
- Viewers, workers, and the hook run as the same user, so the hook can read their arguments, environment, and records on macOS and Linux; the process inspection support added for spec 089 can be reused.
- Changing Claude Code's daemon behavior is out of scope; the upstream issue anthropics/claude-code#102949 asks for consistent stripping of terminal identity and for viewer context in hooks, and this feature keeps working if either ships.
- Remote workspaces (SSH, OpenShell) behave like the local case when Claude Code runs there with background sessions.

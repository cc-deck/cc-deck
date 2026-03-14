# Feature Specification: Demo Recording System

**Feature Branch**: `020-demo-recordings`
**Created**: 2026-03-14
**Status**: Draft
**Input**: User description: "Automated demo recording system for cc-deck with scripted terminal demos, voiceover generation, and multiple output formats"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Record a Scripted Plugin Demo (Priority: P1)

A project maintainer wants to produce a terminal recording that showcases the cc-deck sidebar plugin in action: installing the plugin, launching Zellij, opening multiple Claude Code sessions, navigating between them, and using smart attend to cycle through sessions that need attention. The recording must be fully scripted so it can be re-run after every release to keep the demo current.

**Why this priority**: The plugin demo is the primary marketing and onboarding asset. Without a visual demonstration, potential users cannot understand what cc-deck does. A repeatable recording ensures the demo always matches the latest release.

**Independent Test**: Can be fully tested by running the demo script in a terminal with Zellij installed and verifying the recording captures all plugin interactions (sidebar rendering, navigation, smart attend, tab switching).

**Acceptance Scenarios**:

1. **Given** the demo runner and demo projects are set up, **When** the maintainer runs the plugin demo script, **Then** a terminal recording is produced that shows plugin installation, session creation, sidebar navigation, and smart attend in action.
2. **Given** the plugin has been updated, **When** the maintainer re-runs the same demo script, **Then** a new recording is produced reflecting the current plugin behavior without manual editing.
3. **Given** the recording is complete, **When** converted to a short looping format, **Then** the output fits within 60 seconds and highlights the core value proposition.

---

### User Story 2 - Control the Plugin via Scripted Commands (Priority: P1)

A demo script author needs to programmatically trigger plugin actions (toggle navigation mode, move cursor up/down, select a session, invoke smart attend) without simulating keyboard input. The plugin must accept external commands so that demo scripts can drive it like a remote control.

**Why this priority**: Without programmatic control, demo scripts would need fragile key simulation tools that vary across operating systems. Pipe-based control makes demos reliable and portable, and has value beyond demos (automation, CI integration, accessibility).

**Independent Test**: Can be tested by sending pipe commands to the plugin from a shell script and verifying the plugin responds with the correct action (cursor moves, tab switches, mode toggles).

**Acceptance Scenarios**:

1. **Given** the plugin is running in a Zellij session, **When** a pipe command for "navigate:toggle" is sent, **Then** the plugin enters navigation mode, visually identical to pressing the navigation keybinding.
2. **Given** the plugin is in navigation mode, **When** pipe commands for "navigate:down" and "navigate:select" are sent in sequence, **Then** the cursor moves down and the selected session tab becomes active.
3. **Given** multiple sessions exist with varying states, **When** a pipe command for "attend" is sent, **Then** the plugin executes the smart attend algorithm and switches to the highest-priority session.

---

### User Story 3 - Create Realistic Demo Projects (Priority: P2)

A demo script needs a set of small, recognizable source code projects that viewers immediately understand. Each project should have a pre-staged task for Claude Code to work on, producing visible progress in the recording. The projects must look authentic (git history, README, clear structure) while being small enough that Claude Code completes the task within a predictable time window.

**Why this priority**: The demo is only compelling if viewers understand what is happening in each session. Generic or project-internal repos create confusion. Well-crafted demo projects make the value proposition self-evident.

**Independent Test**: Can be tested by setting up the demo projects, launching Claude Code in each, and verifying that Claude Code can complete the pre-staged task within 2 minutes.

**Acceptance Scenarios**:

1. **Given** the demo project setup script is available, **When** it is executed, **Then** three distinct demo projects are created in a temporary directory, each with a README, git history, and a pre-staged task.
2. **Given** a demo project exists, **When** Claude Code is started in that project, **Then** it discovers and begins working on the pre-staged task without manual guidance.
3. **Given** all three demo projects are set up, **When** viewed side by side, **Then** each project uses a different language/framework and has a clearly different tab name in the sidebar.

---

### User Story 4 - Generate Voiceover Audio (Priority: P3)

A maintainer wants to add narration to the long-form demo video. A narration script (plain text with chapter markers) is converted to audio using a text-to-speech service. The audio aligns with the chapter markers from the demo recording so that narration matches the on-screen action.

**Why this priority**: Voiceover adds significant production value to the long-form demo intended for team sharing and presentations. However, the core demo recordings are valuable without narration, making this an enhancement rather than a prerequisite.

**Independent Test**: Can be tested by providing a narration script and verifying the generated audio file matches the expected duration and chapter timing.

**Acceptance Scenarios**:

1. **Given** a narration script with chapter markers, **When** the voiceover generation tool is run, **Then** an audio file is produced with speech matching the script text.
2. **Given** the generated audio and a demo recording with matching chapter markers, **When** combined, **Then** the narration aligns with the corresponding visual sections within a 1-second tolerance.

---

### User Story 5 - Produce Multiple Output Formats (Priority: P3)

A maintainer needs different output formats from the same recording: a short looping clip for the landing page hero section, a full-length video with voiceover for team sharing, and an embeddable recording for documentation pages. Each format has different requirements for duration, resolution, and delivery.

**Why this priority**: Different audiences consume content differently. The landing page needs a quick visual hook, the team needs a thorough walkthrough, and documentation readers need step-by-step tutorials. Producing all formats from a single recording run maximizes the investment in scripting.

**Independent Test**: Can be tested by running the conversion pipeline on a completed recording and verifying each output format meets its specification (duration, file size, embedding compatibility).

**Acceptance Scenarios**:

1. **Given** a completed terminal recording, **When** the landing page conversion is run, **Then** a looping clip under 60 seconds is produced in a web-embeddable format (GIF or MP4).
2. **Given** a completed recording and voiceover audio, **When** the full video conversion is run, **Then** an MP4 file is produced combining the terminal recording with narration audio.
3. **Given** a completed recording, **When** the documentation conversion is run, **Then** an embeddable format suitable for documentation pages is produced.

---

### Edge Cases

- What happens when Claude Code takes longer than expected to complete a demo task? The demo runner should have configurable timeouts with graceful fallback (skip to next scene).
- How does the system handle a Zellij crash or plugin failure during recording? The demo runner should detect the failure, log the error, and allow resuming from the last completed scene.
- What happens when the text-to-speech API is unavailable? The system should gracefully degrade by producing the video without voiceover and logging a warning.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The plugin MUST accept pipe messages that trigger navigation mode toggle, cursor movement (up/down), session selection, smart attend, pause toggle, and help display.
- **FR-002**: The demo runner MUST provide helper functions for scene management (scene markers, pauses, checkpoint-based waits).
- **FR-003**: The demo runner MUST create and tear down demo projects (setup and cleanup scripts).
- **FR-004**: Each demo project MUST include a git repository with at least 2 commits, a README, and a CLAUDE.md file that directs Claude Code to a specific task.
- **FR-005**: The demo runner MUST integrate with a terminal recording tool for capturing sessions (start, stop, metadata).
- **FR-006**: The system MUST provide a voiceover generation script that converts a narration text file to audio using a text-to-speech service.
- **FR-007**: The system MUST provide conversion scripts for producing landing page clips (under 60 seconds), full-length videos with audio, and documentation-embeddable formats.
- **FR-008**: Demo scripts MUST use terminal multiplexer CLI commands for tab and pane management and pipe messages for plugin interactions, with no operating-system-level key simulation.
- **FR-009**: The demo runner MUST support checkpoint-based timing (wait for specific terminal output patterns) rather than fixed-duration sleeps.
- **FR-010**: The system MUST include a pre-built image builder manifest for the demo projects, used in the image builder demo.

### Key Entities

- **Demo Project**: A small source code repository with a language, framework, pre-staged task, and expected completion time. Includes git history and task guidance for Claude Code.
- **Demo Script**: A shell script organized as a sequence of scenes. Each scene has a name, a set of actions (commands, pipe messages, waits), and chapter markers for voiceover alignment.
- **Narration Script**: A plain text file with chapter markers matching demo script scenes, containing the voiceover text for each section.
- **Demo Recording**: A terminal recording file produced by running a demo script, serving as the source for all output format conversions.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A maintainer can produce a complete plugin demo recording by running a single command, with no manual interaction during the recording.
- **SC-002**: The same demo script produces visually consistent recordings across multiple runs (same scene order, same visual elements, predictable timing).
- **SC-003**: The landing page clip conveys the core value proposition (sidebar with multiple sessions, smart attend switching) in under 60 seconds.
- **SC-004**: All plugin interactions in the demo are driven via pipe commands, requiring zero key simulation tools.
- **SC-005**: Demo projects can be set up and torn down in under 30 seconds.
- **SC-006**: The full recording-to-output pipeline (record, convert, produce all formats) completes in under 15 minutes for a 5-minute demo.

## Assumptions

- The terminal multiplexer supports pipe/message-based communication with plugins.
- A text-to-speech API key is available as an environment variable for voiceover generation.
- A terminal recording tool (such as asciinema) and conversion tools are available in the recording environment.
- Demo projects use Claude Code in a mode where pre-staged tasks are picked up automatically (via project-level instructions).
- The demo environment has the terminal multiplexer, the cc-deck plugin, and Claude Code installed.

## Scope Boundaries

### In Scope

- Plugin pipe message handler for programmatic control
- Three demo projects (Python, Go, HTML/CSS)
- Demo runner framework (shell script helpers)
- Three demo scripts (plugin features, image deployment, custom image creation)
- Voiceover generation via text-to-speech API
- Output format conversion (landing page clip, full video, docs embed)
- Pre-built image builder manifest for demo

### Out of Scope

- Voice cloning or custom voice models
- Live interactive demos (only pre-scripted recordings)
- Edge case coverage in demos (focus on happy path)
- Kubernetes deployment demo (future enhancement)
- CI/CD integration for automatic demo re-recording on release

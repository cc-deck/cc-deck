# Feature Specification: Voice Relay

**Feature Branch**: `042-voice-relay`
**Created**: 2026-04-24
**Status**: Draft
**Input**: [Brainstorm 042 - Voice Relay for Remote Workspaces](../../brainstorm/042-voice-relay.md)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Voice Dictation to Active Agent Session (Priority: P1)

A developer working with a remote workspace wants to dictate prompts to Claude Code instead of typing. The developer runs a voice command from a separate terminal on their local machine. The system captures audio from the local microphone, transcribes it using a local speech recognition model, and relays the transcribed text into the currently attended agent pane in the remote workspace. The text appears in the agent's input as if the developer had typed it. The developer sees what was transcribed and where it was delivered in a real-time terminal display.

**Why this priority**: This is the core value proposition. Without reliable audio-to-text-to-agent-pane delivery, no other voice feature matters. It validates the full pipeline across the local-remote boundary.

**Independent Test**: Can be fully tested by speaking into the microphone while a workspace is running, then verifying the transcribed text appears in the attended agent pane. Delivers immediate value for hands-free coding workflows.

**Acceptance Scenarios**:

1. **Given** a running workspace with an active Claude Code session, **When** the developer speaks "add error handling to the API endpoint" into their microphone while the voice relay is active, **Then** the text "add error handling to the API endpoint" appears in the attended agent pane's terminal input.
2. **Given** a running container workspace, **When** the developer dictates text via voice relay, **Then** the text is delivered to the remote workspace through PipeChannel and injected into the attended pane without the developer needing to manually switch focus.
3. **Given** a running SSH workspace, **When** the developer dictates text, **Then** the text is delivered via PipeChannel over the SSH transport.
4. **Given** a running K8s deploy workspace, **When** the developer dictates text, **Then** the text is delivered via PipeChannel over kubectl exec.
5. **Given** a running compose workspace, **When** the developer dictates text, **Then** the text is delivered via PipeChannel to the compose service container.
6. **Given** a running local workspace, **When** the developer dictates text, **Then** the text is delivered via PipeChannel using the local zellij pipe.
7. **Given** the voice relay is active, **When** the developer speaks, **Then** a terminal display shows the audio level, transcribed text, and delivery status in real time.
8. **Given** a workspace that is not running, **When** the developer starts voice relay, **Then** a clear error is returned indicating the workspace is unavailable.

---

### User Story 2 - Voice-Triggered Prompt Submission (Priority: P1)

A developer who has finished dictating a prompt wants to submit it without switching focus or reaching for the keyboard. The developer speaks a designated command word ("submit" or "enter") as a standalone utterance. The system recognizes this as a submission command and sends a newline character to the agent pane, which submits the prompt.

**Why this priority**: Without prompt submission, voice relay is only half-useful. The developer would still need to switch to the agent pane and press Enter. This completes the hands-free workflow loop.

**Independent Test**: Can be tested by dictating a prompt followed by a pause, then saying "submit". Verify the prompt text is followed by a newline that triggers agent processing.

**Acceptance Scenarios**:

1. **Given** the developer has dictated text into the agent pane, **When** the developer says "submit" as a standalone utterance (after a pause), **Then** a newline character is sent to the agent pane, submitting the prompt.
2. **Given** the developer has dictated text, **When** the developer says "enter" as a standalone utterance, **Then** a newline character is sent to the agent pane.
3. **Given** the developer dictates "please submit the form", **When** the system transcribes this text, **Then** the full sentence is relayed as-is and no newline is injected, because "submit" was not a standalone utterance.
4. **Given** the developer dictates "press enter to continue", **When** the system transcribes this, **Then** the full sentence is relayed without injecting a newline.
5. **Given** the developer says "okay submit" as an utterance, **When** the system transcribes this, **Then** the full text "okay submit" is relayed without injecting a newline, because the transcription contains additional words beyond the command word.
6. **Given** the developer says "submit it" as an utterance, **When** the system transcribes this, **Then** the full text "submit it" is relayed without injecting a newline.
7. **Given** the developer says "um, submit" as an utterance, **When** the system transcribes this, **Then** a newline character is sent, because "um" is a filler word and the remaining content is exactly the command word.

---

### User Story 3 - Push-to-Talk via Zellij Keybinding (Priority: P2)

A developer in a noisy environment wants to control when audio is captured. Instead of continuous listening, they press a designated key (configurable, default F8) to start recording and press it again to stop. The key works from any pane in the Zellij session, including the Claude Code pane they are watching, so they never need to switch focus to the voice relay terminal.

**Why this priority**: PTT is essential for noisy environments and for developers who prefer explicit control over recording. The Zellij keybinding approach solves the focus-switching problem that would otherwise make PTT impractical.

**Independent Test**: Can be tested by pressing the PTT key in a Claude Code pane, speaking, pressing the key again, and verifying the text is transcribed and delivered. No focus switching to the voice relay terminal should be needed.

**Acceptance Scenarios**:

1. **Given** voice relay is running in VAD mode, **When** the developer switches to PTT mode, **Then** audio capture stops automatically and waits for the PTT key.
2. **Given** voice relay is running in PTT mode, **When** the developer presses the PTT key (F8) from any pane in the Zellij session, **Then** audio recording starts.
3. **Given** audio recording is active in PTT mode, **When** the developer presses the PTT key again, **Then** recording stops and the captured audio is sent for transcription.
4. **Given** voice relay is running in PTT mode and the developer is focused on a Claude Code pane, **When** the developer presses the PTT key, **Then** the key press is captured by the Zellij keybinding system and relayed to the voice process without interfering with the Claude Code session.
5. **Given** voice relay is running in PTT mode targeting a remote workspace, **When** the developer presses the PTT key, **Then** the toggle signal reaches the voice process within 500ms for all workspace types.

---

### User Story 4 - Permission-Safe Voice Relay (Priority: P2, Status: Deferred)

A developer is using voice relay while Claude Code enters a permission prompt ("Allow tool execution? [y]es / [n]o / [a]lways"). Voice text must not be injected into the terminal during this state, as it could accidentally approve or deny actions. The system detects the permission state and pauses text relay until the developer handles the prompt manually.

**Why this priority**: Without permission-state awareness, voice relay could approve destructive tool executions. This is a safety requirement that prevents data loss or unintended side effects.

**Deferral note**: Permission-state detection and text relay pausing are not implemented in the initial release. Users should be aware that voice-dictated text continues to be injected during permission prompts. This is tracked as a follow-up safety improvement. The risk is mitigated by the fact that voice text is unlikely to match "y", "n", or "a" exactly as standalone characters, but is not eliminated.

**Independent Test**: Can be tested by starting voice relay, triggering a permission prompt in Claude Code, dictating text, and verifying the text is not injected until after the permission is resolved.

**Acceptance Scenarios**:

1. **Given** the attended session is in a permission prompt state, **When** the developer dictates text, **Then** the text is buffered and not injected into the terminal.
2. **Given** the attended session is in a permission prompt state, **When** the voice relay detects this state, **Then** the terminal display shows a warning that voice relay is paused.
3. **Given** voice text was buffered during a permission prompt, **When** the session leaves the permission state, **Then** the buffered text is discarded (the developer re-speaks if needed).
4. **Given** the attended session transitions from Working to Permission state while voice text is being dictated, **Then** any text already sent is delivered but new text is buffered until the permission is resolved.

---

### User Story 5 - Voice Setup and Model Management (Priority: P2)

A developer wants to start using voice relay for the first time. They run a setup command that checks for required external tools (whisper-server or whisper-cli), downloads a speech recognition model to a local cache directory, and verifies the audio capture pipeline works. The model is not bundled with the cc-deck binary, so developers who never use voice pay no storage cost.

**Why this priority**: The setup experience determines whether developers adopt voice relay. On-demand model download avoids bloating the cc-deck installation for users who don't need voice.

**Independent Test**: Can be tested on a clean machine by running the setup command and verifying it detects missing tools, downloads the model, and reports readiness.

**Acceptance Scenarios**:

1. **Given** a developer has never used voice relay, **When** they run the setup command, **Then** the system checks for whisper-server or whisper-cli and reports whether they are installed.
2. **Given** the transcription tool is installed but no model exists, **When** the developer runs the setup command, **Then** the default model (base.en, ~57 MB) is downloaded to the local cache directory with a progress indicator.
3. **Given** the developer wants a different model size, **When** they specify a model name (tiny, base, small, medium), **Then** that model is downloaded instead of the default.
4. **Given** the transcription tool is not installed, **When** the developer runs the setup command, **Then** the system prints clear instructions for installing the tool on the developer's platform.
5. **Given** setup has completed successfully, **When** the developer runs voice relay, **Then** the system starts without repeating the setup steps.
6. **Given** the developer runs voice relay without prior setup, **When** the system detects missing dependencies, **Then** it shows an error with instructions to run the setup command.

---

### User Story 6 - Session Focus Tracking (Priority: P3)

A developer has multiple Claude Code sessions running across tabs. When the developer switches attention to a different session (via the cc-deck attend mechanism or by focusing a different pane), voice relay automatically follows. The text always goes to the session the developer is currently watching.

**Why this priority**: Multi-session workflows are common in cc-deck. Voice relay must follow attention naturally without explicit target switching.

**Independent Test**: Can be tested by starting voice relay, switching to a different session via attend, dictating text, and verifying it arrives in the newly attended session.

**Acceptance Scenarios**:

1. **Given** voice relay is active and the developer attends a different session, **When** the developer dictates text, **Then** the text is delivered to the newly attended session's pane.
2. **Given** voice relay is active and the developer manually focuses a different Claude Code pane, **When** the developer dictates text, **Then** the text is delivered to the newly focused pane.
3. **Given** voice relay is active and no session is attended (e.g., all sessions are in Done state), **When** the developer dictates text, **Then** the terminal display shows a warning that no target session is available and the text is not delivered.

---

### Edge Cases

- **Audio device unavailable**: If the microphone is not accessible (permissions denied, device not found), the voice relay reports a clear error at startup and does not proceed.
- **Transcription produces empty text**: If the speech recognition model returns an empty or whitespace-only result (e.g., background noise), the system silently discards the result without sending anything to the agent pane.
- **Workspace disconnects mid-relay**: If the workspace becomes unavailable (container stopped, SSH disconnected, Pod evicted) while voice relay is running, PipeChannel returns an error. The terminal display shows the delivery failure and voice relay continues capturing audio (text is discarded until the workspace reconnects).
- **Rapid successive utterances**: If the developer speaks multiple utterances faster than the transcription system can process them, utterances are queued and processed in order. No utterance is dropped.
- **Very long utterance**: If the developer speaks for longer than the transcription model's maximum window (30 seconds), the audio is split into chunks at VAD-detected boundaries and each chunk is transcribed separately.
- **PTT toggle during transcription**: If the developer presses the PTT key while a previous utterance is still being transcribed, the new recording starts immediately. The in-progress transcription completes and is delivered normally.
- **Transcription backend crash**: If the transcription backend crashes mid-session, the system automatically restarts it (up to 3 attempts) with a notification in the terminal display. Audio captured during restart is queued. If all retries fail, voice relay stops with an error message.
- **Corrupted or incomplete model**: If the cached model file is corrupted or incomplete (e.g., from an interrupted download), the system detects this at startup and prompts the developer to re-run the setup command to re-download the model.
- **Session focus change mid-utterance**: If the developer switches session focus while an utterance is being transcribed, the transcribed text is delivered to the session that was attended when the utterance started. The next utterance uses the new session target.
- **Multiple voice relay instances**: Only one voice relay instance per workspace is supported. Starting a second instance for the same workspace shows an error.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST capture audio from the local machine's microphone at 16kHz mono sample rate for speech recognition input. By default, the OS default input device is used. The developer MAY list available input devices via a `--list-devices` flag. *(Deferred: a `--device` flag for selecting a non-default input device is planned for a future iteration.)*
- **FR-002**: The system MUST provide voice activity detection that segments continuous audio into discrete utterances based on energy thresholds and silence duration.
- **FR-003**: The system MUST transcribe audio utterances to text using a local speech recognition model, without sending audio data to external services.
- **FR-004**: The system MUST relay transcribed text to the attended agent pane in the target workspace via PipeChannel, working across all workspace types (local, container, compose, SSH, k8s-deploy).
- **FR-005**: The system MUST inject relayed text into the agent pane using the Zellij plugin's text injection API, so the user does not need to switch focus to the target pane.
- **FR-006**: The system MUST recognize designated command words ("submit", "enter") and send a newline character instead of the spoken text. A command word is "standalone" when the entire transcription result, after trimming whitespace and removing filler words ("um", "uh", "hmm", "ah", "er"), equals exactly the command word. Any utterance containing additional non-filler words is relayed as regular text.
- **FR-007**: The system SHOULD detect when the attended session is in a permission prompt state and pause text relay, preventing accidental permission responses. *(Known limitation: permission-state pause is not yet implemented. Users should be aware that voice-dictated text may be injected during permission prompts. This is tracked as a follow-up safety improvement.)*
- **FR-008**: The system MUST support a push-to-talk mode where a configurable Zellij keybinding (default F8) toggles recording on and off from any pane in the Zellij session.
- **FR-009**: The PTT toggle MUST work from any pane in the Zellij session without requiring the developer to switch focus to the voice relay terminal.
- **FR-010**: The system MUST support bidirectional communication between the local voice process and the remote workspace plugin, enabling the plugin to signal the voice process asynchronously (e.g., in response to key presses).
- **FR-011**: The system MUST display a real-time terminal interface showing audio level, current mode (VAD/PTT), transcription results, delivery status, and target session information.
- **FR-012**: The system MUST download speech recognition models on demand to a local cache directory, so developers who never use voice pay no storage or download cost.
- **FR-013**: The system MUST provide a setup command that verifies external tool availability, downloads the default model, and reports readiness.
- **FR-014**: The system MUST provide a generic pipe CLI command that sends arbitrary text to a named pipe in any workspace, reusable beyond voice relay.
- **FR-015**: The system MUST automatically follow session focus changes, delivering voice text to whichever session the developer most recently attended.
- **FR-016**: The system SHOULD discard buffered voice text when a permission prompt resolves, rather than flushing potentially stale dictation. *(Deferred: depends on FR-007 permission-state detection, which is not yet implemented.)*
- **FR-017**: The audio capture subsystem MUST work on systems both with and without CGo support, so that cross-compilation remains possible.
- **FR-019**: The system MUST support a `--verbose` flag that adds per-utterance transcription latency, audio sample rate, and transcription backend health status to the terminal display, without affecting the default non-verbose output.
- **FR-020**: The system MUST sanitize transcribed text before injection, stripping ANSI escape sequences and non-printable control characters. This prevents a compromised transcription backend from injecting terminal escape codes into the attended pane.
- **FR-018**: The voice relay MUST manage the transcription backend's lifecycle automatically (start on demand, stop on exit), so the developer does not need to manage it separately. If the backend crashes, the system MUST attempt up to 3 automatic restarts with a visible notification in the terminal display, then stop with an error if all retries fail.

### Non-Functional Requirements

- **NFR-001**: End-to-end latency from the developer finishing speaking to text appearing in the agent pane MUST NOT exceed 5 seconds. Of this budget, transcription may consume up to 3.5 seconds, and transport plus injection must complete within the remaining time.
- **NFR-002**: The voice relay process MUST NOT consume more than 200 MB of resident memory during normal operation (excluding the transcription backend, which runs as a separate process).
- **NFR-003**: Transcribed text in transit between the local machine and the remote workspace MUST use the same transport security as the underlying workspace connection (SSH encryption for SSH workspaces, kubectl's TLS for K8s workspaces). Voice relay MUST NOT introduce additional unencrypted channels.
- **NFR-004**: Audio data MUST remain on the local machine at all times. Only transcribed text is transmitted to the remote workspace.
- **NFR-005**: The voice relay MUST NOT log transcribed text to persistent storage by default, to protect developer privacy. Debug logging of transcription content MUST require an explicit opt-in flag.

### Key Entities

- **AudioSource**: The audio capture backend that provides a stream of PCM audio samples from the local microphone. Two implementations: integrated (library) and subprocess (external tool).
- **Transcriber**: The speech recognition backend that converts audio chunks into text. Three implementations: HTTP server, CLI subprocess, and embedded (build-tag opt-in).
- **VoiceRelay**: The orchestrator that connects audio capture, VAD, transcription, stopword detection, and PipeChannel delivery into a single pipeline.
- **VoiceControl**: The plugin-side state that tracks the long-poll pipe for PTT keybinding communication and the voice text buffer for permission-state pausing.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer can dictate a prompt and have it appear in a remote agent session within 5 seconds of finishing speaking (including transcription time).
- **SC-002**: Voice relay works across all five workspace types (local, container, compose, SSH, k8s-deploy) with identical user experience.
- **SC-003**: Push-to-talk toggle reaches the voice process within 500ms for all workspace types.
- **SC-004**: The setup command downloads the default model and verifies readiness in under 2 minutes on a typical broadband connection.
- **SC-005**: Voice relay does not inject text during permission prompts, preventing 100% of accidental permission responses.
- **SC-006**: Session focus changes are reflected in voice text routing within the next utterance (no lingering delivery to the previous session).
- **SC-007**: The cc-deck binary size does not increase by more than 100KB for users who never use voice features (models and transcription tools are external).
- **SC-008**: Voice relay correctly handles at least 95% of clearly spoken English utterances in a quiet environment (measured by comparing transcription output to intended input).

## Clarifications

### Session 2026-04-24

- Q: What words constitute the "common filler words" list for stopword detection? → A: Minimal disfluencies only: "um", "uh", "hmm", "ah", "er"
- Q: What should happen when the transcription backend crashes during a voice relay session? → A: Auto-restart with TUI notification showing the restart attempt (up to 3 retries, then stop with error)
- Q: Should voice relay support selecting a non-default audio input device? → A: Yes, support `--device` flag to select input device and `--list-devices` to enumerate available devices
- Q: What level of diagnostic output should voice relay provide? → A: TUI display plus `--verbose` flag showing transcription latency per utterance, audio sample rate, and backend health

## Assumptions

- The developer has a working microphone accessible to the local operating system. Microphone permissions (macOS privacy settings, Linux PulseAudio/ALSA access) are the developer's responsibility.
- The developer installs whisper-server or whisper-cli separately (via Homebrew or package manager). cc-deck does not bundle speech recognition tools.
- The Zellij session in the target workspace has the cc-deck plugin loaded and running. Voice relay depends on the plugin for text injection and PTT keybinding support.
- English-only models are the default. Multilingual support can be added by selecting a non-English model variant, but is not a primary design target.
- The developer has adequate disk space for the chosen model (57 MB for base.en quantized, up to 515 MB for medium).
- Network connectivity is not required during voice relay operation (transcription is local). Network is only needed during initial model download.
- A single voice relay instance per workspace is sufficient. Concurrent voice relay to multiple workspaces simultaneously is out of scope.
- Auto-start of whisper-server by the voice relay command is the expected default behavior.

## Changelog

- **2026-04-28 (post-implementation evolution)**:
  - FR-001: Deferred `--device` flag to future iteration; `--list-devices` is implemented
  - FR-007: Weakened MUST to SHOULD; permission-state pause deferred as follow-up
  - FR-016: Weakened MUST to SHOULD; depends on FR-007 implementation
  - FR-020: Added new requirement for terminal text sanitization (security hardening)
  - User Story 4: Marked as Deferred with risk documentation

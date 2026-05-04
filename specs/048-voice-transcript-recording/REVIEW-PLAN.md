# Review Guide: Voice Transcript Recording

**Spec:** [spec.md](spec.md) | **Plan:** [plan.md](plan.md) | **Tasks:** [tasks.md](tasks.md)
**Generated:** 2026-05-04

---

## What This Spec Does

Adds the ability to record voice relay transcriptions to a plain text file, controlled entirely from within the voice TUI via keyboard shortcuts. Users can start, pause, resume, and stop recording without leaving the TUI. The transcript captures raw recognized text with no metadata, making files directly usable as input for other tools.

**In scope:** Start/stop recording with filename prompt, pause/resume, visual recording indicator in header, mute-bypass (transcribe while muted if recording is active), plain text file output.

**Out of scope:** CLI flag for auto-start recording, filtering command words from transcript, audio recording, structured output formats (JSON, CSV). These boundaries are intentional: the feature targets a minimal, keyboard-driven workflow with maximum output simplicity.

## Bigger Picture

This is the third feature extending the voice relay subsystem. Spec 045 established the voice relay with `[[command]]` protocol and sidebar integration. Spec 046 added the "next" stop word for session cycling. This spec adds a data capture dimension: instead of speech flowing only to the attended pane, it can now be persisted.

The mute-bypass behavior (FR-011/FR-012) is the most architecturally significant change. It modifies a core relay invariant: until now, muted = no transcription. After this feature, muted + recording = transcription without delivery. This creates a new usage pattern (notes-to-self, commentary capture) that could influence future features.

---

## Spec Review Guide (30 minutes)

> Focus on the mute-bypass behavior and the state machine. The file I/O and TUI changes are straightforward extensions of existing patterns.

### Understanding the approach (8 min)

Read [User Story 3](spec.md#user-story-3---record-while-muted-priority-p3) and [FR-011/FR-012](spec.md#functional-requirements) for the mute-bypass design. As you read, consider:

- Does the distinction between "muted + recording = transcribe" and "muted + not recording = discard" feel intuitive to a user, or could it be confusing?
- The relay currently uses a simple `sync.Mutex`-protected bool for mute state. Adding a second bool (`recording`) checked in the same code path follows the pattern. Is this sufficient, or could the two flags interact in unexpected ways?

### Key decisions that need your eyes (12 min)

**Plain text format with no metadata** ([FR-004](spec.md#functional-requirements))

The transcript contains only recognized text, one line per utterance. No timestamps, no latency, no delivery status. This maximizes simplicity and reusability but loses context about when things were said.
- Question for reviewer: Is the absence of timestamps a problem for any anticipated use case? Would an optional `--timestamps` flag be worth adding later?

**Overloaded `r` key** ([FR-001](spec.md#functional-requirements), [FR-007](spec.md#functional-requirements))

The `r` key serves three functions depending on state: start (idle), pause (recording), resume (paused). This mirrors how many audio recorders work but could be surprising.
- Question for reviewer: Should pause/resume use a separate key to avoid accidental state changes?

**Mute-bypass only when recording** ([FR-011/FR-012](spec.md#functional-requirements))

The relay skips transcription entirely when muted (saves CPU). The spec adds an exception: if recording is active, transcribe anyway. This means mute becomes "don't send to pane" rather than "don't listen."
- Question for reviewer: Does this semantic shift in what "mute" means create user confusion? The TUI still shows "MUTED" but Whisper is running and consuming CPU.

**Default transcript directory** ([FR-013](spec.md#functional-requirements))

Transcripts go to `$XDG_DATA_HOME/cc-deck/transcripts/`. This follows XDG conventions and keeps transcripts separate from logs and config.
- Question for reviewer: Is `XDG_DATA_HOME` the right choice, or should transcripts go to `XDG_STATE_HOME` (where voice.log already lives)?

### Areas where I'm less certain (5 min)

- [FR-008](spec.md#functional-requirements): "While paused, transcriptions MUST continue to appear in the TUI history." This means pause only affects file output, not the TUI display. But the relay's `SetRecording` stays true during pause (so mute-bypass continues). If the user pauses and mutes, speech is still transcribed and shown in the TUI but not written to file. Is this the right behavior, or should pause also suppress mute-bypass?

- [Plan Phase 3, T006](plan.md#implementation-approach): The `updateFilenamePrompt` method must handle `relayEventMsg` to keep consuming relay events while the prompt is visible. If events are not consumed, the relay channel blocks. I'm confident this is the right approach (the device picker does the same), but the textinput key handling interaction with relay events needs careful testing.

### Risks and open questions (5 min)

- If Whisper transcription is slow (>1s per utterance), does the file write add noticeable latency to the TUI update cycle? File writes are single `fmt.Fprintf` calls to local disk, so likely negligible, but worth confirming under load.
- The spec assumes command words ("send", "next") are included in the transcript ([Assumptions](spec.md#assumptions)). If a user says "send" while recording, it appears in the transcript AND triggers the submit action. Is this the desired behavior, or should command words be filtered from the transcript?

---

*Full context in linked [spec](spec.md) and [plan](plan.md).*

# Brainstorm: Voice Relay for Remote Workspaces

**Date:** 2026-04-23 (updated 2026-04-24)
**Status:** Brainstorm
**Trigger:** lince's VoxCode integration (from brainstorm 022) demonstrated that voice-to-text relay through Zellij pipes is viable with minimal code. Workspace channels (spec 041) provide the transport abstraction needed to make this work across all workspace types.

## Problem Statement

When working in remote workspaces (container, SSH, K8s), a developer using speech-to-text on their local machine has no way to relay transcribed text to the focused agent pane. The text lands in whatever local application has focus, not the remote terminal session.

Text paste via terminal TTY works, but requires manual copy-paste of each utterance. Voice workflows need automatic relay from the local speech-to-text engine into the remote agent's input.

## How lince / VoxCode Does It

[lince](https://github.com/RisorseArtificiali/lince) integrates [VoxCode](https://github.com/RisorseArtificiali/voxcode), a standalone Python CLI that runs in its own Zellij pane:

**VoxCode architecture:**
- **Audio capture**: `sounddevice` (Python PortAudio binding), 16kHz mono, 30ms frames. Automatic resampling from native device rate if 16kHz is not supported natively.
- **Input modes**: Push-to-talk (PTT) via `termios`/`tty` cbreak mode with `select()` polling. Space key records, Tab copies to clipboard. Also supports continuous VAD mode.
- **VAD**: Custom energy-based (RMS threshold 0.015, 1.5s silence duration, 0.3s pre-roll buffer). Not Silero or WebRTC VAD.
- **Transcription**: `faster-whisper` (Python, not whisper.cpp). Lazy-loaded model. Batch per utterance (not streaming). Default: `large-v3` on CUDA.
- **Zellij integration**: Two modes:
  1. **Focus-switching**: `zellij action write-chars <text>` after focusing the target pane, then returns focus. Simple but causes visible focus flicker.
  2. **Pipe mode**: `zellij pipe --name "voxcode-text" --payload "<text>"`. Dashboard plugin receives pipe and calls `write_chars_to_pane_id()` to inject text into the agent's pane without focus switching.
- **Configuration**: TOML file (`~/.config/voxcode/config.toml`) with model, device, VAD threshold, PTT keys, multiplexer backend, and target pane settings.

**Key limitation**: VoxCode is Python-only, Linux-only (macOS unsupported for voice), and only works with local Zellij sessions. It cannot cross the local-remote boundary.

## Proposed Approach

Build an integrated voice relay into `cc-deck ws voice <workspace>` that captures audio locally, transcribes locally, and relays text to any workspace type (local, container, SSH, K8s) through PipeChannel.

### Architecture

```
cc-deck ws voice <workspace> [--mode ptt|vad|toggle] [--model base.en]
┌──────────────────────────────────────────────────────────┐
│ Bubbletea TUI                                            │
│                                                          │
│  Mode: PTT (press F8 to toggle)     Model: base.en      │
│  Audio: ▁▂▃▅▇▅▃▂▁                   Target: session #3  │
│                                                          │
│  [recording...]                                          │
│  > "add error handling to the API endpoint"              │
│    → Sent to api-backend ✓                               │
│                                                          │
│  > "enter"                                               │
│    → Submitted ↵                                         │
│                                                          │
│  Space: record | q: quit | m: mode | s: session info     │
╰──────────────────────────────────────────────────────────╯
```

### Audio Pipeline

```
Microphone
    ↓
AudioSource (malgo default, ffmpeg fallback)
    → raw PCM stream (16kHz, mono, s16le)
    → RMS level meter (for TUI display)
    ↓
Input gating (PTT toggle or VAD)
    → Audio chunks (on key-release or silence detection)
    ↓
Transcriber (whisper-server default, whisper-cli fallback, embedded opt-in)
    → Transcribed text per utterance
    ↓
Stopword engine
    → "submit" or "enter" as isolated utterance → \n
    → Otherwise → relay text
    ↓
PipeChannel.Send(ctx, "cc-deck:voice", text)
    → Transported to remote workspace via Exec()
    ↓
cc-zellij-plugin receives cc-deck:voice pipe
    → Checks session state (pause if Permission)
    → write_chars_to_pane_id(text, attended_pane)
```

## Component Design

### Audio Capture: Hybrid Approach

Every cross-platform Go audio capture library requires CGo. We use a hybrid approach selected via build tags:

| Backend | CGo | Platforms | Selection |
|---|---|---|---|
| `gen2brain/malgo` (miniaudio) | Yes | macOS (CoreAudio), Linux (PulseAudio/ALSA) | Default (CGo available) |
| `ffmpeg` subprocess | No | macOS (AVFoundation), Linux (PulseAudio/ALSA) | Fallback (`//go:build !cgo`) |

Both backends implement a common interface:

```go
type AudioSource interface {
    Start(ctx context.Context, sampleRate int) (<-chan []int16, error)
    Stop() error
    Level() float64 // current RMS level for the TUI meter
}
```

**malgo advantages**: Zero runtime dependencies, automatic backend selection, 20-100ms latency, sample-level callbacks.

**ffmpeg advantages**: No CGo, trivial cross-compilation, process isolation (crash doesn't kill CLI), stable CLI interface (14 years unchanged).

**ffmpeg capture commands**:
```bash
# macOS
ffmpeg -f avfoundation -i ":0" -f s16le -ar 16000 -ac 1 pipe:1
# Linux (PulseAudio)
ffmpeg -f pulse -i default -f s16le -ar 16000 -ac 1 pipe:1
# Linux (ALSA)
ffmpeg -f alsa -i hw:0 -f s16le -ar 16000 -ac 1 pipe:1
```

### Voice Activity Detection (VAD)

Energy-based VAD, following VoxCode's proven approach:

- **Algorithm**: RMS energy per frame (`sqrt(mean(samples^2))`)
- **Threshold**: 0.015 (configurable)
- **Pre-roll buffer**: 0.3s (captures audio before speech onset)
- **Silence duration**: 1.5s before ending an utterance
- **Implementation**: Pure Go, no external dependencies

This is sufficient for quiet development environments. More sophisticated VAD (Silero, WebRTC) can be added later if noise handling becomes a concern.

### Transcription: Three-Tier Backend

No pure-Go local Whisper inference exists. We support three backends behind a common interface:

```go
type Transcriber interface {
    Transcribe(ctx context.Context, audio []int16, sampleRate int) (string, error)
    Close() error
}
```

| Tier | Implementation | CGo | Latency | Model Loading |
|---|---|---|---|---|
| **Default** | whisper-server (HTTP) | No | ~1ms overhead | Once (server stays running) |
| **Fallback** | whisper-cli subprocess | No | ~50ms + reload per chunk | Per invocation |
| **Opt-in** | whisper.cpp Go bindings | Yes | ~0ms overhead | Once (in-process) |

**whisper-server approach** (recommended default):
- whisper.cpp ships `whisper-server` with an HTTP API
- `cc-deck ws voice --setup` installs and starts the server
- `cc-deck ws voice` auto-starts/stops the server on demand
- Model loaded once, reused across chunks
- Native C++ speed (no CGo callback overhead)
- Process isolation (whisper crash doesn't kill cc-deck)

**whisper-cli fallback**: For environments where running a server is impractical. Writes audio chunks as temp WAV files, calls `whisper-cli` per chunk. Higher latency due to model reload per invocation.

**Embedded opt-in**: Build with `whisper_embedded` build tag. Uses `github.com/ggerganov/whisper.cpp/bindings/go`. Requires CGo with C++ toolchain. Known to be 29-129% slower than native C due to CGo callback overhead.

**Selection logic**:
1. If built with embedded support (build tag), use that
2. If whisper-server is running on localhost, use HTTP
3. If whisper-cli is in PATH, use subprocess
4. Error with setup instructions

### Whisper Model Management

Models are downloaded on demand. Users who never use voice never download anything.

| Model | Full Size | Quantized (Q5_1) | Quality |
|---|---|---|---|
| tiny.en | 75 MB | **31 MB** | Acceptable for commands |
| base.en | 142 MB | **57 MB** | Good for dictation |
| small.en | 466 MB | **182 MB** | Very good |
| medium.en | 1.5 GB | 515 MB | Excellent |

- **Default model**: `base.en` (57 MB quantized, good quality-to-size ratio)
- **Storage**: `~/.cache/cc-deck/models/ggml-base.en.bin`
- **Setup**: `cc-deck ws voice --setup` checks dependencies, downloads model with progress bar
- **Override**: `cc-deck ws voice --model tiny` for smaller/larger models
- **Dependency check**: `cc-deck ws voice` verifies whisper-cli or whisper-server is installed, errors with install instructions if missing

### Transcription Flow

Whisper processes audio in chunks (up to 30-second window). The flow:

1. VAD or PTT gates audio capture
2. On utterance end (silence or key release), audio chunk is ready
3. For whisper-server: POST audio to `http://localhost:8234/inference`
4. For whisper-cli: Write temp WAV file, invoke `whisper-cli -m model.bin -f chunk.wav --no-timestamps -l en`
5. Parse transcribed text from response
6. Check for stopwords, relay or submit

Typical latency: ~3.5 seconds from end of speaking to text delivery (3s utterance + 0.5s inference on Apple Silicon with base.en).

### Stopword Engine

Utterance-level stopword detection (Option B from brainstorm discussion):

- "submit" or "enter" spoken as an **isolated utterance** (after a pause) triggers `\n` (submit)
- These words inside a longer sentence ("please submit the form") do not trigger
- Implementation: check if normalized transcription (lowercase, filler words stripped) equals exactly "submit" or "enter"

Future extensions: "clear" (delete current input), "next session" (attend next), "previous session" (attend prev).

### Input Modes

| Mode | Trigger | Best For | Works Remote |
|---|---|---|---|
| **VAD** (default) | Auto-detect speech via energy threshold | Quiet environments, hands-free | Yes |
| **Toggle PTT** | Press key to start recording, press again to stop | Noisy environments, intentional control | Yes (via Zellij keybinding) |
| **Hold PTT** | Hold key to record, release to stop | Quick commands | Local only (future) |

Default is VAD mode because it requires no focus management and works for all workspace types.

## PTT Without Focus Switching

### The Problem

PTT requires a key press, but the user should be watching their Claude Code session (where the text arrives), not the voice TUI. Switching focus between the voice TUI and the agent pane for every utterance is terrible UX.

### The Solution: Zellij Keybinding + Long-Poll Pipe

cc-deck's plugin already registers global keybindings via `reconfigure()` that work from **any pane** in the Zellij session. The same mechanism enables voice PTT:

**Step 1: Plugin registers a voice toggle keybinding:**
```rust
bind "F8" {
    MessagePluginId {id} {
        name "cc-deck:voice-toggle"
    }
}
```

**Step 2: Local voice process establishes a long-poll pipe:**
```go
// Goroutine: blocks until plugin responds
response, _ := ch.SendReceive(ctx, "cc-deck:voice-control", "listen")
// response is "toggle" when F8 is pressed
```

**Step 3: Plugin holds the pipe, responds on keypress:**
```rust
// On receiving cc-deck:voice-control pipe:
//   Store pipe_id in state, do NOT unblock

// On receiving cc-deck:voice-toggle keybinding:
//   cli_pipe_output(held_pipe_id, "toggle")
//   unblock_cli_pipe_input(held_pipe_id)
```

**Step 4: Local voice process toggles recording, re-establishes long-poll:**
```go
v.toggleRecording()
// Immediately re-send long-poll for next toggle
response, _ = ch.SendReceive(ctx, "cc-deck:voice-control", "listen")
```

This uses the existing `cli_pipe_output` API (already used by `DumpState` for returning session data to the CLI).

### Channel Architecture Changes

`PipeChannel.SendReceive()` currently returns `ErrNotSupported`. Implementation:

```go
// Local workspace: direct zellij pipe call that blocks
func (c *localPipeChannel) SendReceive(ctx context.Context, pipeName, payload string) (string, error) {
    cmd := exec.CommandContext(ctx, "zellij", "pipe", "--name", pipeName, "--", payload)
    out, err := cmd.Output() // blocks until plugin responds
    return strings.TrimSpace(string(out)), err
}

// Remote workspace: ExecOutput wrapping zellij pipe
func (c *execPipeChannel) SendReceive(ctx context.Context, pipeName, payload string) (string, error) {
    cmd := []string{"zellij", "pipe", "--name", pipeName, "--", payload}
    return c.execOutputFn(ctx, cmd) // blocks until plugin responds
}
```

The `execPipeChannel` gets a new `execOutputFn` field pointing to the workspace's `ExecOutput()` method (already on the Workspace interface).

### PTT Toggle Latency (Remote)

| Workspace Type | Toggle Latency | Acceptable? |
|---|---|---|
| Local | ~5ms | Yes |
| Podman exec | ~20-40ms | Yes |
| SSH (ControlMaster) | ~10-20ms | Yes |
| SSH (no ControlMaster) | ~100-300ms | Marginal |
| kubectl exec | ~100-400ms | Marginal |

For toggle-mode PTT (not hold-to-talk), this latency is acceptable because the user presses F8 once to start, speaks for seconds, then presses F8 again to stop.

**Note**: Hold-to-talk PTT is not viable for remote workspaces because Zellij keybindings don't have key-up events. Hold-to-talk is deferred as a future enhancement for local-only workflows using OS-level hotkeys.

## Plugin-Side Handler

The cc-zellij-plugin needs handlers for voice pipe messages:

### Text Injection (cc-deck:voice)

```rust
PipeAction::VoiceText => {
    if let Some(payload) = pipe_message.payload.as_deref() {
        // Find the attended session's pane
        if let Some(pane_id) = state.attended_pane_id() {
            let session = state.sessions.get(&pane_id);
            // Pause if session is in permission state
            if let Some(s) = session {
                if matches!(s.activity, Activity::Waiting(WaitReason::Permission)) {
                    // Buffer text, show warning via render broadcast
                    state.voice_buffer.push(payload.to_string());
                    return;
                }
            }
            write_chars_to_pane_id(payload, PaneId::Terminal(pane_id));
        }
    }
}
```

### PTT Control (cc-deck:voice-control / cc-deck:voice-toggle)

```rust
PipeAction::VoiceControl(payload) => {
    // Hold the pipe_id for later response
    if let PipeSource::Cli(ref pipe_id) = pipe_message.source {
        state.voice_control_pipe = Some(pipe_id.clone());
        // Do NOT unblock - the caller is waiting for our response
    }
}

PipeAction::VoiceToggle => {
    // Respond to the waiting voice process
    if let Some(ref pipe_id) = state.voice_control_pipe.take() {
        cli_pipe_output(pipe_id, "toggle");
        unblock_cli_pipe_input(pipe_id);
    }
}
```

### Permission State Handling

When a session enters `Waiting(Permission)` state, voice text relay is paused:
- Incoming text is buffered in `state.voice_buffer`
- The TUI shows a warning: "Voice paused: permission prompt active"
- When the session leaves permission state, buffered text is flushed

This prevents voice text from accidentally answering permission prompts (y/n/a).

## TUI Design

Uses Charm's bubbletea/lipgloss/bubbles stack, consistent with the 031-session-tui branch:
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/lipgloss`
- `github.com/charmbracelet/bubbles`

The TUI runs in alt-screen mode and displays:
- Current mode (VAD/PTT) and model name
- Audio level meter (RMS visualization)
- Recording state indicator
- Transcription history (last N utterances with relay status)
- Target session name and workspace
- Keyboard shortcuts
- Permission pause warning when applicable

## CLI Interface

```bash
# Start voice relay to a workspace (foreground TUI)
cc-deck ws voice <workspace-name>

# With options
cc-deck ws voice <workspace-name> --mode ptt    # toggle PTT mode
cc-deck ws voice <workspace-name> --mode vad    # continuous VAD mode (default)
cc-deck ws voice <workspace-name> --model tiny  # use tiny.en model
cc-deck ws voice <workspace-name> --model small # use small.en model

# Setup: check dependencies, download model
cc-deck ws voice --setup
cc-deck ws voice --setup --model small  # download a specific model

# Generic pipe command (useful beyond voice)
cc-deck ws pipe <workspace-name> --name <pipe-name> --payload "<text>"
cc-deck ws pipe <workspace-name> --name <pipe-name> --stdin  # stream from stdin
```

## Why Not a Zellij Pane (Like lince)

Lince runs VoxCode as a separate pane in the Zellij layout. We deliberately choose a different approach:

| Aspect | lince (VoxCode in pane) | cc-deck (voice as CLI command) |
|---|---|---|
| **Remote workspaces** | Only works locally (needs mic) | Works from local to any remote workspace |
| **Screen real estate** | VoxCode pane always visible | Voice runs in separate terminal |
| **Lifecycle** | Tied to Zellij session | Independent, start/stop anytime |
| **Display** | Text-mode in Zellij pane | Full bubbletea TUI with level meter |
| **User control** | Always present in layout | Opt-in when needed |

The critical difference: cc-deck supports remote workspaces. Voice capture must run on the local machine (where the microphone is), but the text needs to reach a remote Zellij session via PipeChannel. A Zellij pane-based approach only works for local sessions.

A `--pane` flag for tighter local integration (floating Zellij pane running `cc-deck ws voice`) can be added as a future enhancement.

## Dependencies

### Existing
- **Spec 041 (workspace channels)**: PipeChannel implementation (done, merged)
- **Plugin pipe handler**: Existing `parse_pipe_message()` dispatch (extend with voice actions)
- **Workspace ExecOutput**: Already on the interface (for SendReceive implementation)

### New
- **Bubbletea TUI**: `charmbracelet/bubbletea`, `lipgloss`, `bubbles` (from 031-session-tui branch)
- **Audio capture**: `gen2brain/malgo` (CGo, default) or `ffmpeg` subprocess (CGo-free fallback)
- **Whisper**: `whisper-server` or `whisper-cli` (external), optionally `whisper.cpp` Go bindings (embedded)
- **Plugin**: `write_chars_to_pane_id()` from zellij-tile 0.44 (available, not yet used)

### External (user installs separately)
- `whisper-cli` or `whisper-server` from whisper.cpp (Homebrew: `brew install whisper-cpp`)
- `ffmpeg` (only if CGo-free build, Homebrew: `brew install ffmpeg`)

## Plugin Changes Summary

| Component | Change | Size Estimate |
|---|---|---|
| `pipe_handler.rs` | Add `VoiceText`, `VoiceToggle`, `VoiceControl` pipe actions | ~15 lines |
| `controller/mod.rs` | Handle voice pipes, hold/respond control pipe, text injection | ~50 lines |
| `controller/events.rs` | Register F8 keybinding for voice toggle | ~10 lines |
| `controller/state.rs` | Track `voice_control_pipe`, `voice_buffer` | ~10 lines |

## Sequencing

1. **PipeChannel.SendReceive implementation** (Go, ~30 lines per workspace type)
2. **Plugin voice handlers** (Rust, ~85 lines total)
3. **AudioSource interface + malgo backend** (Go, audio capture)
4. **Energy-based VAD** (Go, pure, silence detection)
5. **Transcriber interface + whisper-server backend** (Go, HTTP client)
6. **Stopword engine** (Go, isolated utterance detection)
7. **Bubbletea TUI** (Go, display and input handling)
8. **`cc-deck ws voice` CLI command** (Go, orchestration)
9. **`cc-deck ws voice --setup`** (Go, dependency check + model download)
10. **whisper-cli fallback transcriber** (Go, subprocess)
11. **ffmpeg AudioSource fallback** (Go, `//go:build !cgo`)
12. **`cc-deck ws pipe` generic command** (Go, useful beyond voice)

## Prior Art

| Project | Approach | What we learned |
|---|---|---|
| lince / VoxCode | Python CLI in Zellij pane, faster-whisper, PTT via termios | Energy VAD works, pipe mode cleanest for plugin routing, PTT needs focus |
| whisper.cpp stream | C++ with SDL2, sliding window or VAD mode | 3-5s chunks give good accuracy, 30s max window |
| Discord | OS-level global hotkey for PTT | Best UX but requires platform-specific code |

## Related Brainstorms

- **022 (multi-agent-support)**: lince's VoxCode integration as inspiration
- **040 (workspace-channels)**: Design rationale for the channel abstraction
- **Spec 041 (workspace-channels)**: PipeChannel interface definition
- **043 (clipboard-bridge)**: DataChannel consumer, parallel development

## Decisions Made

| Decision | Choice | Rationale |
|---|---|---|
| TUI framework | Bubbletea (from 031 branch) | Already used in cc-deck |
| Audio capture | malgo (default) + ffmpeg (CGo-free fallback) | Hybrid via build tags |
| Audio modes | VAD (default) + toggle PTT | VAD works everywhere, PTT via Zellij keybinding |
| VAD approach | Energy-based RMS | Simple, proven (VoxCode uses same), pure Go |
| Transcription | whisper-server (default) + whisper-cli (fallback) + embedded (opt-in) | Three-tier Transcriber interface |
| Model management | On-demand download to ~/.cache/cc-deck/models/ | Zero bloat for non-voice users |
| Default model | base.en quantized (57 MB) | Good quality-to-size ratio |
| Stopwords | "submit", "enter" as isolated utterances | Triggers \n for prompt submission |
| Text injection | write_chars_to_pane_id() via plugin | Proven pattern from lince/VoxCode |
| Focus tracking | Attended session pane | Follows user attention, agent-agnostic |
| Permission state | Pause relay, buffer text, show warning | Prevents accidental permission answers |
| PTT mechanism | Zellij keybinding + long-poll pipe | Works for all workspace types without focus switching |
| Reverse channel | PipeChannel.SendReceive via cli_pipe_output | Uses existing Zellij API, no new transport |
| Zellij pane mode | Defer | Remote workspaces are the primary use case |
| Hold-to-talk | Defer | Needs OS-level hotkey, local-only |
| Voice commands | Defer | "next session", "clear" as future extensions |

## Open Questions

1. **Whisper-server lifecycle**: Should `cc-deck ws voice` auto-start whisper-server and stop it on exit? Or expect the user to manage it separately? Auto-start is better UX, but adds process management complexity.

2. **Model language**: Default to English-only models (`base.en`) for size and speed, or multilingual (`base`) for broader support? English-only is 2x faster for English text.

3. **Audio device selection**: What if the user has multiple microphones? Should we add `--device` flag or auto-detect the default input device?

4. **Voice buffer flush**: When a session leaves permission state, should buffered voice text be flushed automatically (might be stale) or discarded (user re-speaks)?

5. **Concurrent voice sessions**: Can two `cc-deck ws voice` instances target different workspaces simultaneously? The plugin would need to track multiple voice control pipes. Defer for now (single voice session).

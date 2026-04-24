# Research: Voice Relay (042)

**Date**: 2026-04-24
**Method**: Parallel agent research (4 agents exploring PipeChannel, plugin, CLI patterns, external deps)

## Decision 1: PipeChannel.SendReceive Implementation

**Decision**: Implement SendReceive by extending the existing channel_pipe.go with blocking `zellij pipe` calls. Local channel uses `cmd.Output()` directly. Remote channels use a new `execOutputFn` field on execPipeChannel pointing to each workspace's ExecOutput method.

**Rationale**: The existing `zellij pipe` command naturally blocks until the plugin responds via `cli_pipe_output` + `unblock_cli_pipe_input`. The DumpState pattern in session/save.go already demonstrates this blocking behavior. No new transport mechanism is needed.

**Alternatives considered**:
- WebSocket or TCP connection between voice process and plugin: Rejected because it would bypass the existing workspace transport layer and require new network channels.
- Polling-based approach: Rejected because it adds latency and complexity compared to the natural blocking behavior of `zellij pipe`.

**Key findings**:
- localPipeChannel.SendReceive: `exec.CommandContext(ctx, "zellij", "pipe", "--name", pipeName, "--", payload)` with `cmd.Output()`
- execPipeChannel.SendReceive: needs new `execOutputFn func(ctx, cmd) (string, error)` field, available from all workspace types' ExecOutput methods
- All workspace types already have ExecOutput: podman exec (container/compose), ssh Run (SSH), kubectl exec (k8s)
- Timeout handling via caller's context.Context

## Decision 2: Plugin Voice Handlers

**Decision**: Add three new PipeAction variants (VoiceText, VoiceControl, VoiceToggle) following the existing DumpState held-pipe pattern. Text injection via `write_chars_to_pane_id`. PTT via F8 keybinding registered in reconfigure KDL.

**Rationale**: The plugin already has the exact patterns needed. DumpState demonstrates held pipes with cli_pipe_output response. Keybinding registration via reconfigure is proven for global bindings. write_chars_to_pane_id is available in zellij-tile 0.43.1 (not yet used but available).

**Alternatives considered**:
- Separate plugin for voice handling: Rejected because it duplicates state management and the existing plugin already has attended-pane tracking.
- Using focus-switching text injection: Rejected because it causes visible flicker and interferes with the user's current pane.

**Key findings**:
- PipeAction enum at pipe_handler.rs:17-60, parse_pipe_message at :63-100
- DumpState held-pipe pattern at controller/mod.rs:363-370 (cli_pipe_output then unblock_cli_pipe_input)
- Keybinding registration at controller/events.rs:298-353 via reconfigure() with KDL format
- Permission state detection: session.rs Activity::Waiting(WaitReason::Permission) at line 24
- State fields needed: voice_control_pipe (Option<String>), voice_buffer (Vec<String>), voice_enabled (bool) in controller/state.rs
- attended_pane_id tracking at attend.rs:100,124 provides the voice target pane

## Decision 3: CLI Command Structure

**Decision**: Add `ws voice` and `ws pipe` commands in new files (internal/cmd/ws_voice.go, ws_pipe.go) following the existing newWs*Cmd pattern. Register in the "data" command group.

**Rationale**: All ws subcommands follow an identical pattern: newWs*Cmd factory function, resolveWorkspaceName for arg parsing, resolveWorkspace for workspace loading, then PipeChannel for communication. The voice command adds a Bubbletea TUI loop.

**Alternatives considered**:
- Top-level `voice` command (not under `ws`): Rejected because voice relay targets a workspace and fits the ws command hierarchy.
- Separate binary for voice: Rejected because it would miss the workspace resolution, state management, and PipeChannel infrastructure.

**Key findings**:
- Command pattern: newWs*Cmd(gf *GlobalFlags) in internal/cmd/ws.go
- Workspace resolution: resolveWorkspaceName (lines 1690-1736), resolveWorkspace (lines 1763-1784)
- PipeChannel access: `e.PipeChannel(ctx)` returns the channel for Send/SendReceive
- No Bubbletea/lipgloss/bubbles in go.mod yet (needs to be added)
- XDG cache path: `xdg.CacheHome + "/cc-deck/models/"` using internal/xdg package

## Decision 4: Audio Capture

**Decision**: Use gen2brain/malgo with CGo build tag for default audio capture, ffmpeg subprocess fallback with !cgo build tag. Both implement a common AudioSource interface.

**Rationale**: malgo provides direct access to CoreAudio (macOS) and PulseAudio/ALSA (Linux) with 20-100ms latency and sample-level callbacks. ffmpeg provides identical audio quality but as a subprocess, avoiding CGo for cross-compilation scenarios.

**Alternatives considered**:
- PortAudio Go bindings: Less maintained than malgo, similar CGo requirements.
- Pure Go audio capture: No viable library exists for direct microphone access without CGo.

**Key findings**:
- malgo config: FormatS16, Channels=1, SampleRate=16000, OnRecvFrames callback
- ffmpeg macOS: `ffmpeg -f avfoundation -i ":0" -f s16le -ac 1 -ar 16000 -`
- ffmpeg Linux: `ffmpeg -f pulse -i default -f s16le -ac 1 -ar 16000 -`
- Build tags: `//go:build cgo` and `//go:build !cgo` in separate files
- Device enumeration available via malgo context for --list-devices flag

## Decision 5: Transcription Backend

**Decision**: Default to whisper-server HTTP API at POST /inference with multipart/form-data upload. Fallback to whisper-cli subprocess with temp WAV file. Auto-start whisper-server via lifecycle management.

**Rationale**: whisper-server loads the model once and keeps it in memory, giving ~1ms overhead per request vs ~50ms+ model reload per invocation with whisper-cli. Process isolation protects cc-deck from whisper crashes.

**Alternatives considered**:
- Embedded whisper.cpp via Go CGo bindings: 29-129% slower due to CGo callback overhead, adds C++ build dependency. Deferred as opt-in build tag.
- Cloud transcription API: Rejected per spec requirement FR-003 (no external services).

**Key findings**:
- whisper-server endpoint: POST /inference with multipart file upload
- Also supports OpenAI-compatible endpoint at /v1/audio/transcriptions
- whisper-cli requires 16-bit WAV input files (need temp file creation)
- Default server port: configurable, brainstorm uses 8234

## Decision 6: Model Management

**Decision**: Download ggml models from Hugging Face to `~/.cache/cc-deck/models/`. Default model: ggml-base.en.bin (142 MB). Use HTTP GET with progress tracking.

**Rationale**: Hugging Face hosts the official ggml model files. The download-ggml-model.sh script demonstrates the URL pattern. Quantized models (Q5_1) offer 50-60% size reduction with minimal quality loss.

**Alternatives considered**:
- Bundling models in the binary: Rejected per spec (SC-007, <100KB binary impact).
- Mirror to a cc-deck CDN: Unnecessary complexity, Hugging Face is reliable.

**Key findings**:
- URL: `https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-{model}.bin`
- Quantized URL: `https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-{model}-q5_1.bin`
- Cache dir: `~/.cache/cc-deck/models/` (XDG cache, using internal/xdg)
- Model integrity: check file size against known sizes, re-download if corrupted

## Decision 7: Voice Activity Detection

**Decision**: Pure Go energy-based VAD using RMS threshold per frame. Parameters: threshold 0.015, pre-roll 0.3s, silence duration 1.5s.

**Rationale**: Simple RMS energy detection is proven (VoxCode uses the same approach) and sufficient for quiet development environments. No external dependencies needed. More sophisticated VAD (WebRTC, Silero) can be added later if noise becomes a concern.

**Alternatives considered**:
- WebRTC VAD Go port (baabaaox/go-webrtcvad): More robust but adds dependency. Consider for future enhancement.
- Silero VAD: Requires ONNX runtime, too heavy for this use case.

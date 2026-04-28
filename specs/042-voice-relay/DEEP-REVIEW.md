# Deep Review Report: 042-voice-relay

**Branch**: `042-voice-relay`
**Date**: 2026-04-24
**Reviewer**: Claude Opus 4.6 (5-agent parallel review)
**Scope**: All Go and Rust changes vs `main` (30 source files, ~4200 lines added)

---

## Summary

The voice relay MVP implements a solid audio-to-text-to-pane pipeline spanning Go CLI and Rust WASM plugin code. The architecture is clean: well-defined interfaces (AudioSource, Transcriber, PipeSender), proper build-tag separation for CGo/non-CGo, and correct plugin-side WASM gating. PipeChannel.SendReceive and the voice pipe handlers work as designed.

**Critical issues found**: 3
**Important issues found**: 8
**Minor issues found**: 7

The critical issues are: (1) audio frame drops in the malgo callback, (2) voice buffer grows without bound during prolonged permission prompts, and (3) missing relay orchestrator tests.

---

## Critical Findings

### C1. Audio frame drops in malgo callback (Performance/Reliability)

**File**: `cc-deck/internal/voice/audio_malgo.go:73-76`

The malgo audio callback uses a non-blocking channel send:

```go
select {
case out <- samples:
default:
}
```

When the downstream consumer (VAD) is slower than the audio callback (which fires every ~20ms at 16kHz), frames are silently dropped. This causes clipped utterances, missed speech onset, and unreliable transcription. The `default` branch discards audio without any notification.

**Impact**: Corrupted transcription output. Users dictate text that gets partially captured, producing garbled or incomplete prompts in the agent pane.

**Fix**: Increase channel buffer from 16 to 64 frames (covers ~1.3 seconds of buffering at 50 fps). Add a dropped-frame counter exposed via a method so the TUI can warn the user. Consider a ring buffer if drops persist:

```go
select {
case out <- samples:
default:
    atomic.AddInt64(&s.droppedFrames, 1)
}
```

### C2. Unbounded voice buffer during permission prompts (Reliability/Security)

**File**: `cc-zellij-plugin/src/controller/mod.rs:315`

When the attended session is in a permission prompt, voice text is pushed to `voice_buffer` without any size limit:

```rust
self.state.voice_buffer.push(text);
```

If voice relay runs for an extended period during a permission prompt (developer steps away with microphone active), the buffer grows indefinitely. The spec says buffered text is discarded when the permission resolves (FR-016), but the discard logic is not implemented yet (T036 is not done). Even when implemented, there is no cap on buffer growth in the interim.

**Impact**: Memory exhaustion in the plugin process. In a WASM environment, this could crash the Zellij plugin.

**Fix**: Add a maximum buffer size constant (e.g., 100 entries). Drop oldest entries when exceeded:

```rust
const MAX_VOICE_BUFFER: usize = 100;
// In VoiceText handler:
if self.state.voice_buffer.len() >= MAX_VOICE_BUFFER {
    self.state.voice_buffer.remove(0);
}
self.state.voice_buffer.push(text);
```

Also: T036 (voice buffer discard on permission resolution) is marked as not done. This is the primary mitigation. Without it, old text accumulates even after the developer handles the permission prompt.

### C3. No tests for relay orchestrator (Testing)

**File**: `cc-deck/internal/voice/relay.go` (entire file, 201 lines)

The VoiceRelay orchestrator is the core pipeline logic connecting audio -> VAD -> transcription -> stopword -> delivery. It has zero tests. This is the most important component: it manages concurrency (two goroutines), lifecycle (Start/Stop), event emission, and the transcription-to-delivery path including command word detection.

**Impact**: Any regression in the pipeline (wrong event ordering, goroutine leak, missed delivery, incorrect stopword handling in context) goes undetected.

**Fix**: Add tests using mock implementations of AudioSource, Transcriber, and PipeSender:
- Test that spoken text flows through to PipeSender.Send
- Test that "submit" produces "\n" payload
- Test that empty transcription results are discarded
- Test that Stop() waits for goroutines and closes the events channel
- Test that transcription errors produce error events

---

## Important Findings

### I1. Server port hardcoded with no conflict detection

**File**: `cc-deck/internal/cmd/ws_voice.go:55` and `cc-deck/internal/voice/server.go:48-51`

The whisper-server binds to port 8234 by default. If another instance is already running (or any other process uses this port), the server will fail to start with an opaque error. There is no port-in-use check or automatic port selection.

**Fix**: Before starting whisper-server, check if the port is already in use. If a healthy whisper-server is already running on that port, reuse it instead of starting a new one. Add this check to `WhisperServer.Start()`:

```go
if s.Healthy(ctx) {
    // Server already running on this port, reuse it
    return nil
}
```

### I2. Whisper-server context detached from caller

**File**: `cc-deck/internal/voice/server.go:46`

The server creates a `context.WithCancel(context.Background())` instead of using the passed-in `ctx`. This means the server process survives caller cancellation, the caller's deadline, and signal handling:

```go
cmdCtx, cancel := context.WithCancel(context.Background())
```

**Impact**: If the voice command is interrupted (Ctrl+C), the whisper-server process may become orphaned.

**Fix**: Derive from the caller's context:

```go
cmdCtx, cancel := context.WithCancel(ctx)
```

### I3. Response body size unlimited in HTTP transcriber

**File**: `cc-deck/internal/voice/transcriber_http.go:63-68`

`io.ReadAll(resp.Body)` reads the entire response body without size limits. If the whisper-server returns an unexpectedly large response (bug, misconfiguration, or a different service on the port), this allocates unbounded memory.

**Fix**: Use `io.LimitReader`:

```go
respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
```

### I4. No model path sanitization in setup

**File**: `cc-deck/internal/voice/setup.go:58-60`

`ModelPath()` uses `filepath.Base(name)` to sanitize unknown model names, but the `name` comes from user input (the `--model` flag). While `filepath.Base` prevents directory traversal in the filename, the download URLs are hardcoded in the `models` map, so this is low risk. However, `modelFileName()` in `ws_voice.go:130` does no sanitization at all:

```go
func modelFileName(name string) string {
    return fmt.Sprintf("ggml-%s.bin", name)
}
```

**Fix**: Use `voice.ModelPath()` consistently instead of constructing paths manually in `ws_voice.go`. The function already exists and handles sanitization.

### I5. TUI mode toggle has no effect on audio pipeline

**File**: `cc-deck/internal/tui/voice/update.go:19-23`

The 'm' key toggles `m.mode` between "vad" and "ptt" in the TUI model, but this does not propagate to the VoiceRelay orchestrator. The relay continues operating in whatever mode it was started with. The TUI displays a mode change that has no actual effect.

**Fix**: Either remove the mode toggle key until PTT is implemented (Phase 5), or document it as non-functional. The current behavior is misleading.

### I6. Missing context cancellation check in download

**File**: `cc-deck/internal/voice/setup.go:143`

`downloadModel()` uses `client.Get(info.URL)` without context, so the download cannot be cancelled by the user (Ctrl+C). For large models (medium is 1.5 GB), this means the user must wait for the download to complete or kill the process.

**Fix**: Use `http.NewRequestWithContext`:

```go
req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.URL, nil)
resp, err := client.Do(req)
```

This requires threading context through `RunSetup()` and `downloadModel()`.

### I7. Event channel drop silently discards errors

**File**: `cc-deck/internal/voice/relay.go:195-199`

The `sendEvent` method drops events if the channel is full:

```go
func (r *VoiceRelay) sendEvent(ev RelayEvent) {
    select {
    case r.events <- ev:
    default:
    }
}
```

For level events (sent 20x/second), this is acceptable. For error and delivery events, dropping them means the TUI never learns about failures.

**Fix**: Use a blocking send for error and delivery events, or increase the channel buffer for these event types. At minimum, log dropped error events.

### I8. No test for voice pipe message parsing edge cases

**File**: `cc-zellij-plugin/src/pipe_handler.rs:215-227`

The voice command test (`test_parse_voice_commands`) covers the happy path but does not test:
- VoiceText with very long payload
- VoiceText with special characters (newlines, null bytes)
- VoiceToggle when voice is not enabled

**Fix**: Add edge case tests for payloads containing control characters and very long strings.

---

## Minor Findings

### M1. Duplicate RMS calculation

**Files**: `audio_malgo.go:65-71`, `audio_ffmpeg.go:74-79`, `vad.go:98-108`

RMS level calculation is implemented three times: once in each audio backend and once in the VAD. The audio backends calculate RMS for the TUI level display; the VAD calculates it for speech detection. These are semantically different uses, but the code is identical.

**Fix**: Consider extracting an `rmsLevel([]int16) float64` function (already exists in `vad.go`). The audio backends could use it. Not urgent.

### M2. TUI does not show workspace type

The TUI displays the workspace name but not the type (local, container, SSH, etc.). For debugging transport issues, knowing the workspace type would be helpful.

### M3. No --dry-run or --test mode

There is no way to test the voice pipeline without a running workspace. A `--dry-run` flag that captures, transcribes, and displays text without sending it via PipeChannel would aid development and debugging.

### M4. VAD threshold not configurable via CLI

The VAD threshold (0.015) and silence duration (1.5s) are hardcoded in `DefaultVADConfig()`. These should be tunable via CLI flags for different microphone sensitivities and speaking styles.

### M5. ListDevices returns nil for ffmpeg backend

**File**: `cc-deck/internal/voice/audio_ffmpeg.go:117-119`

The ffmpeg backend returns `nil, nil` from `ListDevices()`, which means `--list-devices` silently shows "No audio input devices found" when building without CGo. The function should return an error explaining that device enumeration requires CGo.

### M6. pcmToWAV ignores binary.Write errors

**File**: `cc-deck/internal/voice/transcriber_http.go:88-104`

All `binary.Write` calls in `pcmToWAV` discard errors with `_ = binary.Write(...)`. Writing to a `bytes.Buffer` does not fail in practice, but the pattern is inconsistent with the error handling elsewhere.

### M7. Stopword test file is incomplete per task T031

**File**: `cc-deck/internal/voice/stopword_test.go`

T031 is marked as not done in tasks.md, but the test file already exists with comprehensive test cases (14 scenarios). The task should be marked as done, or the test file should be reviewed against the T031 requirements to confirm coverage.

---

## Spec Compliance Summary

| Requirement | Status | Notes |
|-------------|--------|-------|
| FR-001 Audio capture 16kHz mono | PASS | Both malgo and ffmpeg backends implement this |
| FR-002 VAD segmentation | PASS | Energy-based VAD with configurable thresholds |
| FR-003 Local transcription | PASS | HTTP and CLI backends, no external service calls |
| FR-004 PipeChannel relay | PASS | All workspace types updated with SendReceive |
| FR-005 Text injection via plugin | PASS | write_chars_to_pane_id with WASM gating |
| FR-006 Stopword detection | PASS | Filler stripping + command word matching |
| FR-007 Permission prompt pause | PARTIAL | Buffering works, but buffer discard (T036) not implemented |
| FR-008 PTT mode (F8) | PARTIAL | Keybinding registered, plugin handler exists, relay-side PTT not done (T033) |
| FR-014 Generic pipe command | PASS | `cc-deck ws pipe` command works |
| FR-017 CGo-free fallback | PASS | Build tags correctly separate malgo/ffmpeg |
| FR-018 Backend lifecycle | PASS | Auto-start/restart with 3 retries |
| NFR-004 Audio stays local | PASS | Only text transmitted via PipeChannel |
| NFR-005 No persistent logging | PASS | No file logging of transcriptions |

---

## Recommended Fix Priority

1. **C2**: Cap voice buffer size (5 min, Rust)
2. **C1**: Increase audio channel buffer + drop counter (15 min, Go)
3. **I2**: Fix server context derivation (5 min, Go)
4. **I5**: Remove misleading mode toggle (5 min, Go)
5. **I3**: Limit HTTP response body (5 min, Go)
6. **I1**: Add server port reuse detection (15 min, Go)
7. **I4**: Use voice.ModelPath consistently (5 min, Go)
8. **C3**: Add relay orchestrator tests (30-45 min, Go)
9. **I7**: Don't drop error/delivery events (10 min, Go)
10. **I6**: Add context to download (10 min, Go)

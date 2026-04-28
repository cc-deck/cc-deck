# Contract: Voice Pipeline Interfaces

## AudioSource Interface

```go
type AudioSource interface {
    // Start begins audio capture. Returns a channel that receives PCM frames
    // (signed 16-bit, mono, at the specified sample rate).
    // The channel is closed when Stop() is called or an error occurs.
    // Callers MUST call Stop() to release the device.
    Start(ctx context.Context, sampleRate int) (<-chan []int16, error)

    // Stop halts audio capture and releases the device.
    // Safe to call multiple times.
    Stop() error

    // Level returns the current RMS audio level (0.0 to 1.0)
    // for TUI visualization. Returns 0.0 if not capturing.
    Level() float64

    // ListDevices enumerates available audio input devices.
    ListDevices() ([]DeviceInfo, error)
}
```

**Behavioral Requirements**:
- Start MUST return an error if the device is already in use or inaccessible
- The PCM channel MUST deliver frames in real-time (no buffering beyond the device's native buffer)
- Stop MUST be safe to call even if Start was never called
- Level MUST be thread-safe (called from TUI goroutine while capture runs)
- ListDevices MUST work before Start is called

## Transcriber Interface

```go
type Transcriber interface {
    // Transcribe converts PCM audio samples to text.
    // Audio must be mono, signed 16-bit, at the specified sample rate.
    // Returns empty string for silence/noise (not an error).
    Transcribe(ctx context.Context, audio []int16, sampleRate int) (string, error)

    // Close releases resources. For HTTP transcriber, stops the server
    // if it was auto-started. Safe to call multiple times.
    Close() error
}
```

**Behavioral Requirements**:
- Transcribe MUST NOT send audio to external services (all processing local)
- Transcribe MUST respect context cancellation
- Empty/whitespace-only results are valid (represent silence/noise), not errors
- Close MUST stop any auto-started server process
- Transcribe MUST handle audio up to 30 seconds long (Whisper's maximum window)

## PipeChannel.SendReceive Contract

```go
// SendReceive sends a payload to a named pipe and blocks until the plugin
// responds. The plugin controls when the response is sent, enabling
// long-poll patterns where the caller waits for an asynchronous event.
//
// Returns the plugin's response as a trimmed string.
// Returns an error if the pipe name is empty, the workspace is unreachable,
// or the context is cancelled.
SendReceive(ctx context.Context, pipeName string, payload string) (string, error)
```

**Behavioral Requirements**:
- MUST block until the plugin calls cli_pipe_output + unblock_cli_pipe_input
- MUST respect context cancellation (return error on timeout/cancel)
- Empty payload is valid (used by the "listen" command in PTT long-poll)
- Empty response is valid (plugin may acknowledge without data)
- MUST use the same transport as Send (local zellij pipe, or workspace exec)

## Plugin Voice Handler Contract

### VoiceText (cc-deck:voice)

**Input**: Pipe payload containing transcribed text
**Behavior**:
1. Find the attended pane (from state.last_attended_pane_id)
2. If no attended pane, discard text silently
3. If attended session is in Waiting(Permission) state, buffer text in voice_buffer
4. Otherwise, call write_chars_to_pane_id(pane_id, text)
5. Unblock the CLI pipe immediately

### VoiceControl (cc-deck:voice-control)

**Input**: Pipe payload "listen"
**Behavior**:
1. Store the CLI pipe_id in state.voice_control_pipe
2. Set state.voice_enabled = true
3. DO NOT unblock the pipe (caller blocks until F8 is pressed)

### VoiceToggle (cc-deck:voice-toggle)

**Input**: Message from F8 keybinding (no payload)
**Behavior**:
1. If voice_control_pipe is Some, respond with "toggle" via cli_pipe_output
2. Unblock the held pipe via unblock_cli_pipe_input
3. Clear voice_control_pipe to None
4. If voice_control_pipe is None, ignore (F8 pressed without active voice relay)

### Permission State Transition

**Behavior**:
- When session enters Waiting(Permission): voice text is buffered, not injected
- When session leaves Waiting(Permission): voice_buffer is cleared (discarded, not flushed)
- The TUI is NOT notified via the pipe (the voice relay detects paused state via the response pattern or periodic state query)

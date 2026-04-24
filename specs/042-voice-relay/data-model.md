# Data Model: Voice Relay (042)

## Entities

### AudioSource (Go interface)

The audio capture backend that provides PCM audio samples from the local microphone.

| Field/Method | Type | Description |
|-------------|------|-------------|
| Start(ctx, sampleRate) | (<-chan []int16, error) | Begin capturing audio at given sample rate, return channel of PCM frames |
| Stop() | error | Stop capturing and release device |
| Level() | float64 | Current RMS level for TUI meter (0.0 to 1.0) |
| ListDevices() | ([]DeviceInfo, error) | Enumerate available input devices |

**Implementations**: malgoSource (CGo, //go:build cgo), ffmpegSource (subprocess, //go:build !cgo)

**State**: Started or Stopped. Start transitions to Started, Stop transitions to Stopped. Cannot Start when already Started.

### DeviceInfo (Go struct)

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Platform-specific device identifier |
| Name | string | Human-readable device name |
| IsDefault | bool | Whether this is the OS default input device |

### Transcriber (Go interface)

The speech recognition backend that converts audio chunks into text.

| Field/Method | Type | Description |
|-------------|------|-------------|
| Transcribe(ctx, audio, sampleRate) | (string, error) | Convert audio samples to text |
| Close() | error | Release resources (stop server if auto-started) |

**Implementations**: httpTranscriber (whisper-server), cliTranscriber (whisper-cli subprocess)

### VADConfig (Go struct)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| Threshold | float64 | 0.015 | RMS energy threshold for speech detection |
| PreRollDuration | time.Duration | 300ms | Audio buffer before speech onset |
| SilenceDuration | time.Duration | 1500ms | Silence duration to end an utterance |
| MaxUtteranceDuration | time.Duration | 30s | Maximum utterance length before forced split |

### Utterance (Go struct)

| Field | Type | Description |
|-------|------|-------------|
| Audio | []int16 | PCM audio samples for this utterance |
| SampleRate | int | Sample rate (16000) |
| StartedAt | time.Time | When speech was detected |
| EndedAt | time.Time | When silence was detected |

### TranscriptionResult (Go struct)

| Field | Type | Description |
|-------|------|-------------|
| Text | string | Transcribed text |
| Latency | time.Duration | Time from utterance end to transcription complete |
| IsCommand | bool | True if text matched a command word after filler stripping |
| CommandAction | string | "submit" or "enter" if IsCommand is true |

### VoiceRelayConfig (Go struct)

| Field | Type | Description |
|-------|------|-------------|
| WorkspaceName | string | Target workspace name |
| Mode | string | "vad" or "ptt" |
| ModelName | string | Whisper model name (e.g., "base.en") |
| DeviceID | string | Audio device ID (empty = default) |
| Verbose | bool | Show diagnostic details in TUI |
| ServerPort | int | Whisper server port (default: 8234) |

### ModelInfo (Go struct)

| Field | Type | Description |
|-------|------|-------------|
| Name | string | Model name (e.g., "base.en", "tiny.en") |
| FileName | string | File name on disk (e.g., "ggml-base.en.bin") |
| Size | int64 | Expected file size in bytes |
| URL | string | Download URL (Hugging Face) |
| Downloaded | bool | Whether model exists in cache |
| Valid | bool | Whether downloaded model passes integrity check |

### VoiceControl (Rust, plugin-side state)

| Field | Type | Description |
|-------|------|-------------|
| voice_control_pipe | Option<String> | Held CLI pipe ID for PTT long-poll |
| voice_buffer | Vec<String> | Text buffered during permission prompts |
| voice_enabled | bool | Whether voice relay is currently connected |

### PipeAction (Rust enum, new variants)

| Variant | Pipe Name | Payload | Description |
|---------|-----------|---------|-------------|
| VoiceText(String) | cc-deck:voice | Transcribed text | Inject text into attended pane |
| VoiceControl | cc-deck:voice-control | "listen" | Hold pipe for PTT long-poll |
| VoiceToggle | cc-deck:voice-toggle | (none) | F8 keybinding pressed, respond to held pipe |

## Relationships

```
AudioSource --[produces]--> []int16 PCM frames
     |
     v
VAD --[segments]--> Utterance
     |
     v
Transcriber --[transcribes]--> TranscriptionResult
     |
     v
StopwordEngine --[filters]--> text or command
     |
     v
PipeChannel.Send --[transports]--> Plugin (VoiceText action)
     |
     v
VoiceControl (plugin) --[injects]--> attended pane via write_chars_to_pane_id
```

## State Transitions

### Voice Relay Lifecycle

```
Idle --> Starting (setup check, server start)
Starting --> Listening (VAD mode) | WaitingForKey (PTT mode)
Listening --> Capturing (speech detected)
WaitingForKey --> Capturing (F8 pressed)
Capturing --> Transcribing (silence detected or F8 pressed)
Transcribing --> Delivering (text ready)
Delivering --> Listening | WaitingForKey (text sent or buffered)
Any --> Stopping (quit command)
Stopping --> Idle
```

### Permission Pause State

```
Normal --> Paused (session enters Waiting(Permission))
Paused --> Normal (session leaves Permission state)
   On transition Paused --> Normal: discard voice_buffer
```

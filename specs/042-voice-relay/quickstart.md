# Quickstart: Voice Relay Development

## Prerequisites

- Go 1.25+ (from go.mod)
- Rust stable with wasm32-wasip1 target
- whisper-server or whisper-cli installed (`brew install whisper-cpp`)
- Working microphone

## Build

```bash
# From project root (never from subdirectories)
make install    # Build both Go CLI and Rust plugin
make test       # Run all tests
make lint       # Run linters
```

## Test Voice Relay

```bash
# 1. Setup: check dependencies and download model
cc-deck ws voice --setup

# 2. Start a workspace
cc-deck ws start my-workspace

# 3. Start voice relay
cc-deck ws voice my-workspace

# 4. Speak into microphone - text appears in attended agent pane
# 5. Say "submit" to press Enter
# 6. Press 'q' to quit
```

## Test Generic Pipe

```bash
# Send text to a named pipe in any workspace
cc-deck ws pipe my-workspace --name cc-deck:voice --payload "hello world"
```

## Development Workflow

### Go CLI changes (audio, transcription, TUI, commands)

```bash
# Edit files in cc-deck/internal/voice/ or cc-deck/internal/cmd/
make test       # Run tests
make install    # Build and install
cc-deck ws voice --setup  # Verify setup
cc-deck ws voice <workspace>  # Test end-to-end
```

### Rust plugin changes (pipe handlers, keybindings, state)

```bash
# Edit files in cc-zellij-plugin/src/
make install    # Build WASM and install plugin
zellij kill-all-sessions -y 2>/dev/null
zellij --layout cc-deck  # Start fresh session with new plugin
```

### Testing PTT keybinding

```bash
# Start voice relay in PTT mode
cc-deck ws voice <workspace> --mode ptt
# Press F8 from ANY pane in Zellij to toggle recording
# No focus switching needed
```

## Key File Locations

| Component | Path |
|-----------|------|
| Voice commands | cc-deck/internal/cmd/ws_voice.go, ws_pipe.go |
| Audio capture | cc-deck/internal/voice/audio*.go |
| VAD | cc-deck/internal/voice/vad.go |
| Transcription | cc-deck/internal/voice/transcriber*.go |
| Stopwords | cc-deck/internal/voice/stopword.go |
| Voice relay orchestrator | cc-deck/internal/voice/relay.go |
| TUI | cc-deck/internal/tui/voice/ |
| Plugin pipe handler | cc-zellij-plugin/src/pipe_handler.rs |
| Plugin controller | cc-zellij-plugin/src/controller/mod.rs |
| Plugin state | cc-zellij-plugin/src/controller/state.rs |
| Plugin keybindings | cc-zellij-plugin/src/controller/events.rs |
| Whisper models | ~/.cache/cc-deck/models/ |

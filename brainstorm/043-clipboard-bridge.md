# Brainstorm: Remote Clipboard Bridge for Image Paste

**Date:** 2026-03-03 (updated 2026-04-23)
**Status:** Brainstorm
**Trigger:** Claude Code supports Ctrl+V for image paste locally, but remote workspaces have no clipboard access
**Updated:** Refactored to use DataChannel from workspace channels (spec 041) instead of custom transport plumbing

## Problem

Claude Code supports pasting images from the clipboard via Ctrl+V. Locally, it calls platform-specific tools (`pbpaste`/CoreGraphics on macOS, `xclip`/`wl-paste` on Linux) as subprocess calls to read image data from the system clipboard.

When Claude Code runs in a remote workspace (container, SSH, K8s), these clipboard tools find nothing because the workspace has no display server and no connection to the user's local clipboard. The user loses the ability to paste screenshots and images.

Text paste (CMD-V on macOS / Ctrl+Shift+V on Linux) still works because it goes through the terminal's normal paste handling, which flows through the exec TTY connection.

## How Claude Code Reads Clipboard (Linux)

On Linux (which runs in the workspace), Claude Code:

1. Intercepts Ctrl+V keystroke (also Alt+V since v1.0.93)
2. Calls `xclip -selection clipboard -t TARGETS -o` to list available MIME types
3. Greps for `image/*` types
4. Calls `xclip -selection clipboard -t "image/png" -o` to extract the image binary
5. Sends the image to the Claude API as a multimodal message

This means a custom `xclip` wrapper placed higher in PATH will transparently intercept these calls. Claude Code does not need modification.

## Two-Part Design

### Part 1: Local Clipboard Watcher + DataChannel Push

`cc-deck attach` (or a companion process) watches the local clipboard for image content changes and pushes image data to the remote workspace via DataChannel.

**Clipboard watching strategy (robustness first):**
- **macOS**: Poll `NSPasteboard.changeCount` every 300ms (just a counter check, near-zero cost). When the count changes, read clipboard with `osascript` or CoreGraphics.
- **Linux X11**: Poll with `xclip -selection clipboard -t TARGETS -o` every 500ms. Detect image types.
- **Linux Wayland**: Use `wl-paste --watch` for event-driven notification. Falls back to polling if not available.
- **Fallback**: Manual trigger via `cc-deck ws clipboard push <workspace>` command.

**Transport via DataChannel (spec 041):**

```go
func pushClipboardImage(ctx context.Context, ws Workspace, imageData []byte) error {
    ch, err := ws.DataChannel(ctx)
    if err != nil {
        return err
    }
    return ch.PushBytes(ctx, imageData, "/tmp/.cc-clipboard/latest.png")
}
```

This replaces the custom kubectl exec / podman cp / rsync plumbing from the original design (brainstorm 05). The DataChannel handles transport differences per workspace type internally. No custom background goroutine for exec management needed, no custom reconnection logic.

**Data format in workspace:**
```
/tmp/.cc-clipboard/
  latest.png          # Image data (PNG format, converted if needed)
  metadata.json       # {"timestamp": "...", "mime": "image/png", "size": 12345, "hash": "sha256:..."}
```

**Deduplication:** Skip push if image data hash matches the previously pushed hash.

### Part 2: Clipboard Shim (in base container image / remote workspace)

Custom `xclip` wrapper installed at a PATH-priority location in the workspace.

**`/usr/local/bin/xclip`** (wrapper):
```bash
#!/bin/sh
# cc-deck clipboard bridge shim for xclip
# Intercepts clipboard read requests and serves from bridge staging area

CLIPBOARD_DIR="/tmp/.cc-clipboard"
REAL_XCLIP="/usr/bin/xclip"

# Detect if this is a clipboard read with image target
case "$*" in
  *"-selection clipboard"*"-t TARGETS"*"-o"*)
    # List available targets
    if [ -f "$CLIPBOARD_DIR/latest.png" ]; then
      echo "image/png"
      echo "TARGETS"
    fi
    # Also pass through to real xclip for text targets
    [ -x "$REAL_XCLIP" ] && "$REAL_XCLIP" "$@" 2>/dev/null
    exit 0
    ;;
  *"-selection clipboard"*"-t"*"image/"*"-o"*)
    # Read image from bridge
    if [ -f "$CLIPBOARD_DIR/latest.png" ]; then
      cat "$CLIPBOARD_DIR/latest.png"
      exit 0
    fi
    exit 1
    ;;
  *)
    # Pass through all other calls to real xclip
    [ -x "$REAL_XCLIP" ] && exec "$REAL_XCLIP" "$@"
    exit 1
    ;;
esac
```

**Key design decisions:**
- Only intercepts clipboard READ operations for images
- Passes through all other operations (write, text) to real xclip
- Fails gracefully if no bridge data is available
- No dependencies beyond shell

## Architecture

```
Local Machine                          Remote Workspace
~~~~~~~~~~~~~                          ~~~~~~~~~~~~~~~~

User takes screenshot
        |
        v
System Clipboard (image)
        |
        v
Clipboard watcher
  - Change detection (hash-based)
        |
        v
DataChannel.PushBytes()          ----> /tmp/.cc-clipboard/
  (spec 041 handles transport)           latest.png
                                         metadata.json
                                              ^
                                        Custom xclip shim
                                        (/usr/local/bin/xclip)
                                              ^
                                        Claude Code (Ctrl+V)
                                        (calls xclip, gets image)
```

## Security

### Transport
- DataChannel uses the workspace's native transport (SSH, kubectl exec, podman exec), all of which are encrypted in transit
- Optional additional encryption at the consumer level (AES-256-GCM) if needed for sensitive clipboard content
- This is a consumer-level concern, not a channel-level concern (per spec 041 design decision)

### Data at Rest
- Staging files in `/tmp/.cc-clipboard/` have `0600` permissions (owner-only)
- The staging directory is created with `0700` permissions
- Files are rotated (only latest image kept, previous overwritten)
- Workspace is single-user by design
- Optional: cleanup timer deletes stale files older than 5 minutes

### Threat Model
- **Intercepted transport**: Mitigated by workspace transport encryption (TLS, SSH)
- **Workspace filesystem access**: Mitigated by file permissions + single-user design
- **Clipboard exfiltration**: The bridge only pushes TO the workspace, never reads FROM it. Unidirectional by design.
- **Malicious clipboard content**: Same risk as local clipboard usage, not amplified by the bridge

## Plugin Integration (Optional)

The cc-zellij-plugin could enhance the experience:
- Status indicator showing clipboard bridge connection state
- Notification when new image data arrives ("Image received")
- Alt+V keybinding to trigger manual clipboard sync

The plugin enhances the experience but is NOT required. The shim + DataChannel provide full transparency without it.

## Implementation Plan

### Phase 1: Core Bridge (MVP)
1. Custom `xclip` shim script in the base container image
2. Clipboard watcher with polling-based detection
3. DataChannel push (requires spec 041 Phase 2)
4. Basic file staging without encryption

### Phase 2: Robustness
1. Event-driven clipboard detection (macOS changeCount, wl-paste --watch)
2. Hash-based deduplication
3. Graceful degradation when workspace is unreachable
4. `cc-deck ws clipboard push` manual command for fallback

### Phase 3: Plugin Integration (Stretch)
1. Zellij plugin: clipboard bridge status indicator
2. Zellij plugin: image arrival notification
3. Support for multiple image formats (JPEG, WebP auto-convert to PNG)

## Dependencies

- **Spec 041 (workspace channels)**: DataChannel implementation (Phase 2), specifically `PushBytes()` method
- **Base image**: xclip shim must be included in `quay.io/cc-deck/cc-deck-base`
- **Build command**: `cc-deck.build` should install the shim for SSH targets via Ansible too

## Related Brainstorms

- **040 (workspace-channels)**: Design rationale for the channel abstraction
- **042 (voice-relay)**: Uses PipeChannel, same pattern of local-to-remote bridging
- **Spec 041 (workspace-channels)**: DataChannel interface definition
- **Attic/05 (clipboard-bridge)**: Original brainstorm with custom transport design (superseded)

## Prior Art

| Tool | Approach | Limitation |
|------|----------|------------|
| Kitty clipboard script | Save clipboard image to file, send path via `kitty @ send-text` | Kitty-specific, not remote |
| VS Code terminal-paste-image | VS Code extension intercepts paste, saves to project | VS Code-specific |
| Claude Code clipboard plugin | `/clipboard:paste` slash command | Requires user to type command |
| RDP/VNC | Virtual clipboard channel in protocol | Different transport entirely |
| OSC 52 | Terminal escape sequence for clipboard | Text-only, size limits |

## Key Differentiator

This solution provides **transparent remote clipboard bridging for a terminal-based AI assistant**. No other tool in the Claude Code ecosystem or the broader remote dev environment space (DevPod, Dev Spaces, Codespaces) handles image clipboard sharing for terminal workflows. The xclip shim approach means Claude Code works exactly the same locally and remotely.

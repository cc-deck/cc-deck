# Brainstorm: Remote Clipboard Bridge for Image Paste

**Date**: 2026-03-03
**Status**: active
**Feature**: cc-deck (Kubernetes CLI)
**Affects**: cc-deck connect workflow, base container image, cc-zellij-plugin

## Problem

Claude Code supports pasting images from the clipboard via Ctrl+V. Locally, it calls platform-specific tools (`pbpaste`/CoreGraphics on macOS, `xclip`/`wl-paste` on Linux) as subprocess calls to read image data from the system clipboard.

When Claude Code runs in a remote Pod (via `cc-deck connect`), these clipboard tools find nothing because the Pod has no display server and no connection to the user's local clipboard. The user loses the ability to paste screenshots and images into Claude Code.

Text paste (CMD-V on macOS / Ctrl+Shift+V on Linux) still works because it goes through the terminal's normal paste handling, which flows through the exec TTY connection.

## How Claude Code Reads Clipboard (Confirmed)

On Linux (which runs in the Pod), Claude Code:

1. Intercepts Ctrl+V keystroke (also Alt+V since v1.0.93)
2. Calls `xclip -selection clipboard -t TARGETS -o` to list available MIME types
3. Greps for `image/*` types
4. Calls `xclip -selection clipboard -t "image/png" -o` to extract the image binary
5. Sends the image to the Claude API as a multimodal message

This means a custom `xclip` wrapper placed higher in PATH will transparently intercept these calls. Claude Code does not need modification.

## Solution: Three-Part Design

### Part 1: Local Clipboard Bridge (cc-deck enhancement)

`cc-deck connect` is enhanced to run a background clipboard bridge alongside the exec session.

**Responsibilities:**
- Watch the local clipboard for image content changes
- When new image data is detected, push it to the Pod
- Encrypt data in transit (beyond TLS)
- Handle disconnection and reconnection gracefully

**Clipboard watching strategy (robustness first):**
- **macOS**: Poll `NSPasteboard.changeCount` every 300ms (just a counter check, near-zero cost). When the count changes, read clipboard with `osascript` or CoreGraphics.
- **Linux X11**: Poll with `xclip -selection clipboard -t TARGETS -o` every 500ms. Detect image types.
- **Linux Wayland**: Use `wl-paste --watch` for event-driven notification. Falls back to polling if not available.
- **Fallback**: If event/polling mechanism fails, a manual trigger via a separate `cc-deck clipboard push <session>` command always works.

**Transport mechanism:**
- Use client-go's SPDY executor (same as sync push) to pipe image data into the Pod
- Write to `/tmp/.cc-clipboard/latest.png` with metadata sidecar
- Retry with exponential backoff on exec failures (1s, 2s, 4s, max 30s)
- Skip push if image data hasn't changed (hash comparison)

**Data format pushed to Pod:**
```
/tmp/.cc-clipboard/
  latest.png          # Image data (PNG format, converted if needed)
  metadata.json       # {"timestamp": "...", "mime": "image/png", "size": 12345, "hash": "sha256:..."}
```

### Part 2: Clipboard Shim (in base container image)

Custom `xclip` and `wl-paste` wrappers installed in the base container image at a PATH-priority location.

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

**Key design decisions for the shim:**
- Only intercepts clipboard READ operations for images
- Passes through all other operations (write, text) to real xclip
- Fails gracefully if no bridge data is available
- No dependencies beyond shell

### Part 3: Zellij Plugin Enhancement (cc-zellij-plugin)

The existing cc-zellij-plugin gets optional clipboard bridge integration.

**Core features (nice to have, not required for basic functionality):**
- Status indicator showing clipboard bridge connection state
- Notification when new image data arrives from the bridge ("Image received")
- Alt+V keybinding to trigger manual clipboard sync (sends signal to bridge)

**Stretch features (future):**
- Image preview in the Zellij status bar
- Clipboard history (browse previously pasted images)
- Paste counter per session

The plugin enhances the experience but is NOT required. The shim + bridge provide full transparency without the plugin.

## Architecture Diagram

```
Local Machine                          Remote Pod (K8s)
~~~~~~~~~~~~~                          ~~~~~~~~~~~~~~~~

User takes screenshot
        |
        v
System Clipboard (image)
        |
        v
cc-deck connect (background goroutine)
  - Clipboard watcher
  - Change detection (hash-based)
  - AES-256 encryption
        |
        v
kubectl exec pipe (SPDY, TLS)  -----> /tmp/.cc-clipboard/
        |                                latest.png
        |                                metadata.json
        |                                    ^
        |                              Custom xclip shim
        |                              (/usr/local/bin/xclip)
        |                                    ^
        |                              Claude Code (Ctrl+V)
        |                              (calls xclip, gets image)
        |
        |                              Zellij Plugin (optional)
        |                              - Shows notification
        |                              - Status indicator
```

## Security

### Transport
- The kubectl exec connection goes through the K8s API server, which is TLS-encrypted
- Additional layer: AES-256-GCM symmetric encryption of clipboard payload
- Session key generated at `cc-deck connect` time, shared to Pod via a one-time kubectl exec
- This protects against API server logs or middleware inspecting clipboard content

### Data at Rest
- Staging files in `/tmp/.cc-clipboard/` have `0600` permissions (owner-only)
- The staging directory is created with `0700` permissions
- Files are rotated (only latest image kept, previous overwritten)
- Pod is single-user by design (spec constraint)
- Optional: cleanup timer deletes stale files older than 5 minutes

### Threat Model
- **Intercepted transport**: Mitigated by TLS + AES-256 encryption
- **Pod filesystem access**: Mitigated by file permissions + single-user Pods
- **Clipboard exfiltration**: The bridge only pushes TO the Pod, never reads FROM it. Bidirectional is explicitly out of scope.
- **Malicious clipboard content**: Same risk as local clipboard usage, not amplified by the bridge

## Implementation Plan

### Phase 1: Core Bridge (MVP)
1. Custom `xclip` shim script in the base container image
2. `cc-deck connect` enhancement: background clipboard watcher + push goroutine
3. Polling-based clipboard detection (simplest, most robust)
4. Basic file staging without encryption

### Phase 2: Robustness + Security
1. Event-driven clipboard detection (macOS changeCount, wl-paste --watch)
2. AES-256 encryption for clipboard payload
3. Hash-based deduplication (don't re-push same image)
4. Retry with exponential backoff
5. Graceful degradation when bridge disconnects

### Phase 3: Plugin Integration (Stretch)
1. Zellij plugin: clipboard bridge status indicator
2. Zellij plugin: image arrival notification
3. Zellij plugin: manual sync trigger (Alt+Shift+V)

### Phase 4: Polish (Stretch)
1. Image preview in plugin status bar
2. Clipboard history
3. Support for multiple image formats (JPEG, WebP auto-convert to PNG)
4. `cc-deck clipboard push` manual command for fallback

## Affected Components

| Component | Change | Scope |
|-----------|--------|-------|
| cc-deck CLI | New clipboard watcher goroutine in `connect` command | `internal/session/connect.go`, new `internal/clipboard/` package |
| Base container image | Custom `xclip` shim, staging directory setup | Containerfile, `/usr/local/bin/xclip` |
| cc-zellij-plugin | Optional clipboard bridge integration | Stretch goal |
| cc-deck deploy | No changes needed | N/A |

## Prior Art

| Tool | Approach | Limitation |
|------|----------|------------|
| [Kitty clipboard script](https://blog.shukebeta.com/2025/07/11/quick-fix-claude-code-image-paste-in-linux-terminal/) | Save clipboard image to file, send path via `kitty @ send-text` | Kitty-specific, not remote |
| [VS Code terminal-paste-image](https://github.com/cybersader/vscode-terminal-image-paste) | VS Code extension intercepts paste, saves to project, inserts path | VS Code-specific |
| [Claude Code clipboard plugin](https://martin.hjartmyr.se/articles/claude-code-clipboard-plugin/) | Claude Code plugin with `/clipboard:paste` slash command | Requires user to type command |
| RDP/VNC | Virtual clipboard channel in protocol | Different transport entirely |
| OSC 52 | Terminal escape sequence for clipboard | Text-only, size limits |

## Key Differentiator

This solution is the first to provide **transparent remote clipboard bridging for a terminal-based AI assistant**. No other tool in the Claude Code ecosystem or the broader remote dev environment space (DevPod, Dev Spaces, Codespaces) handles image clipboard sharing for terminal workflows. The xclip shim approach means Claude Code works exactly the same locally and remotely.

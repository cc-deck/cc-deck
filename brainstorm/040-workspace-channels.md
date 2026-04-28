# Brainstorm: Workspace Channels (Design Rationale)

**Date:** 2026-04-21
**Status:** Specified -> [spec 041](../specs/041-workspace-channels/spec.md)
**Trigger:** Comparative analysis with [lince](https://github.com/RisorseArtificiali/lince) revealed that voice relay, clipboard bridging, file transfer, and git harvest all solve the same fundamental problem: bridging the gap between the local machine and a remote workspace.

## Why Channels Exist

cc-deck supports six workspace types (local, container, compose, SSH, k8s-deploy, k8s-sandbox). Multiple features need to send data between the local machine and a remote workspace, and each was building its own exec-based transport:

| Feature | Direction | Previous Approach | Channel Type |
|---|---|---|---|
| Clipboard (images) | local -> remote | Custom kubectl exec pipe | DataChannel |
| Voice text relay | local -> remote | Custom Exec() + zellij pipe | PipeChannel |
| Git push | local -> remote | Custom ext:: over exec | GitChannel |
| Git harvest | remote -> local | Custom ext:: over exec | GitChannel |
| File sync (push) | local -> remote | podman cp / kubectl cp / rsync | DataChannel |
| File sync (pull) | remote -> local | podman cp / kubectl cp / rsync | DataChannel |

This was the same pattern repeated five times with different plumbing. The channel abstraction unifies these into three typed interfaces that wrap the workspace's existing `Exec()` capability.

## The Specification

The full channel design is captured in **[spec 041 (workspace-channels)](../specs/041-workspace-channels/spec.md)**, which covers:

- **PipeChannel**: Unidirectional text/commands to zellij pipes
- **DataChannel**: Bidirectional file/binary data transfer
- **GitChannel**: Git protocol tunneling via ext::
- Per-workspace-type implementations
- Refactoring of existing `Push()`, `Pull()`, `Harvest()` to delegate to channels
- Error model with `ChannelError`
- Success criteria and acceptance scenarios

## Channel Consumers

The channel interfaces are designed to support these higher-level features:

- **[042 Voice Relay](042-voice-relay.md)**: PipeChannel consumer, relays transcribed speech to focused agent pane
- **[043 Clipboard Bridge](043-clipboard-bridge.md)**: DataChannel consumer, pushes clipboard images to remote workspace

## Design Decisions

Key decisions made during the brainstorm that inform the spec:

1. **Channels wrap Exec(), not replace it.** No new network protocol. Channels are Go abstractions over the workspace's existing execution capability.

2. **Security is consumer-level, not channel-level.** The clipboard brainstorm proposed AES-256-GCM encryption. This stays at the consumer level because not all data transfers need encryption. Channels provide transport, consumers decide on encryption.

3. **Local workspaces go through channels too.** For API consistency, local workspaces use thin channel implementations (filesystem access, local zellij pipe). This avoids special-casing in consumer code.

4. **Channels are lazy and cached.** Created on demand, cached per workspace instance. No pre-creation during `cc-deck attach`.

## Prior Art

| Project | Approach | What we learned |
|---|---|---|
| lince | VoxCode + zellij pipe for voice relay | Pipe-based voice relay works, but only for local Zellij |
| DevPod | SSH-based dev environment with port forwarding | Persistent SSH connection for all operations |
| VS Code Remote | Persistent websocket channel for all remote ops | Multiplexed channel over single connection |
| git ext:: protocol | Tunnel git over arbitrary exec commands | Proven pattern for exec-based data transfer |

# Brainstorm: SSH Remote Machine Provisioning

**Date:** 2026-04-08
**Context:** Hands-on testing of spec 033 (SSH Remote Execution Environment) against a Hetzner CAX11 VM (Fedora 43, aarch64)
**Status:** active
**Depends on:** 033-ssh-environment (transport layer, implemented)

## Background

Spec 033 implements the SSH environment transport layer: create, attach, detach, credentials, status, exec, push, pull, and harvest. All of these work correctly against a real remote host. The missing piece is automated provisioning of the remote machine so that the full cc-deck experience (sidebar monitoring, session tracking, credential sourcing) works out of the box.

During testing against a bare Hetzner VM (hostname: marovo, Fedora 43, aarch64), we discovered that the pre-flight bootstrap in spec 033 is insufficient for a production-quality setup. This brainstorm captures what went wrong and proposes the design for the next iteration.

## Findings from Testing

### F-001: Workspace directory not created automatically

The `Create()` flow validates SSH connectivity but does not create the workspace directory on the remote. When the workspace does not exist, `Attach()` fails with "No such file or directory" when trying to `cd` into it.

**Resolution:** Create the workspace directory during `Create()` after pre-flight checks pass, before recording state.

### F-002: Tilde expansion happens locally instead of remotely

When the user passes `--workspace ~/workspace`, the shell expands `~` to the local home directory (e.g., `/Users/rhuss/workspace`). The stored path is then wrong for the remote machine.

**Resolution:** Document that users should quote the path (`'~/workspace'`) or use `$HOME/workspace`. Consider expanding `~` to `$HOME` in the CLI before storing.

### F-003: Layout file contains absolute local paths

Copying the local `cc-deck.kdl` layout to the remote preserves absolute paths like `/Users/rhuss/.config/zellij/plugins/cc_deck.wasm`. Zellij on the remote cannot find the plugin at that path.

**Resolution:** The layout on the remote must use relative paths (e.g., `file:~/.config/zellij/plugins/cc_deck.wasm`). Running `cc-deck plugin install` on the remote generates the layout with correct paths.

### F-004: Controller plugin not loaded

The cc-deck layout only defines the sidebar plugin instance. The controller plugin is loaded via `load_plugins` in the Zellij config, which is set up by `cc-deck plugin install`. Without the controller, the sidebar shows "Waiting for controller."

**Resolution:** Running `cc-deck plugin install` on the remote adds the controller to `load_plugins` in the Zellij config. This is not something we can replicate by copying individual files.

### F-005: Claude Code hooks not registered

Even with the controller running, the sidebar shows "No Claude sessions" because the Claude Code hooks (in `~/.claude/settings.json` on the remote) are not configured. These hooks pipe session events through Zellij pipes to the controller plugin. `cc-deck plugin install` registers them.

**Resolution:** Same as F-004. Running `cc-deck plugin install` on the remote is the only correct approach.

### F-006: Credential file not sourced by shell

Credentials are written to `~/.config/cc-deck/credentials.env` on the remote, but new Zellij panes do not source this file automatically. The spec says "no modification of user shell config files," but without sourcing there is no way for new panes to pick up credentials.

**Resolution:** Two options. First, `cc-deck plugin install` could add sourcing to `.bashrc` (practical but violates spec intent). Second, generate a layout where pane commands wrap credential sourcing. This needs further design.

### F-007: Zellij remedy uses wrong URL format

The Zellij install remedy constructs URLs using Go-normalized architecture names (`arm64`, `amd64`) but the GitHub release uses raw uname names (`aarch64`, `x86_64`). The download URL is wrong.

**Resolution:** Store the raw uname architecture alongside the normalized one, or reverse the normalization when constructing download URLs.

### F-008: Claude Code remedy uses npm instead of official installer

The `ClaudeCodeCheck` remedy runs `npm install -g @anthropic-ai/claude-code`. The official installation method is `curl -fsSL https://claude.ai/install.sh | bash`, which installs a native binary with auto-updates.

**Resolution:** Switch to the official installer in the remedy.

### F-009: cc-deck remedy references nonexistent install script

The `CcDeckCheck` remedy runs `curl | bash` from a `main` branch install script that does not exist in the repository.

**Resolution:** Download the release binary from GitHub Releases for the detected OS and architecture. The release process (spec 021) publishes `cc-deck-linux-amd64` and `cc-deck-linux-arm64` binaries.

### F-010: Zellij `attach --layout` flag does not exist

The attach flow used `zellij attach --create-background <name> --layout cc-deck`, but the `--layout` flag is not valid on `zellij attach`. The layout is a top-level flag: `zellij --layout cc-deck attach --create-background <name>`.

**Resolution:** Fixed during testing. The code now uses the correct flag position and falls back to the default layout if the cc-deck layout is not found.

## Design: Automated Remote Provisioning

### Core Principle

The remote machine should be provisioned by running `cc-deck plugin install` on the remote via SSH. This single command handles the WASM plugin, layout files, controller config, and Claude Code hooks. We should not try to replicate its behavior by copying individual files.

### Proposed Pre-flight Sequence

```
1. SSH connectivity check
2. OS/architecture detection
3. Zellij check → remedy: download release binary
4. Claude Code check → remedy: official installer (curl | bash)
5. cc-deck CLI check → remedy: download release binary from GitHub Releases
6. cc-deck plugin check → remedy: run `cc-deck plugin install` on remote
7. Credential verification
8. Create workspace directory
```

Steps 5 and 6 are the key change. Step 5 downloads the cc-deck binary. Step 6 runs `cc-deck plugin install`, which sets up everything else.

### Analogy: Image Building Pattern

The cc-deck image building system (specs 017-018) uses a declarative manifest (`cc-deck-image.yaml`) to describe what goes into a container image. The build process reads the manifest, resolves dependencies, and produces a ready-to-use environment.

We can apply the same pattern to SSH remote provisioning:

1. **Manifest:** The environment definition (host, workspace, auth mode, tool versions) is the "manifest" for the remote machine.
2. **Build step:** The pre-flight bootstrap reads the manifest, checks the remote state, and installs what is missing.
3. **Idempotent:** Running pre-flight again on an already-provisioned machine is a no-op (all checks pass).
4. **Reproducible:** A second machine with the same definition gets the same setup.

### Version Pinning

The current remedies download "latest" versions. For reproducibility, we should consider:

- Pinning Zellij to the version used during development/testing
- Pinning cc-deck to the version running locally (so remote and local match)
- Claude Code auto-updates itself, so pinning is less relevant

### Credential Sourcing Strategy

The spec says "Zellij layout ENV directive or wrapper command." The most practical approach:

1. Generate a remote-specific layout where panes start with `bash --rcfile <wrapper>` where `<wrapper>` sources credentials then sources the real `.bashrc`.
2. `cc-deck plugin install` could accept a `--remote` flag that generates this wrapper-style layout.
3. Alternatively, add a small shell snippet to `.bashrc` that sources the credential file if it exists. This is what tools like `nvm`, `rvm`, and `sdkman` do, and users generally accept it.

### Open Questions

- Q: Should we version-pin all tools or use latest?
- Q: Should `cc-deck plugin install --remote` be a separate mode or should the regular install detect it is on a headless machine?
- Q: Should the credential sourcing be in `.bashrc`, a layout wrapper, or a Zellij environment directive (if Zellij adds support)?
- Q: Should the pre-flight bootstrap run again on every `attach` (detect drift) or only on `create`?
- Q: How do we handle cc-deck upgrades on the remote? Auto-update like Claude Code, or manual via `refresh-creds`-style command?

## Next Steps

1. Create a spec (034 or similar) for SSH remote provisioning
2. Fix the immediate remedy bugs in spec 033 (wrong URLs, wrong installer)
3. Design the `cc-deck plugin install --remote` mode
4. Implement and test against marovo (Hetzner CAX11, Fedora 43, aarch64)

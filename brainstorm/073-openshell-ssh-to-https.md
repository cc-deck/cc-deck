# Brainstorm: OpenShell SSH-to-HTTPS Git Clone Conversion

**Date:** 2026-06-20
**Status:** active

## Problem Framing

When `cc-deck ws new` clones repositories into an OpenShell sandbox using SSH URLs (`git@github.com:owner/repo.git`), the clone fails with "Could not resolve hostname github.com: Temporary failure in name resolution". HTTPS URLs work fine for the same host.

The user passes `--repo git@github.com:cc-deck/cc-deck.git` on the command line (or has SSH URLs in their manifest sources). The clone fails silently with a warning, leaving the workspace without source code.

## Root Cause

OpenShell routes all outbound sandbox traffic through an **HTTP CONNECT proxy**. The proxy handles:

- **HTTPS (port 443)**: Full support. TLS termination, L7 inspection, credential injection. DNS resolution happens inside the proxy's CONNECT tunnel.
- **Raw TCP (port 22)**: Supported as raw binary relay once a tunnel is established. No L7 inspection.

The problem is **DNS resolution**, not the SSH tunnel itself:

1. `git clone git@github.com:...` calls the system resolver first (nameserver `10.89.0.1`, podman DNS)
2. DNS queries are UDP port 53, which do NOT go through the HTTP CONNECT proxy
3. The podman DNS resolver cannot reach external DNS servers from inside the sandboxed network
4. DNS fails before SSH even attempts to connect
5. If DNS somehow succeeded, the SSH tunnel would work (raw binary relay is native to the proxy)

HTTPS works because the DNS resolution happens inside the CONNECT tunnel, not via the system resolver.

## Approaches Considered

### A: Convert SSH URLs to HTTPS in cc-deck for OpenShell workspaces (Recommended)

When the workspace type is OpenShell, `buildCloneCommand()` converts `git@github.com:owner/repo.git` to `https://github.com/owner/repo.git` before cloning. The conversion is straightforward and well-defined for GitHub, GitLab, and Bitbucket.

- Pros: Simple, no OpenShell changes needed, works today. Matches OpenShell's design intent (HTTP-native proxy). Token injection via `injectToken()` already handles HTTPS auth.
- Cons: SSH keys are not used for auth inside the sandbox (must use token-based HTTPS auth or rely on public repos). Users who pass SSH URLs may not realize they are being converted.

### B: Use git's `insteadOf` config inside the sandbox

Instead of converting URLs in Go code, configure git inside the sandbox:
```
git config --global url."https://github.com/".insteadOf "git@github.com:"
```

This could be done in the Containerfile or during workspace setup.

- Pros: Transparent to all git operations inside the sandbox, not just cc-deck clones. Works for submodules and dependencies too.
- Cons: Affects all git operations (may surprise users). Needs to cover GitHub, GitLab, Bitbucket patterns. Still needs HTTPS auth.

### C: Enable DNS-over-HTTPS (DoH) in the sandbox

Configure the sandbox to resolve DNS via a DoH endpoint that goes through the HTTP CONNECT proxy, instead of plain UDP DNS.

- Pros: SSH would work naturally. No URL conversion needed.
- Cons: Requires OpenShell platform changes (not in cc-deck's control). DoH configuration is complex and varies by OS. Not a standard sandbox feature.

## Decision

**Approach A + B combined**: Convert URLs in Go code (Approach A) for the `ws new` clone operation, AND set up `insteadOf` git config inside OpenShell images (Approach B) to cover submodules, `go get`, and other git operations that happen inside the sandbox.

Approach A handles the immediate `ws new --repo` case. Approach B handles all other git operations inside the running sandbox.

## Key Requirements

- `buildCloneCommand()` must convert SSH URLs to HTTPS when the workspace type is OpenShell
- The conversion must handle: `git@github.com:owner/repo.git`, `git@gitlab.com:owner/repo.git`, `git@bitbucket.org:owner/repo.git`, and custom SSH hosts
- General SSH URL format: `git@<host>:<path>.git` becomes `https://<host>/<path>.git`
- Log a message when conversion happens so the user knows: `"Converting SSH URL to HTTPS for OpenShell sandbox: git@github.com:owner/repo.git -> https://github.com/owner/repo.git"`
- The `insteadOf` config should be added to the OpenShell Containerfile (build skill, Section C2) for GitHub, GitLab, and Bitbucket
- If git token credentials are available (from the profile), inject them into HTTPS URLs for private repo access
- Public repos should clone without auth

## Open Questions

- Should the `insteadOf` config also be applied to container builds (Section A), or only OpenShell? Container builds may have different network characteristics.
- Should `cc-deck ws new` warn when SSH URLs are converted, or just log at debug level?
- For private repos: the current `GitCredentialToken` mechanism injects tokens into HTTPS URLs. Is this sufficient, or do we need a GitHub App token or `gh auth` setup inside the sandbox?

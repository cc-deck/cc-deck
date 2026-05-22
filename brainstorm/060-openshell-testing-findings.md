# Brainstorm: OpenShell Integration Testing Findings

**Date:** 2026-05-22
**Status:** active

## Problem Framing

End-to-end testing of cc-deck's OpenShell integration (specs 056 and 058) uncovered several issues at different layers: image build, sandbox lifecycle, credential injection, network policy, and the supervisor's seccomp restrictions. This document captures what we learned, what works, what does not, and what still needs resolution.

## What works

### Image build and capture
- `/cc-deck.capture --all` detects tools, settings, plugins, MCP servers, and credentials from the host environment
- `claude-vertex` credential type detected correctly when `CLAUDE_CODE_USE_VERTEX` + `ANTHROPIC_VERTEX_PROJECT_ID` are set
- `/cc-deck.build --target openshell` generates a Containerfile from the manifest using the snippet assembly model (Go templates for fixed layers, Claude Code generates variable parts)
- Policy generation includes all Vertex AI regions (explicit hostnames, not wildcards) and package registries for detected tools
- `cc-deck build run --target openshell` builds the image via podman

### Workspace lifecycle
- `cc-deck ws new --type openshell` creates providers, provisions sandbox, clones repos
- `claude-vertex` env vars (`CLAUDE_CODE_USE_VERTEX`, `ANTHROPIC_VERTEX_PROJECT_ID`, `CLOUD_ML_REGION`) injected into sandbox shell rc files after sandbox starts
- GitHub provider created via `openshell provider create --type github --from-existing`
- Repo cloning works inside the sandbox via `openshell sandbox exec`
- `cc-deck ws attach` connects via `openshell sandbox connect` (SSH-based, proper terminal handling)
- Zellij auto-start works from `.bashrc` (the `exec zellij --layout cc-deck attach --create cc-deck` block)

### Sidebar and badges
- Monochrome badge icons work (`▶` for ship, `◆` for flow, `✎ ? ☑ ▦ ☰ ⚙ ◉ ✓` for phases)
- Per-badge custom colors via `#RRGGBB:` prefix in config values
- CJK width calculation (`width_cjk()`) fixes alignment for ambiguous-width characters
- Session indicator prefix dynamically calculated instead of hardcoded

## What does not work

### Claude Code in OpenShell sandbox (getifaddrs / AF_NETLINK)

**The blocking issue.** Claude Code v2.1.145 fails with `API Error: A system error occurred: getifaddrs returned an error` when making Vertex AI API calls from inside an OpenShell sandbox.

**Root cause:** The OpenShell supervisor's seccomp filter blocks `AF_NETLINK` sockets. Node.js (Claude Code's runtime) calls `os.networkInterfaces()` (which uses `getifaddrs()`, which uses `AF_NETLINK`) before making API requests. The call fails and Claude Code does not catch the error.

**What we verified:**
- `curl` to `global-aiplatform.googleapis.com` works (404) after adding endpoints to policy
- Node.js HTTPS works if `getifaddrs` error is caught first: `try{require('os').networkInterfaces()}catch(e){};` then HTTPS succeeds
- The error comes from `uv_interface_addresses` in libuv (Claude Code's bundled Node.js runtime)
- Claude Code starts fine (detects Vertex AI), but fails on the first API call

**Workarounds attempted:**
- `NODE_OPTIONS="--require shim.js"` with a JS shim: Does not work. Claude Code bundles its own Node.js as a compiled ELF binary, ignores `NODE_OPTIONS`.
- `LD_PRELOAD` with a C shim intercepting `getifaddrs` at libc level: Not yet tested to completion (sandbox exec had issues with multi-line heredocs)

**What others do:**
- Emilien Macchi (emacchi) builds the supervisor from source with `sed -i /libc::AF_NETLINK,/d in seccomp.rs`. Works but requires carrying a fork.
- Colin Walters (walters) runs OpenShell with patches that remove all inner sandboxing ([gist](https://gist.github.com/cgwalters/4f2e641ccb0c42361a89f71d17ede04f))
- Florent Benoit (fbenoit) says he runs Claude Code through `openshell create` / `openshell connect` successfully with Vertex, but details are unclear. He may be on a different supervisor build.
- Nobody in the Red Hat Slack has confirmed Claude Code working in a stock OpenShell sandbox with the upstream supervisor.

**Upstream status:**
- [Issue #955](https://github.com/NVIDIA/OpenShell/issues/955) (getifaddrs) is closed, but was fixed in OpenClaw, not in OpenShell's supervisor
- [PR #1006](https://github.com/NVIDIA/OpenShell/pull/1006) (allow AF_NETLINK) was auto-closed and never re-submitted
- The Red Hat blog about "Claude self-hosted sandboxes on OpenShell" is about Claude Managed Agents (`ant` CLI), not Claude Code

### OpenShell 0.0.46 macOS regression

Upgrading from 0.0.41 to 0.0.46 broke the gateway on macOS with podman (libkrun VMs). The new version tries to bind a bridge network listener on `10.89.0.1:17670` (the podman bridge subnet gateway IP). On macOS, this IP only exists inside the VM, not on the host, causing `EADDRNOTAVAIL`.

**Workaround:** `sudo ifconfig lo0 alias 10.89.0.1/32` makes the gateway start, but sandboxes stay stuck in Provisioning because the bridge network is not actually routable between host and container on macOS.

**Recommendation:** Stay on 0.0.41 until the macOS bridge networking is fixed, or file an issue.

## Policy learnings

### Wildcards do not work for Vertex endpoints
OpenShell wildcards match subdomains (`*.example.com` = `foo.example.com`) but NOT hostname prefixes (`*.aiplatform.googleapis.com` does NOT match `us-east1-aiplatform.googleapis.com`). All Vertex AI regions must be listed explicitly.

The wildcard fix in [issue #1303](https://github.com/NVIDIA/OpenShell/issues/1303) / [PR #1304](https://github.com/NVIDIA/OpenShell/pull/1304) relaxes intra-label validation (`*-aiplatform.googleapis.com`), but this is in 0.0.46+ which has the macOS regression.

### Binary scoping required
Adding an endpoint to the policy without `--binary` flags means no process can use it. The proxy enforces per-binary access. All endpoint additions must include the relevant binaries (`/usr/local/bin/claude`, `/sandbox/.local/bin/claude`, `/usr/bin/node`, `/usr/bin/curl`).

### Provider update syntax
`openshell provider update` takes the name as a positional arg (not `--name`) and does not accept `--type` (type is immutable after creation). Different from `provider create`.

## Image build learnings

### Shell environment
- OpenShell `sandbox connect` uses bash regardless of `/etc/passwd` setting. `$SHELL` is `/bin/bash`.
- Zellij `default_shell "zsh"` in config.kdl makes panes use zsh, but the connect shell is still bash.
- The Zellij auto-start block must be in `.bashrc` (for connect) AND `.zshrc` (for Zellij panes).
- `export SHELL=/bin/zsh` before `exec zellij` ensures Zellij inherits the right shell.

### File ownership
All Containerfile layers that modify `/sandbox/` files must be followed by a final `chown -R sandbox:sandbox /sandbox` before `USER sandbox`. Without this, shell rc files end up owned by root and the sandbox user cannot modify them.

### Starship and shell finalization
The starship init and Zellij auto-start must come AFTER all user config COPY/RUN layers. If placed before, user config layers can overwrite `.zshrc` and lose the additions. The build spec template now enforces this order with a `FINAL SHELL SETUP` marker.

### Zellij version
`--install-zellij` now queries GitHub for the latest release instead of hardcoding the SDK version. The plugin SDK version (0.44) and the Zellij binary version (0.44.3) are independent.

## Next steps

1. **Try `LD_PRELOAD` shim** from inside the sandbox (compile a C shim that stubs `getifaddrs`, set `LD_PRELOAD` in shell rc, test with `claude`)
2. **If shim works**: Bake it into the Containerfile (compile during build, add `LD_PRELOAD` to shell rc)
3. **File OpenShell issue** for 0.0.46 macOS bridge network regression
4. **Stay on 0.0.41** until macOS is fixed
5. **Consider contributing** the AF_NETLINK fix upstream (re-submit PR #1006 or a variant)
6. **Test with API key auth** (not Vertex) to check if `getifaddrs` is Vertex-specific or affects all auth modes

## Open Questions

- Does `getifaddrs` fail with API key auth too, or only Vertex? The Vertex code path in the Anthropic SDK may be the only one calling `os.networkInterfaces()`.
- Is Florent running a patched supervisor, or does his setup differ in a way that avoids the seccomp filter?
- When will OpenShell fix the macOS bridge networking in 0.0.46+?
- Should cc-deck provide an `--openshell-version` flag to pin the supervisor version in the image?

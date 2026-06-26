# Brainstorm Overview

Last updated: 2026-06-26 (075 openshell native vertex provider)

## Active Brainstorms

| # | Date | Topic | Status | Spec |
|---|------|-------|--------|------|
| 022 | 2026-03-19 | Multi-agent support | active | - |
| 023 | 2026-03-12 | Git workflow | brainstorm | - |
| 025 | 2026-03-12 | Security model | brainstorm | - |
| 040 | 2026-04-21 | Workspace channels | specified | 041 |
| 042 | 2026-04-23 | Voice relay | brainstorm | - |
| 043 | 2026-04-23 | Clipboard bridge | brainstorm | - |
| 044 | 2026-04-24 | Sidebar session isolation | brainstorm | - |
| 045 | 2026-04-29 | Voice sidebar integration | active | - |
| 046 | 2026-04-30 | Voice attend stop word | active | - |
| 047 | 2026-04-30 | Landing page revival | active | - |
| 048 | 2026-05-04 | Voice transcript recording | active | - |
| 049 | 2026-05-06 | WASM dead code cleanup | active | - |
| 050 | 2026-05-06 | Test coverage measurement | active | - |
| 053 | 2026-05-15 | OpenShell build integration | active | - |
| 056 | 2026-05-18 | Sidebar badges | active | - |
| 058 | 2026-05-19 | Image tool plugins | parked | - |
| 059 | 2026-05-19 | OpenShell credential injection | active | - |
| 060 | 2026-05-22 | OpenShell testing findings | active | - |
| 061 | 2026-05-22 | Deterministic policy generation | active | - |
| 062 | 2026-05-22 | OCI policy extraction | active | 060 |
| 063 | 2026-05-23 | Policy binary resolution | active | 061 |
| 063 | 2026-05-24 | MCP endpoint policy | spec-created | 063 |
| 064 | 2026-05-24 | Two-pass binary probing | active | - |
| 065 | 2026-05-24 | Egress recording mode | active | - |
| 066 | 2026-05-25 | Tool PATH restoration | active | - |
| 067 | 2026-06-02 | Config validation | active | - |
| 068 | 2026-06-06 | Network policy generalization | active | - |
| 069 | 2026-06-06 | Credential transport abstraction | active | - |
| 070 | 2026-06-06 | Build system multi-agent | active | - |
| 071 | 2026-06-08 | Sidebar session sort | active | - |
| 072 | 2026-06-17 | Build skill iteration reduction | active | - |
| 073 | 2026-06-20 | OpenShell SSH-to-HTTPS | active | - |
| 074 | 2026-06-22 | OpenShell resource limits | active | - |
| 075 | 2026-06-26 | OpenShell native Vertex provider | active | - |

## Open Threads

- Multi-agent support: Pure Go Agent interface, Claude + OpenCode adapters as first spec (066), cc-deck-agent-wrapper for hookless agents, Rust plugin generalization (from #022)
- Network policy generalization: per-agent domain declarations, remove `match: always: true`, hybrid approach (Agent interface declares groups, YAML provides endpoint lists). Depends on #066 (from #068)
- Credential transport: **decided** hybrid with agent-heavy ownership. Agent interface declares `CredentialSpecs()` per auth mode. Thin shared `internal/credential` package for transport only. User-selectable auth mode with auto-detection fallback. Eager validation with "externally provided" escape for K8s/OpenShell. Ready for specification. (from #069, revisited 2026-06-07)
- Build system multi-agent: manifest `agents` field, per-agent InstallScript/ConfigPaths/ProbeCommands, multi-agent Containerfile generation, pipefail + verification. Depends on #066, #068, #069 (from #070)
- Voice relay: speech-to-text relay via PipeChannel, plugin-side handler, local capture strategy (from #042, depends on spec 041)
- Voice sidebar integration: ♫ indicator, mute toggle, [[command]] protocol, PTT removal, bidirectional state sync (from #045)
- Voice attend stop word: whether additional voice actions beyond "submit" and "attend" will be needed (from #046)
- Voice transcript recording: auto-start recording via CLI flag, whether to include command words in transcript (from #048)
- Landing page revival: Tabler icon selection, demo container one-liner wording, local install path (brew vs curl), screenshot/GIF asset creation timeline (from #047)
- Clipboard bridge: image paste in remote workspaces via DataChannel + xclip shim (from #043, depends on spec 041)
- Git workflow: git push/harvest patterns for remote workspaces (from #023)
- Security model: credential proxy, git-push restriction, sandbox hardening (from #025)
- WASM dead code cleanup: binary size reduction measurement after LTO (may already strip dead code), audit sync.rs for shared helpers worth keeping (from #049)
- Test coverage measurement: coverage floor value TBD after first baseline, per-module CI reporting TBD after initial results (from #050)
- OpenShell build integration: capture-phase binary-to-endpoint discovery, skills-to-plugins mapping, Zellij-specific policy auto-additions, policy precedence (--policy > env > image-embedded), verify target for openshell (from #053)
- Sidebar badges: badge evaluation caching, max badge count, YAML format support, dot-path array handling (from #056)
- OpenShell credential injection: missing credential error handling (error vs warn vs prompt), provider idempotency (reuse existing?), Vertex migration path when OpenShell adds native support, custom provider types beyond known profiles, credential refresh for OpenShell workspaces (from #059)
- Unified credential handling: refactor credential injection across all workspace types (container/SSH/K8s/compose/OpenShell) into shared interface (from #059, future brainstorm)
- OpenShell testing: LD_PRELOAD shim validated but temporary (needs upstream AF_NETLINK fix via PR #1006), macOS bridge networking workaround documented, policy binary glob pattern for Claude Code versioned binary (from #060)
- Deterministic policy generation: catalog repo name TBD, MCP server endpoint extraction format, component dependency model, supply chain signing deferred (from #061)
- OCI policy extraction: go-containerregistry integration for runtime policy extraction from image layers (from #062)
- Policy binary resolution: well-known paths table vs manifest-driven resolution at assembly time (from #063)
- Two-pass binary probing: probe built image for actual binary paths, remove hardcoded well-known paths table (from #064)
- Egress recording mode: CoreDNS sidecar image, non-interactive/CI mode, DNS noise filtering, merge vs replace strategy for recorded domains, OpenShell OCSF enhancement (from #065)
- Tool PATH restoration: registry format (Go map vs YAML), user-relative paths with {{.HomeDir}}, directory guards, curated zshrc dedup of tool paths (from #066)
- Config validation: load-time warning suppression mechanism, curated safe icon list vs constraint description (from #067)
- Sidebar session sort: move_focus_or_tab swap mechanics (focus requirement during sort sequence), controller vs sidebar sort computation, performance for 10+ sessions (from #071)
- Build skill iteration reduction: 13 skill changes to eliminate build iterations. Skill-first approach chosen (edit markdowns + templates, no new Go code). Dual-phase asset verification (capture + build), shell config dependency scanning, post_install dry-run at capture, snippet verification on refresh. Depends on #064, #060 (from #072, revisited 2026-06-20)
- OpenShell SSH-to-HTTPS: Convert SSH git URLs to HTTPS for OpenShell sandboxes. OpenShell's HTTP CONNECT proxy cannot resolve DNS for SSH (UDP port 53 bypasses proxy). Fix: convert in buildCloneCommand() + git insteadOf config in image. (from #073)
- OpenShell resource limits: Expose --cpu and --memory flags on ws new for OpenShell sandboxes. Defaults are 2 vCPU / 2 GB (too low for Rust/Java builds). Phase 1: CLI flags. Phase 2: manifest defaults with capture-time detection. (from #074)
- OpenShell native Vertex provider: Replace homegrown Vertex credential handling with OpenShell's native google-cloud provider (GCE metadata emulator, PR #1763). Remove file credential upload, dead vertex profile, Vertex network domains from OpenShell policy. Keep env var injection for Claude Code. OpenShell workspaces only. (from #075)

## Parked Ideas

- Image tool plugins: multi-harness plugin config sections, harness detection at container start, runtime vs build-time plugin install (#058)
  Reason: deferred until a second harness beyond Claude Code is supported. RTK integration was implemented directly in capture wizard.

## Attic

Completed brainstorms that have corresponding specs (active or attic) are in `brainstorm/attic/`. See `ls brainstorm/attic/` for the full list.

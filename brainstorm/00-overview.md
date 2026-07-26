# Brainstorm Overview

Last updated: 2026-07-26

## Sessions

| # | Date | Topic | Status | Spec | Issue |
|---|------|-------|--------|------|-------|
| 01 | 2026-04-30 | cc-deck-openshell-backend | active | 049 | - |
| 022 | 2026-03-19 | multi-agent-support | active | - | - |
| 23 | 2026-03-16 | git-workflow | brainstorm | - | - |
| 25 | 2026-03-16 | security-model | brainstorm | - | - |
| 26 | 2026-05-18 | sidebar-badges | active | 057 | - |
| 27 | 2026-06-30 | openshell-sdk-migration | active | 075 | - |
| 040 | 2026-04-21 | workspace-channels | specified | 041 | - |
| 042 | 2026-04-23 | voice-relay | brainstorm | 042 | - |
| 043 | 2026-03-03 | clipboard-bridge | brainstorm | - | - |
| 043 | - | dead-code-cleanup | - | - | - |
| 044 | 2026-04-25 | sidebar-session-isolation | active | 044 | - |
| 045 | 2026-04-29 | voice-sidebar-integration | active | 045 | - |
| 046 | 2026-04-30 | voice-attend-stopword | active | 046 | - |
| 047 | 2026-04-30 | landing-page-revival | active | 047 | - |
| 048 | 2026-05-04 | voice-transcript-recording | active | 048 | - |
| 049 | 2026-05-06 | openshell-grpc-vs-cli | active | 049 | - |
| 049 | 2026-05-06 | wasm-dead-code-cleanup | active | 049 | - |
| 050 | 2026-05-06 | test-coverage-measurement | active | 050 | - |
| 051 | 2026-05-06 | proptest-fuzz-testing | proposed | 051 | - |
| 052 | 2026-05-06 | plugin-integration-e2e-testing | proposed | 052 | - |
| 053 | 2026-05-15 | openshell-build-integration | active | 056 | - |
| 053 | 2026-05-14 | render-pipeline-stability | proposed | 053 | - |
| 055 | 2026-05-15 | dual-controller-workarounds | - | 055 | - |
| 055 | - | sidebar-voice-indicator-stability | - | - | - |
| 055 | - | zellij-upstream-issue-draft | - | - | - |
| 056 | - | agent-id-waiting-guard | - | - | - |
| 057 | 2026-05-18 | autonomous-task-dispatch | brainstorm | - | - |
| 058 | 2026-05-19 | image-tool-plugins | parked | - | - |
| 059 | 2026-05-19 | openshell-credential-injection | active | 058 | - |
| 060 | 2026-05-22 | openshell-testing-findings | active | - | - |
| 061 | 2026-05-22 | deterministic-policy-generation | active | 059 | - |
| 062 | - | oci-policy-extraction | - | 060 | - |
| 063 | - | mcp-endpoint-policy | - | 063 | - |
| 063 | - | policy-binary-resolution | - | 061 | - |
| 064 | 2026-05-24 | two-pass-binary-probing | active | 062 | - |
| 065 | 2026-05-24 | egress-recording-mode | active | 062 | - |
| 066 | 2026-05-25 | tool-path-restoration | active | 064 | - |
| 067 | 2026-06-02 | config-validation | active | 065 | - |
| 068 | 2026-06-06 | network-policy-generalization | active | 078 | - |
| 069 | 2026-06-06 | credential-transport-abstraction | active | 069 | - |
| 070 | 2026-06-06 | build-system-multi-agent | active | - | - |
| 071 | 2026-06-08 | sidebar-session-sort | active | 067 | - |
| 072 | 2026-06-17 | build-skill-iteration-reduction | active | 071 | - |
| 073 | 2026-06-20 | openshell-ssh-to-https | active | 072 | - |
| 074 | 2026-06-22 | openshell-resource-limits | active | - | - |
| 075 | 2026-06-26 | openshell-native-vertex-provider | active | 073 | - |
| 076 | 2026-06-28 | virtual-sort-fix | active | 074 | - |
| 076 | 2026-07-01 | worktree-sidebar-visibility | active | 076 | - |
| 077 | 2026-07-03 | voice-glossary | active | 077 | - |
| 080 | 2026-07-11 | sidebar-auto-sort | active | 080 | - |
| 081 | 2026-07-12 | codex-agent-adapter | active | 081 | - |
| 082 | 2026-07-22 | session-sharing-spike | active | - | - |
| 083 | 2026-07-22 | pluggable-tunnel-interface | active | - | - |
| 084 | 2026-07-22 | multi-backend-sharing | parked | - | - |
| 085 | 2026-07-22 | sidebar-presence-panel | parked | - | - |
| 086 | 2026-07-22 | speckit-pair-programming | parked | - | - |
| 087 | 2026-07-22 | multiplayer-plugin-resilience | active | 082 | - |
| 088 | 2026-07-21 | multiplayer-focus-modes | active | 083 | - |
| 089 | 2026-07-26 | cross-pane-mcp | active | - | [#13](https://github.com/cc-deck/cc-deck/issues/13) |
| - | 2026-05-14 | zellij-load-plugins-duplicate-instance | draft | - | - |

## Open Threads

- Custom sandbox Dockerfile contents, sidebar plugin tunneling inside sandbox, multi-sandbox vs single-sandbox layout, domain group to OPA/Rego mapping, gRPC API stability, credential provider flow (from #01)
- Multi-agent support: Pure Go Agent interface, Claude + OpenCode adapters, cc-deck-agent-wrapper for hookless agents, Rust plugin generalization (from #022)
- Sidebar session isolation: orphaned state file cleanup vs PID reuse (from #044)
- Voice sidebar integration: indicator color values, [[command]] protocol extensibility, click region sizing, mute toggle reverse pipe direction (from #045)
- Voice attend stop word: whether additional voice actions beyond "submit" and "attend" will be needed (from #046)
- Landing page revival: Tabler icon selection, demo container one-liner wording, local install path (brew vs curl), screenshot/GIF asset creation timeline (from #047)
- Voice transcript recording: auto-start recording via CLI flag, whether to include command words in transcript (from #048)
- WASM dead code cleanup: binary size reduction measurement after LTO, audit sync.rs shared helpers worth keeping (from #049)
- Test coverage measurement: coverage floor value TBD after first baseline, per-module CI reporting TBD (from #050)
- Proptest fuzz testing: test directory location (inline vs separate), Zellij E2E framework (Go vs Rust/shell), timer-dependent behavior, screenshot/golden-file testing value (from #051, #052)
- OpenShell build integration: capture-phase binary-to-endpoint discovery, registry push vs local build, skills-to-plugins mapping, build verify for OpenShell, Zellij-specific policy auto-additions, policy precedence documentation (from #053)
- Render pipeline stability: why Zellij creates second controller, controller single-instance validation, broadcast_render_all necessity, simpler timer-based render model (from #053)
- OpenShell credential injection: missing credential error handling, provider idempotency, Vertex migration path, custom provider types, credential refresh for OpenShell (from #059)
- OpenShell testing: LD_PRELOAD shim scope (API key vs Vertex), macOS bridge networking fix timeline, auto-detection of shim need (from #060)
- Two-pass binary probing: interpreter symlink resolution, probe result caching (from #064)
- Egress recording mode: CoreDNS image selection, non-interactive/CI mode, DNS noise filtering, merge vs diff strategy, multi-session recording, OpenShell OCSF log enhancement (from #065)
- Tool PATH restoration: registry format (Go map vs YAML), user-relative paths with {{.HomeDir}}, directory guards, curated zshrc dedup (from #066)
- Config validation: load-time warning suppression, curated safe icon list vs constraint description (from #067)
- Network policy generalization: custom per-agent domain groups, per-agent domain alias interaction with domains.yaml, per-agent deny rules (from #068)
- Credential transport abstraction: credentials check command, credential rotation in long-running containers (from #069)
- Build system multi-agent: default vs opt-in agents, version pinning, agent listing command, OS dependency conflicts, manifest installability validation, per-agent port exposure (from #070)
- Sidebar session sort: move_focus_or_tab swap mechanics, controller vs sidebar sort computation, performance for 10+ sessions (from #071)
- Build skill iteration reduction: capture-time GitHub release verification, snippet verification on refresh, npm vs native installer for Claude Code, post_install dry-run at capture (from #072)
- OpenShell SSH-to-HTTPS: container build applicability, warn vs debug log on conversion, private repo token handling (from #073)
- OpenShell resource limits: live resize support, global resource defaults vs per-project, ws status resource display (from #074)
- OpenShell native Vertex provider: providers_v2_enabled flag scope, minimum OpenShell version for PR #1763 (from #075)
- Virtual sort fix: none (focused bug fix with clear path) (from #076)
- Worktree sidebar visibility: in_worktree state persistence across reattach, worktree icon color differentiation (from #076)
- Voice glossary: glossary token limit warning vs silent truncation, comment support in glossary file (from #077)
- Sidebar auto-sort: separator line style (thin/dashed/labeled), separator width and alignment (from #080)
- Codex agent adapter: indicator symbol selection, version detection for hooks, tool name normalization (apply_patch vs Write/Edit) (from #081)
- Pluggable tunnel interface: provider abstraction, Cloudflare Quick Tunnel as default (from #083)
- Multiplayer plugin resilience: get_plugin_ids client distinction, election with web client instances, sidebar skip for web clients, client_id=0 reliability, upstream plugin cleanup on disconnect (from #087)
- Multiplayer focus modes: tab switch sync scope, per-client vs global mode state, session tracking in independent mode, focus_terminal_pane cross-client correctness (from #088)
- Cross-pane MCP: prompt injection format across agents, timeout/error handling, temp file cleanup strategy, auto-add MCP config on install, voice relay infrastructure sharing, new pipe message type vs extension (from #089)
- Git workflow: auto-stash on harvest, merge conflict handling, SSH key support for private repos, ext:: vs tar performance, reset scope (git only vs conversation), sync.go integration, sidebar dual sync status (from #23)
- Security model: credential watchdog default, re-injection after removal, YOLO mode enforcement vs warning, agent action auditing, external secret management, security scoring, secret vs passthrough typing, env var sync configurability (from #25)
- Sidebar badges: badge evaluation caching, max badge count, YAML format support, dot-path array handling (from #26)
- OpenShell SDK migration: GatewayConfig to SDK Config mapping, credentials.go SDK type usage, consumer surface audit (22 files), go.mod replace directive for CI (from #27)

## Parked Ideas

- Image tool plugins: multi-harness plugin config sections, harness detection at container start, runtime vs build-time plugin install (#058)
  Reason: deferred until a second harness beyond Claude Code is supported. RTK integration was implemented directly in capture wizard.
- Multi-backend session sharing: sharing across SSH, OpenShell, K8s backends with different networking models (#084)
  Reason: start with local sharing first. Proxy-through-host is natural next step for SSH. K8s and OpenShell need more investigation.
- Sidebar presence panel: connected users, access levels, quick actions for token/sharing management (#085)
  Reason: recommend aggregate presence panel for V1. Data available via SessionInfo. Depends on spike findings about Zellij client connection API.
- SpecKit pair programming hooks: collaboration metadata, role awareness for shared spec workflows (#086)
  Reason: existing shared Zellij session provides pair programming experience naturally. Revisit if users report friction.

## Attic

Completed brainstorms that have corresponding specs (active or attic) are in `brainstorm/attic/`. See `ls brainstorm/attic/` for the full list.

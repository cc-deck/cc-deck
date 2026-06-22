# Brainstorm: OpenShell Sandbox Resource Limits

**Date:** 2026-06-22
**Status:** active

## Problem Framing

OpenShell sandboxes default to 2 vCPUs and 2 GB RAM (hardcoded in `openshell-driver-vm`). This is insufficient for memory-intensive operations like Rust release builds with LTO (rustc alone consumes 1.1 GB for `zellij-utils`). The sandbox becomes unresponsive during these builds, with `krunkit` hitting 500%+ CPU.

The OpenShell CLI already supports `--cpu` and `--memory` flags on `openshell sandbox create`, but cc-deck's `ws new` does not expose them. Users have no way to request more resources.

## Approaches Considered

### A: Add --cpu and --memory flags to ws new (Recommended)

Add two new flags to `cc-deck ws new` for OpenShell workspaces. Pass them through to `CreateSandbox()` which adds them to the `openshell sandbox create` args.

```
cc-deck ws new dev --type openshell --image cc-deck:latest --memory 4Gi --cpu 4
```

- Pros: Simple, direct mapping to OpenShell CLI flags. Users control resources per workspace. No config file changes needed.
- Cons: Users must remember to pass flags. Defaults remain 2 vCPUs / 2 GB.

### B: Configure defaults in build.yaml manifest

Add a `resources` section to the OpenShell target in `build.yaml`:

```yaml
targets:
  openshell:
    name: cc-deck
    resources:
      cpu: "4"
      memory: 4Gi
```

The capture wizard sets sensible defaults based on detected tools (Rust projects get more RAM). CLI flags override manifest values.

- Pros: Defaults are project-aware. Heavy projects (Rust, Java) get more resources automatically. No need to remember flags.
- Cons: More complexity. Manifest schema change. Capture wizard needs heuristics.

### C: Both A + B

CLI flags for ad-hoc overrides, manifest for project defaults. CLI wins when both are set.

- Pros: Best of both. Project defaults for convenience, CLI for one-off adjustments.
- Cons: Most implementation effort. Need to document precedence.

## Decision

**Approach C (A + B combined)**: Manifest defaults with CLI overrides. The manifest captures project-level resource needs (detected during capture), and CLI flags allow per-workspace adjustments.

Priority: implement A first (CLI flags), then B (manifest defaults) as a follow-up.

## Key Requirements

- Add `--memory` and `--cpu` flags to `cc-deck ws new` (apply only to OpenShell type)
- Pass flags through `SandboxConfig` to `CreateSandbox()` to `openshell sandbox create`
- If flags are omitted and no manifest defaults, let OpenShell use its own defaults (2 vCPU / 2 GB)
- Add `resources.cpu` and `resources.memory` fields to `targets.openshell` in the manifest schema
- Manifest values are used when CLI flags are not provided
- CLI flags always override manifest values
- The capture wizard should suggest higher defaults for Rust projects (4 GB) and Java projects (4 GB)
- Log the effective resource limits at workspace creation

## Open Questions

- Should `cc-deck ws update` also support changing resource limits on an existing workspace? (OpenShell may not support live resize.)
- Should there be a global default in `config.yaml` (e.g., "always create OpenShell sandboxes with 4 GB") separate from per-project manifest defaults?
- Should `ws status` show the sandbox's resource limits?

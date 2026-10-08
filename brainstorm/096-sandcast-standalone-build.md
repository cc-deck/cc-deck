# Brainstorm: sandcast, a Standalone Sandbox Image Builder

**Date:** 2026-10-08
**Status:** active
**Related:** [053-openshell-build-integration](053-openshell-build-integration.md), [070-build-system-multi-agent](070-build-system-multi-agent.md), [072-build-skill-iteration-reduction](072-build-skill-iteration-reduction.md), [091-nemoclaw-as-base-image](091-nemoclaw-as-base-image.md), [058-image-tool-plugins](058-image-tool-plugins.md)

## Problem Framing

cc-deck contains a build system that turns a project directory into a sandbox image. It has two halves. A capture skill examines the project and the host, asks the user questions, and writes a descriptor (the manifest). A build skill turns that descriptor into a Containerfile, probes the base image for what is already present, and fixes failed builds in a retry loop. Go subcommands provide the deterministic pieces: manifest handling, base image registry, image probing with a digest cache, OpenShell policy assembly from a component catalog, the two-pass binary probe, and the podman build.

The capture-to-descriptor-to-image flow is the innovative part of cc-deck, and it is useful far beyond cc-deck. Anyone who runs an agent inside an OpenShell sandbox wants an image that mirrors their project and their tooling, with a network policy that fits. Today that capability is locked inside cc-deck:

- The build package is embedded in the cc-deck binary and its skills are copied into the project by `cc-deck build init`.
- The mandatory stack (cc-deck itself, Zellij, cc-session, cc-setup, Claude Code) is hard-wired into the templates.
- The skills call `cc-deck build probe`, `cc-deck build run` and `cc-deck build refresh`, and suggest `cc-deck ws new` as the next step.
- Only Claude Code is supported as a harness, both for running the skills and as the agent baked into the image.
- The shared types (`PolicyFile`, `Manifest`) are used by the workspace and record packages, and the manifest file name differs between the Go code, the scripts and the K8s backend.

The goal is a standalone tool, in its own repository, that offers this flow to any project and any harness. cc-deck keeps its own build package for now. Whether cc-deck later consumes the standalone tool and drops its embedded build package is a separate decision.

## Name

**sandcast.** Sand casting uses a pre-formed mold and pours the metal into it. The base image is the mold, the project is the pour. The word also reads as sandbox plus cast. Binary `sandcast`, skills `/sandcast:capture` and `/sandcast:build`.

Alternatives considered: shellsmith (strong OpenShell tie, longer), imprint (generic, likely collisions), envcast (neutral, less character).

## Agreed Shape

The following points were settled during the session before the approaches were compared.

| Dimension | Decision |
|-----------|----------|
| Relationship to cc-deck | Independent tool in a separate repo. cc-deck is not switched over now. Removal of the cc-deck build package is a possible later step. |
| Targets | OpenShell sandbox images first (sandbox user, policy at `/etc/openshell/policy.yaml`). Plain OCI images for podman as a thin second variant. No SSH or Ansible target. |
| Form factor | Go CLI plus a skill bundle. The binary can install its skill bundle into a detected harness. Skills in a portable format so they work in Claude Code, OpenCode, Codex and pi. |
| Base images | A registry of allowed base images (name, ref, default, notes) plus a live probe of the chosen image, cached by digest. No hand-maintained capability lists. |
| Composition unit | Unified components. One component carries an install recipe, a probe check, network endpoints, and filesystem and env needs. Agents, languages and shell tools are all components. |
| Discovery scope | Project scan plus host tooling and shell config. Harness-specific assets (MCP servers, plugins, skills) are discovered through pluggable harness adapters. The capture skill asks upfront which harnesses to include. Credentials stay out of the image. |

## Approaches Considered

### A: Component-driven delta builder (strict)

The Go core probes the base image, marks which components it already satisfies, and renders the missing ones into a Containerfile and an OpenShell policy from component templates. Skills own discovery and the retry loop.

- Pros: Reproducible. The same manifest and base digest give the same Containerfile and policy, so builds run in CI without an LLM. Fewest build iterations, because facts the LLM used to guess (package manager, present tools, release asset names, binary paths) come from code. Brainstorm 072 traced all 13 recorded failures to guessing. Reuses most existing Go code. Harness-agnostic by design.
- Cons: A template can only render what the component schema expresses. An odd tool the catalog has never seen needs a recipe before it can be installed. That loses the flexibility of the current build skill, which writes the layer itself and adapts.

### A refined: plan in Go, assembly by the LLM, recipes promoted over time (chosen)

Capture stays LLM-driven. The Go core produces a build plan rather than a finished Containerfile. The skill assembles the Containerfile from the plan, handles components without a recipe itself, and promotes working layers into recipes.

- Pros: Same reproducibility as strict A for known components. Same flexibility as today for unknown tools. Flexibility converts into determinism as recipes accumulate. CI never needs an LLM because fragments the LLM wrote are stored next to the manifest.
- Cons: Slightly larger surface: a plan format and a fragment store in the project directory. Two artifacts (binary and skill bundle) with version coupling.

### B: Skill-first with thin helpers

The binary only probes images and runs podman. The skill reads catalog YAML and writes the Containerfile and policy itself.

- Pros: Fastest to stand up, mostly a port of the current skills with cc-deck specifics removed. Very flexible. Almost no Go types to design.
- Cons: Not reproducible, no CI path without an LLM. Repeats the iteration problem from 072. Policy correctness becomes an LLM responsibility, which is the wrong place for a security boundary. Harness independence is harder because the logic lives entirely in skill files that differ per harness.

### C: Pre-built layer stacks

Maintain pre-built images per base, language stack and harness. The tool composes FROM chains and adds a thin project layer.

- Pros: Very fast per-project builds. Central curation and scanning. Fits the SAW and NemoClaw model where published images are consumed, not built.
- Cons: Combinatorial image matrix to build and publish. No real detection of what is missing. Host tooling and shell config replication does not fit a pre-built model. FROM chains are linear, so two stacks still need a custom build step.

## Decision

**A refined.** Capture remains the flexible, LLM-driven front end that produces the descriptor. The Go core turns the descriptor plus the probed base image into a build plan. The skill assembles and repairs the Containerfile, and feeds what it learned back into recipes. Pre-built stack images (C) are parked as a later optimization: a published image can serve as a base whose probe shows most components already satisfied.

### Pipeline

1. **Capture** (skill): scan the project for languages, services and build files. Scan the host for companion tools and shell config. Ask which harnesses to include. Run each selected harness adapter's discovery for local MCP servers, plugins and skills. Propose components. Write the manifest.
2. **Plan** (Go): probe the base image (cached by digest). Compute the component delta. Verify release assets and tarball layouts for github-release recipes. Render fragments for every component with a recipe. Assemble the OpenShell policy. List components without a recipe.
3. **Assemble** (skill): stitch the plan's fragments into a Containerfile. Write layers for components without a recipe. Store LLM-authored fragments next to the manifest.
4. **Build** (Go): podman build. For OpenShell, the two-pass build with the binary path probe and the policy stamped as an OCI label.
5. **Repair** (skill): on failure, fix the Containerfile and retry. Record each fix in a structured learnings file.
6. **Promote** (skill, with user confirmation): turn a working LLM-authored layer into a project-local component recipe. Offer to contribute it to the catalog.
7. **Verify** (Go): run every included component's probe check against the built image.

When every component has a recipe, steps 2, 4 and 7 run from the manifest alone. When some do not, the stored fragments let the binary rebuild without an LLM.

### Harness adapters

Each supported harness (Claude Code, Codex, OpenCode, pi) is an adapter with four duties:

- Discover local MCP servers, plugins and skills on the host and propose which to bake in.
- Provide the install recipe for the agent binary, with pipefail and a post-install check.
- Scaffold the agent's config inside the image (marketplaces, settings, hook registration).
- Contribute network endpoints for the agent and its inference providers.

The adapters also define where the sandcast skill bundle is installed on the host, so `sandcast skills install` can target any supported harness.

## Key Requirements

- Separate repository and Go module. No import of cc-deck packages. cc-deck's stack can later be expressed as a component set in cc-deck's own catalog.
- One manifest file name, project-local, with a documented schema and version. Targets: `openshell` and `container`.
- Base image registry file with name, ref, default flag and notes. Probe results cached by image digest, invalidated when the digest changes.
- Unified component schema: key, name, match conditions, install recipe (package, github-release, script), probe check, endpoints, binaries, filesystem and env needs. Three tiers with documented precedence: embedded, catalog repository, user and project local.
- Build plan as a stable, machine-readable artifact that the skill consumes and that users can inspect.
- Fragment store in the project directory for LLM-authored layers, so CI rebuilds without an LLM.
- Structured learnings file and a promote step that converts a working layer into a recipe.
- OpenShell policy assembly (filesystem, landlock, process, network policies) moves to sandcast, including the two-pass binary probe and the OCI policy label.
- Skill bundle in a portable format, installable by the binary into Claude Code, OpenCode, Codex and pi.
- The capture skill asks upfront which harnesses to include. Only selected harness adapters run discovery.
- Credentials are never written into the image or the manifest. They belong to the runtime that starts the sandbox.
- Non-interactive mode for plan, build and verify from an existing manifest.

## Out of Scope

- Switching cc-deck to sandcast or removing the cc-deck build package.
- SSH and Ansible provisioning.
- Pre-built layer stack images (parked).
- Running or managing sandboxes. sandcast stops at a built and verified image.

## Open Questions

- Catalog migration: should the existing `cc-deck-catalog` repository become the sandcast catalog with install recipes added, or should sandcast start a new catalog and cc-deck's catalog stay as is?
- Manifest compatibility: should sandcast read the current cc-deck `build.yaml` (version 3) as a migration path, or define its schema fresh and ship a converter?
- Build plan format: one file per build or a directory with fragments? How much of the plan is stable API for the skill versus internal?
- Fragment store lifecycle: when a promoted recipe supersedes a stored fragment, who removes the fragment, and how is drift between the two detected?
- Portable skill format: which format do OpenCode, Codex and pi agree on today, and what has to differ per harness (frontmatter, invocation name, tool permissions)?
- Harness adapter discovery of MCP servers: which configs are read (user-level, project-level) and how are secrets inside MCP server definitions kept out of the image?
- Container-target policy: plain OCI images have no OpenShell policy. Should sandcast still emit an allowed-domains list for consumers that filter egress themselves?
- Probe script portability: the current probe assumes podman. Should sandcast support docker as a fallback for users outside the Red Hat ecosystem?
- Version coupling: how does the skill bundle detect a mismatched binary version, and how does the binary refuse a plan written for a different schema?
- Promotion into the catalog: pull request flow from the project-local recipe, and who reviews recipes for supply chain concerns (verified asset names, checksums).

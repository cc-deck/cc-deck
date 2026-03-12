# Review Summary: cc-deck Build Pipeline

**Spec:** specs/018-build-manifest/spec.md | **Plan:** specs/018-build-manifest/plan.md
**Generated:** 2026-03-12

---

## Executive Summary

Developers using cc-deck need a way to create containerized Claude Code environments that mirror their local setup, including project-specific tools, Claude Code plugins, and MCP server configuration. Today, this process is entirely manual.

This feature introduces a build pipeline that follows a CLI-AI-CLI flow: the developer scaffolds a build directory with `cc-deck build init`, then uses AI-driven Claude Code commands to analyze repositories for tool dependencies, add plugins and MCP servers, and generate a Containerfile. Finally, deterministic CLI commands build and push the resulting container image.

The key innovation is the manifest file (`cc-deck-build.yaml`) which stores tool requirements as human-readable, free-form text (e.g., "Go compiler >= 1.22") that the AI resolves to concrete install commands during Containerfile generation. This separates the "what" (developer intent) from the "how" (package manager commands), making the manifest readable and maintainable by humans while leveraging AI for the tedious resolution work.

The feature builds on the base container image (spec 017) and supports both podman and docker as container runtimes. cc-deck self-embeds into the built image to ensure version consistency between the build tool and the containerized environment.

## Review Recipe (30 minutes)

### Step 1: Understand the problem (5 min)
- Read the Executive Summary above
- Skim `spec.md` User Stories 1-3 (the P1 stories that form the MVP)
- Ask yourself: Is the CLI-AI-CLI flow the right approach?

### Step 2: Check critical references (10 min)
- Review each item in the **Critical References** table below
- Focus on the manifest schema (FR-001 to FR-006) and self-embedding (FR-020)
- Check the data-model.md manifest schema for completeness

### Step 3: Evaluate technical decisions (8 min)
- Review the **Technical Decisions** section below
- Key question: Is `go:embed` with `embed.FS` the right approach for embedding multiple command files?
- Key question: Is the free-form tools approach (AI-resolved) better than structured package lists?

### Step 4: Validate coverage and risks (5 min)
- Check **Scope Boundaries**: Deploy integration (brainstorm 20) is explicitly out of scope
- Check **Risk Areas**: AI command quality is the biggest risk

### Step 5: Complete the checklist (2 min)
- Work through the **Reviewer Checklist** below

## PR Contents

| Artifact | Description |
|----------|-------------|
| `spec.md` | 6 user stories, 23 functional requirements for the build pipeline |
| `plan.md` | Go CLI + Claude Code commands, extending existing cc-deck structure |
| `tasks.md` | 31 tasks across 9 phases, MVP is 14 tasks (Phases 1-5) |
| `research.md` | go:embed patterns, manifest design, runtime detection, self-embedding |
| `data-model.md` | Full manifest schema, MCP label schema, state transitions |
| `quickstart.md` | End-to-end workflow example |
| `contracts/cli-commands.md` | CLI command contracts (args, flags, exit codes) |
| `REVIEWERS.md` | This file |

## Technical Decisions

### Free-form tools vs structured package lists
- **Chosen approach:** Tools are human-readable text resolved by AI during Containerfile generation
- **Alternatives considered:**
  - Structured package lists (e.g., `dnf: [golang, protobuf-compiler]`): rejected because it couples the manifest to a specific package manager and requires users to know exact package names
  - Language-specific version files (e.g., `.go-version`): rejected because it fragments tool specification across many files
- **Trade-off:** Flexibility and readability at the cost of determinism (AI resolution may vary)
- **Reviewer question:** Is the non-determinism of AI resolution acceptable, or should we add a lock file mechanism?

### go:embed with embed.FS for commands
- **Chosen approach:** Use `embed.FS` to embed entire `commands/` and `scripts/` directories
- **Alternatives considered:**
  - Individual `//go:embed` per file: rejected because 7+ embed directives are unwieldy
  - External download at init time: rejected because offline support is needed
- **Trade-off:** All assets baked into the binary (larger binary) but no external dependencies

### cc-deck self-embedding
- **Chosen approach:** `cc-deck build` copies its own binary into the build context, Containerfile runs `cc-deck plugin install`
- **Alternatives considered:**
  - Download cc-deck from a release URL inside the Containerfile: rejected because it breaks version consistency
  - Embed cc-deck version and download matching release: rejected because it adds network dependency during build
- **Trade-off:** Guaranteed version consistency, but the binary is ~20MB added to every build context

## Critical References

| Reference | Why it needs attention |
|-----------|----------------------|
| `spec.md` FR-001 to FR-006: Manifest schema | Defines the contract for the manifest file. Schema errors affect all downstream commands. |
| `spec.md` FR-020: cc-deck self-embedding | Complex mechanism (binary copies itself). Must work across architectures. |
| `data-model.md`: MCP Label Schema | New label convention (`cc-deck.mcp/*`) that MCP server images must follow for auto-discovery. |
| `plan.md`: Project Structure | Two `internal/build/` trees shown (likely a formatting issue, should be one). |
| `spec.md` FR-015 to FR-017: Extract command | AI-driven analysis is the most complex and least deterministic part. |

## Reviewer Checklist

### Verify
- [ ] Manifest schema in data-model.md covers all fields referenced in spec.md FR-001 to FR-006
- [ ] CLI command contracts in contracts/cli-commands.md match the spec's acceptance scenarios
- [ ] Task dependencies are correct (US3 depends on US2, US4 depends on US3, US5 is independent)
- [ ] The `.gitignore` for the build directory correctly ignores generated files but tracks the manifest

### Question
- [ ] Should the manifest support a `version: 2` migration path, or is it too early to plan for that?
- [ ] Is the `--install-zellij` flag (T031) in scope for this spec, or should it be a separate task?
- [ ] Should `/cc-deck.extract` support analyzing remote repositories (not just local checkouts)?

### Watch out for
- [ ] The plan shows `internal/build/` listed twice in the project structure (formatting artifact)
- [ ] AI commands (.md files) need to be high quality since they drive the entire analysis phase
- [ ] The `update-manifest.sh` script must handle YAML safely (no sed-based YAML manipulation)

## Scope Boundaries

**In scope:** Manifest schema, `build init`, `build`, `push`, `verify`, `diff`, AI commands (extract, plugin, mcp, containerfile, publish)

**Out of scope:** Deploy integration (compose/k8s manifest generation, covered by brainstorm 20, future spec)

## Risk Areas

| Risk | Impact | Mitigation |
|------|--------|------------|
| AI tool resolution non-determinism | Medium | Containerfile is always regenerated, user reviews before build |
| go:embed size bloat | Low | Commands are small markdown files (~1-5KB each) |
| Container runtime compatibility | Medium | Auto-detection + transparent fallback between podman and docker |
| MCP label schema adoption | Low | Labels are optional, manual entry is the fallback |

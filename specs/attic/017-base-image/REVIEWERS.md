# Review Summary: cc-deck Base Container Image

**Spec:** specs/017-base-image/spec.md | **Plan:** specs/017-base-image/plan.md
**Generated:** 2026-03-12

---

## Executive Summary

cc-deck needs a well-equipped base container image that serves as the foundation for project-specific Claude Code environments. This image is a Fedora-based developer toolbox containing 27+ CLI tools, Node.js, Python/uv, and a configured zsh shell with starship prompt.

The base image deliberately excludes Zellij, Claude Code, and cc-deck itself. These are added during user image build (spec 018) to ensure version consistency between the local cc-deck and the containerized environment.

The image supports both amd64 and arm64 architectures, is published to ghcr.io, and includes a 43-check verification test script. A GitHub Actions workflow automates multi-arch builds on push (with base-image changes) and release events.

## Review Recipe (30 minutes)

### Step 1: Understand the problem (5 min)
- Read the Executive Summary above
- Skim `spec.md` User Story 1 (core image with all tools)
- Ask yourself: Is the tool selection comprehensive enough?

### Step 2: Check critical references (10 min)
- Review the **Critical References** table below
- Focus on the tool list (FR-004 to FR-009) and shell configuration (FR-014)

### Step 3: Evaluate technical decisions (8 min)
- Review the **Technical Decisions** section below
- Key question: Is Fedora the right base OS choice?
- Key question: Is excluding Zellij/CC from the base image the right boundary?

### Step 4: Validate coverage and risks (5 min)
- Check **Scope Boundaries**: Only base image, not the build pipeline
- Verify the test script covers all FRs

### Step 5: Complete the checklist (2 min)
- Work through the **Reviewer Checklist** below

## PR Contents

| Artifact | Description |
|----------|-------------|
| `spec.md` | 4 user stories, 21 functional requirements for the base image |
| `plan.md` | Containerfile + scripts + config, top-level `base-image/` directory |
| `tasks.md` | 23 tasks across 7 phases, MVP is 9 tasks (local image build) |
| `research.md` | Fedora 41 package availability for all 30+ tools |
| `data-model.md` | Entities, tool categories, shell config, layer structure |
| `quickstart.md` | Build, verify, and usage instructions |
| `REVIEWERS.md` | This file |

## Technical Decisions

### Fedora as base OS
- **Chosen approach:** Latest stable Fedora (41) as the base image
- **Alternatives considered:**
  - Ubuntu LTS: rejected because packages are older, Fedora aligns with RHEL ecosystem
  - Alpine: rejected because musl libc causes compatibility issues with Node.js native modules
- **Trade-off:** Freshest packages but shorter OS lifecycle (need to bump version annually)

### Excluding Zellij/CC/cc-deck from base image
- **Chosen approach:** Base image is a pure toolbox, cc-deck components added during user image build
- **Alternatives considered:**
  - Include everything in base image: rejected because Claude Code updates frequently (weekly+) and cc-deck versions must match between builder and image
- **Trade-off:** Simpler base image maintenance, but user images take slightly longer to build

### Starship via GitHub release (not dnf)
- **Chosen approach:** Download starship binary from GitHub releases during build
- **Alternatives considered:**
  - COPR repository: rejected because it adds external repo trust dependency
- **Trade-off:** Simpler and more predictable, but requires network access during build

## Critical References

| Reference | Why it needs attention |
|-----------|----------------------|
| `spec.md` FR-004 to FR-009: Tool list | Defines the complete tool inventory. Missing tools affect developer experience. |
| `spec.md` FR-014: Shell configuration | zsh + starship + aliases + fzf + zoxide. Complex configuration that must work on first container start. |
| `spec.md` FR-016: Multi-arch | Both amd64 and arm64 must work identically. Some tools may have architecture-specific issues. |
| `spec.md` FR-021: Vulnerability scan | Non-blocking scan is a policy decision. Should it ever block? |

## Reviewer Checklist

### Verify
- [ ] All 27+ tools listed in spec FR-004 to FR-009 are present in install-tools.sh
- [ ] The starship.toml config includes git branch, directory, python venv, and kubernetes context modules
- [ ] The coder user has UID 1000, zsh shell, passwordless sudo, and correct npm prefix
- [ ] The GitHub Actions workflow triggers on base-image/ changes AND release events

### Question
- [ ] Should the tool list include `dust` and `hyperfine` (listed as open questions in brainstorm)?
- [ ] Should zsh-autosuggestions and zsh-syntax-highlighting be included for better interactive experience?
- [ ] Is 1.5 GB compressed a reasonable size limit, or should it be tighter?

### Watch out for
- [ ] npm requires a separate Fedora package (`nodejs-npm`), not just `nodejs`
- [ ] `runuser` must be used instead of `su` for npm config (login shell PATH issue)
- [ ] fzf segfaults under QEMU amd64 emulation on Apple Silicon (not a bug, QEMU limitation)

## Scope Boundaries

**In scope:** Base container image with tools, runtimes, shell config, multi-arch, CI pipeline, test script

**Out of scope:** Zellij, Claude Code, cc-deck (added by spec 018), deploy manifests (spec 020)

## Risk Areas

| Risk | Impact | Mitigation |
|------|--------|------------|
| Fedora version EOL | Medium | Parameterized `ARG FEDORA_VERSION`, easy to bump |
| Tool unavailable on arm64 | Low | Only starship needs GitHub release, all others via dnf |
| Image size growth | Low | Single dnf install layer, clean cache, size check in test.sh |
| npm permission issues | Medium | npm prefix at `~/.local/lib/npm`, verified in test.sh |

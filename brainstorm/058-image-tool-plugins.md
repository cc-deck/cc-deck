# Brainstorm: Image Tool Installation and Multi-Harness Plugin Config

**Date:** 2026-05-19
**Status:** parked

## Problem Framing

When building cc-deck container images, users need a way to declare additional CLI tools (like RTK) that should be installed and initialized. Some tools need post-install steps (e.g., `rtk init -g` to set up hooks). Beyond that, as cc-deck grows to support harnesses beyond Claude Code, the configuration needs harness-specific sections for plugins, hooks, and tool integrations.

Two sub-topics were identified and split:

1. **Image tool installation** (concrete, actionable now): general mechanism for declaring tools with install commands, init steps, and config in the build manifest. RTK is the first user.
2. **Multi-harness plugin config** (architectural, deferred): how to structure cc-deck config for harness-specific plugin sections when supporting Claude Code, Aider, Codex, etc.

## Decision

Split into two brainstorms. Topic 1 was resolved directly by adding RTK to the `/cc-deck.capture` Step 2 companion tools catalog with a new `post_install` manifest field. Topic 2 is deferred until a second harness is actually supported.

### Implemented (Topic 1)

- RTK added to companion tools catalog in `cc-deck.capture.md`
- Host detection: wizard runs `which <tool>` and marks detected tools with `(detected)`
- New `post_install` field in manifest tool entries for initialization commands
- RTK manifest entry: `post_install: "rtk init -g"`

## Key Requirements (for Topic 2, when revisited)

- Harness-specific config sections (e.g., `harness.claude-code.plugins`)
- Each harness declares its own tools, hooks, and init steps
- General plugin concept that maps to harness-native mechanisms (Claude Code plugins, Aider config, etc.)
- Init timing: build-time preferred for baked images

## Open Questions

- How should harness detection work at container start if multiple harnesses are installed?
- Should plugins be installable at runtime or only at build time?
- How to handle plugin config that differs between harnesses (e.g., same MCP server, different integration)?

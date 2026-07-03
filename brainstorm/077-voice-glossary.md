# Brainstorm: Voice Glossary for Whisper

**Date:** 2026-07-03
**Status:** active

## Problem Framing

Whisper frequently misrecognizes technical terms (Kubernetes, Zellij, OpenShell, gRPC, etc.) during voice dictation. Whisper supports an `initial_prompt` parameter (limited to 224 tokens / ~800 characters) that biases the decoder toward specific vocabulary. cc-deck's voice relay currently does not use this parameter. Users need a way to provide domain-specific terms, both globally and per-project, so Whisper spells them correctly.

Additionally, since cc-deck manages multiple sessions across different projects, the glossary should automatically switch when the user attends a different session, without manual intervention.

## Approaches Considered

### A: Two-tier glossary with lazy loading (Chosen)

Global glossary in `~/.config/cc-deck/config.yaml` under `voice.glossary`. Project-local glossary in `.cc-deck/voice-glossary.txt` (one term per line, committable to the repo). Relay loads the project glossary lazily on session switch using the attended session's `working_dir` (added to the plugin's state dump response). Merges global + project terms (project last for higher Whisper priority). Caches after first load.

- Pros: simple, automatic session-aware switching, no user action needed, project glossaries are shareable via git
- Cons: file I/O on first session switch per project (cached afterward)

### B: Preload all glossaries at startup

Same file locations, but scan all known workspace directories at relay startup. No file I/O during switches.

- Pros: faster runtime
- Cons: stale if glossary files change while relay is running

### C: Push glossary via pipe from plugin

Plugin reads the glossary and pushes it to the relay via a new pipe message.

- Pros: no file access from relay
- Cons: more protocol changes, WASI filesystem limitations in the plugin, overcomplicates a simple feature

## Decision

Approach A: Two-tier glossary with lazy loading. The relay uses the attended session's `working_dir` (from the plugin state dump) to find and cache project glossaries. Terms are merged (global first, project last) and passed as the `prompt` form field to whisper-server.

## Key Requirements

- Global glossary: `voice.glossary` list in cc-deck config YAML
- Project glossary: `.cc-deck/voice-glossary.txt` in the project root (one term per line)
- Plugin state dump must include `working_dir` for the attended session
- Relay caches project glossaries keyed by directory path
- Merged prompt: global terms first, project terms last (project terms get higher Whisper priority in the 224-token window)
- Pass merged list as `prompt` field in the multipart form to whisper-server `/inference`
- Warn at startup if the global glossary alone exceeds the token limit
- When no project glossary exists, use only the global glossary
- When no glossary is configured at all, omit the prompt field (current behavior)
- Merged glossary MUST contain only unique terms (deduplicate across global and project lists, case-insensitive)

## Open Questions

- Should the relay warn when the merged glossary exceeds ~224 tokens, or silently truncate? (Whisper truncates silently anyway, but the user might want to know)
- Should `.cc-deck/voice-glossary.txt` support comments (lines starting with #)?

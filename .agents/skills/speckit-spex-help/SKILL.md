---
name: speckit-spex-help
description: Quick reference for all spex commands and workflow
compatibility: Requires spec-kit project structure with .specify/ directory
metadata:
  author: github-spec-kit
  source: spex:commands/speckit.spex.help.md
---

# spex Help

## Overview

Display the spex quick reference with workflow diagram, command list, and guidance.

## Behavior

1. Locate and read the quick reference. Try these paths in order (first match wins):
   - `.specify/extensions/spex/docs/help.md` (project installation)
   - `spex/extensions/spex/docs/help.md` (development repo)
   - `spex/docs/help.md` (legacy location)
2. Display the content exactly as written
3. Ask: "Any questions about the spex workflow? I can explain any command in detail."

## Key Principles

- **Reference mode is fast**: Just display the help content
- **Non-pushy**: Offer options, don't force workflows
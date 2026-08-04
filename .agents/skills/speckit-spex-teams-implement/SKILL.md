---
name: speckit-spex-teams-implement
description: Parallel implementation via agent teams for independent tasks
compatibility: Requires spec-kit project structure with .specify/ directory
metadata:
  author: github-spec-kit
  source: spex-teams:commands/speckit.spex-teams.implement.md
---

# Teams Implement

Standalone parallel implementation command. The ship pipeline routes to this command
(instead of standard implement) when spex-teams is enabled and 2+ independent tasks exist.

## Prerequisites

- {harness:teams-enabled}
- tasks.md must exist with task breakdown
- spex-gates extension must be enabled

## Execution

1. Read tasks.md and parse the task list
2. Count tasks marked with [P] (parallel-eligible)
3. If 2+ independent tasks: invoke speckit.spex-teams.orchestrate for parallel agent spawning
4. If <2 independent tasks: fall back to standard implement (inform user)
5. Check .spex-state for autonomous mode; suppress prompts if in ship pipeline

## Ship Pipeline Integration

The ship command routes here when spex-teams is enabled and tasks.md has 2+ independent tasks.
This command is NOT invoked via a hook - it is called directly by ship or the user.
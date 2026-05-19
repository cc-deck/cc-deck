# 057: Autonomous Task Dispatch

**Date**: 2026-05-18
**Status**: brainstorm
**Feature**: Fire-and-forget task dispatch to containerized AI agents
**Inspired by**: [paude](https://github.com/bbrowning/paude) v0.15.0 (Ben Browning)

## Problem

cc-deck currently operates in an interactive model: the user sits in a Zellij session, watches the agent work, and interacts in real time. This is ideal for exploratory development and complex tasks that need human judgment.

However, many development tasks are well-defined and can run autonomously: "write tests for this module", "fix this lint error", "refactor this function to use the new API", "update dependencies". For these, the interactive model is wasteful. The user waits or context-switches while the agent churns through predictable work.

paude demonstrates that a fire-and-forget model works well for these tasks. The user dispatches a task, the agent works in an isolated container, and the user harvests the results later via git. cc-deck could offer both modes: interactive (current) and autonomous (new).

## paude's Approach

### Task Dispatch

Tasks are passed to agents via the `-a` flag or an `PAUDE_AGENT_ARGS` environment variable. The agent receives the task as its initial prompt and begins working immediately without human interaction.

```bash
# Dispatch a task to a new session
paude create my-task -a "Write unit tests for the auth module"

# Dispatch with YOLO mode (safe because network-filtered)
paude create my-task --yolo -a "Refactor database.py to use async"
```

### Session Lifecycle

1. **Create**: Container starts, agent launches with task prompt
2. **Work**: Agent works autonomously (YOLO mode, no permission prompts)
3. **Monitor**: `paude status` shows activity, state, and work summary
4. **Harvest**: `paude harvest my-task --branch feat/auth-tests --pr`
5. **Reset**: `paude reset my-task` clears workspace for next task

### Work Summary Detection

paude's `status_sessions()` enriches running sessions with work summaries by executing commands inside the container. This gives users visibility into what the agent accomplished without connecting to the session:

- Current git branch
- Number of commits ahead of base
- Recent commit messages
- File change statistics

### Batch Orchestration

paude supports running multiple sessions in parallel, each working on a different task. The `status` command shows all sessions with their progress. The `harvest` command can target individual sessions.

## Adaptation for cc-deck

### Two Operating Modes

cc-deck would support both interactive and autonomous modes through the same workspace infrastructure:

**Interactive mode** (current): User launches a Zellij session, works alongside the agent, sees real-time status in the sidebar. Best for: exploration, complex tasks, learning, debugging.

**Autonomous mode** (new): User dispatches a task, cc-deck creates a workspace, starts the agent with the task prompt, and returns control immediately. The user checks status and harvests results later. Best for: well-defined tasks, batch processing, overnight runs.

```bash
# Interactive (current behavior)
cc-deck ws start my-project

# Autonomous dispatch
cc-deck dispatch my-project "Write integration tests for the API layer"

# Autonomous with options
cc-deck dispatch my-project \
  --task "Refactor error handling to use typed errors" \
  --branch feat/typed-errors \
  --auto-harvest \
  --notify-on-done
```

### Workspace Reuse

Unlike paude (which creates a new container per task), cc-deck's workspace model allows reusing existing workspaces. A dispatch command could target a running or stopped workspace:

```bash
# Dispatch to an existing workspace
cc-deck dispatch my-project --task "Fix the failing CI tests"

# Dispatch to a new temporary workspace
cc-deck dispatch --ephemeral --task "Audit dependencies for CVEs"
```

### Status and Monitoring

The sidebar plugin already tracks session states. For autonomous sessions, add:

- A visual indicator distinguishing autonomous vs interactive sessions (e.g., a robot icon or "auto" label)
- Work summary in the sidebar (commits ahead, recent commit message)
- Elapsed time since dispatch
- Completion detection (agent idle for N minutes, or agent explicitly signals done)

```
  my-project       [auto] Done 3m    feat/typed-errors +4 commits
  api-tests        [auto] Working    feat/api-tests    +1 commit
  frontend         [live] Permission main
```

### Completion Detection

Detecting when an autonomous agent is "done" is non-trivial. Approaches:

1. **Idle timeout**: If the agent has been idle for N minutes (no tool calls, no output), consider it done. Configurable, default 5 minutes.
2. **Exit detection**: Agent process exits (for CLI agents that terminate after task completion).
3. **Commit pattern**: Agent creates a commit with a conventional message (e.g., "Task complete: ...").
4. **Hook signal**: Agent emits a `Done` or `SessionEnd` hook event.

cc-deck should support all four, with hook-based detection as the primary signal and idle timeout as the fallback.

### Auto-Harvest

When `--auto-harvest` is specified, cc-deck automatically harvests results when the agent completes:

1. Detect completion (via hook, idle timeout, or exit)
2. Create a local branch from the agent's commits
3. Optionally create a PR (`--auto-pr`)
4. Optionally notify the user (`--notify-on-done` via desktop notification or Zellij notification)
5. Optionally reset the workspace for the next task (`--auto-reset`)

### Batch Dispatch

For processing multiple tasks in parallel:

```bash
# Dispatch multiple tasks to separate workspaces
cc-deck dispatch batch \
  --task "Write tests for auth" --ws auth-tests \
  --task "Refactor DB layer" --ws db-refactor \
  --task "Update API docs" --ws api-docs

# Dispatch from a task file
cc-deck dispatch batch --file tasks.yaml
```

Task file format:

```yaml
tasks:
  - name: auth-tests
    workspace: my-project
    task: "Write comprehensive unit tests for the auth module"
    branch: feat/auth-tests
    auto-harvest: true
    auto-pr: true

  - name: db-refactor
    workspace: my-project
    task: "Refactor database.py to use async/await patterns"
    branch: feat/async-db
    auto-harvest: true
```

### Integration with Existing Features

- **Network filtering** (brainstorm 22): Autonomous sessions should always run with network filtering enabled. YOLO mode is safe only when combined with domain restrictions.
- **Git harvest** (brainstorm 026): Autonomous dispatch is the primary use case for git harvest. The dispatch command should set up the ext:: remote automatically.
- **Multi-agent** (brainstorm 022): Different tasks could be dispatched to different agent types based on the task nature. "Write Python tests" could go to Claude Code, "Update documentation" to Gemini CLI.
- **Session save/restore**: Autonomous sessions should be restorable if the host machine reboots.

## Comparison with paude

| Aspect | paude | cc-deck (proposed) |
|---|---|---|
| Primary model | Fire-and-forget only | Both interactive and autonomous |
| Container lifecycle | One container per task | Reusable workspaces |
| Task dispatch | CLI flag `-a` | Dedicated `dispatch` subcommand |
| Monitoring | `paude status` CLI | Sidebar plugin + CLI |
| Harvest | Manual `paude harvest` | Manual or auto-harvest |
| Batch | Run multiple `paude create` | First-class `dispatch batch` |
| Completion | Not explicitly detected | Multi-strategy detection |
| PR creation | `harvest --pr` | Auto-PR on completion |
| Agent support | 5 agents | Extensible via agent adapters |

## Open Questions

1. Should autonomous dispatch create a new Zellij pane (invisible but trackable), or run entirely outside Zellij?
2. How should the user interact with a running autonomous session if they want to intervene? Convert to interactive mode?
3. Should auto-harvest require the workspace to be in a clean state (no uncommitted changes), or should it stash/commit automatically?
4. What is the right default idle timeout for completion detection? 5 minutes? 10 minutes? Should it be agent-specific?
5. Should batch dispatch reuse a single workspace (sequential tasks) or create separate workspaces (parallel tasks)?
6. How should dispatch handle tasks that require user input (permission prompts)? Fail fast, skip, or queue for later?
7. Should there be a cost/token budget per autonomous task to prevent runaway spending?
8. How should dispatch integrate with CI/CD? Could `cc-deck dispatch` be used in GitHub Actions to run agent tasks as part of a pipeline?

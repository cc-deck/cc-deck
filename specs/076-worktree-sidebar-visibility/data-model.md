# Data Model: Worktree Sidebar Visibility

## Modified Entity: Session

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `in_worktree` | `bool` | `false` | Whether the session is operating inside a `.claude/worktrees/` directory |

**Relationships**: None (standalone field on existing Session struct).

**Lifecycle**: Set to `true` when CWD changes to a `.claude/worktrees/` path, set to `false` when CWD changes to any other path. Persisted via serde serialization to session cache.

## Modified Logic: CWD Filter

| Input | `is_worktree_cwd` | `working_dir` updated | `in_worktree` |
|-------|--------------------|-----------------------|---------------|
| `/home/user/project/.claude/worktrees/076-fix/` | `false` (allowed) | Yes | `true` |
| `/home/user/project/.claude/settings.json` | `true` (blocked) | No | unchanged |
| `/home/user/project/src/` | `false` (allowed) | Yes | `false` |

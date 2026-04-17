# Data Model: Remote Workspace Repository Provisioning

**Branch**: `038-workspace-repos` | **Date**: 2026-04-17

## Entities

### RepoEntry (NEW)

A git repository to be cloned into the workspace during environment creation.

| Field    | Type   | Required | Default                     | Description                              |
|----------|--------|----------|-----------------------------|------------------------------------------|
| `url`    | string | yes      | -                           | Git remote URL (HTTPS or SSH format)     |
| `branch` | string | no       | default branch              | Branch to check out after cloning        |
| `target` | string | no       | repo name from URL          | Target directory name under workspace    |

**YAML representation**:
```yaml
repos:
  - url: "https://github.com/example/repo.git"
    branch: develop
    target: my-project
  - url: "git@github.com:example/other.git"
```

**Go struct** (new file `cc-deck/internal/env/repos.go`):
```go
type RepoEntry struct {
    URL    string `yaml:"url"`
    Branch string `yaml:"branch,omitempty"`
    Target string `yaml:"target,omitempty"`
}
```

**Validation rules**:
- `url` must be non-empty
- `url` must be a valid git URL (HTTPS or SSH format)
- `target` must not contain path separators (repos are siblings, no nesting)
- `target` defaults to the last path component of the URL without `.git` suffix

### EnvironmentDefinition (MODIFIED)

Add `Repos` field to existing struct in `cc-deck/internal/env/definition.go`:

```go
Repos []RepoEntry `yaml:"repos,omitempty"`
```

**Position in YAML**: After `workspace`, before `namespace`:
```yaml
name: my-env
type: ssh
host: user@dev.example.com
workspace: ~/workspace
repos:
  - url: "https://github.com/example/repo.git"
    branch: main
```

### Profile (UNCHANGED)

Uses existing fields, no modifications:

| Field                 | Type              | Description                                    |
|-----------------------|-------------------|------------------------------------------------|
| `git_credential_type` | GitCredentialType | `ssh` or `token`                               |
| `git_credential_secret` | string          | Reference to env var or K8s Secret with token  |

### ssh.Client (MODIFIED)

Add agent forwarding support to `cc-deck/internal/ssh/client.go`:

```go
type Client struct {
    Host            string
    Port            int
    IdentityFile    string
    JumpHost        string
    SSHConfig       string
    AgentForwarding bool  // NEW: enables -A flag for SSH agent forwarding
}
```

## Internal Types (Not Persisted)

### CommandRunner

Function type for executing commands on the remote target. Used by the repo cloning logic to abstract over SSH, podman exec, and kubectl exec.

```go
type CommandRunner func(ctx context.Context, cmd string) (string, error)
```

### GitCredentials

Resolved credential configuration for git operations. Built from the active Profile via `resolveGitCredentials()`.

```go
type GitCredentials struct {
    Type  GitCredentialType // ssh or token
    Token string           // resolved token value (only for token type)
}

func resolveGitCredentials(credType GitCredentialType, credSecret string) (*GitCredentials, error)
```

Resolution logic:
- `credType == "ssh"`: returns `&GitCredentials{Type: "ssh"}` (no token needed; agent forwarding handled at SSH client level)
- `credType == "token"`: resolves `credSecret` as env var name or K8s Secret reference, returns `&GitCredentials{Type: "token", Token: resolved}`
- Empty/unconfigured: returns nil (cloning proceeds without auth)

### RepoCloneResult

Result of a single repo clone operation. Used internally for reporting.

```go
type RepoCloneResult struct {
    Entry   RepoEntry
    Success bool
    Message string
}
```

## Relationships

```
EnvironmentDefinition
  └── repos: []RepoEntry (0..N)

Profile
  └── git_credential_type: GitCredentialType
  └── git_credential_secret: string
       └── resolveGitCredentials() → GitCredentials

Environment.Create()
  └── resolveGitCredentials(profile.GitCredentialType, profile.GitCredentialSecret)
  └── calls cloneRepos(runner, repos, workspace, creds, extraRemotes)
       └── uses CommandRunner (per env type)
       └── extraRemotes: map[string]string (from auto-detection, not part of RepoEntry)
       └── produces []RepoCloneResult
```

## State Transitions

Repo cloning has no persistent state of its own. Idempotency is achieved by checking directory existence on the remote:

1. `env create` called with repos
2. For each repo: check if `<workspace>/<target>/` exists on remote
3. If exists: skip (log message)
4. If not: clone, optionally set branch, add extra remotes, clean token from URL
5. Clone failures are warnings, not errors

# 032: SSH Remote Execution Environment

**Date**: 2026-04-05
**Status**: Brainstorm

## Motivation

Run Claude Code on a remote satellite system (e.g., Hetzner VM, cloud dev box) that stays running permanently without requiring the user to remain connected. The user connects via SSH, works within a remote Zellij session, and detaches when done. The remote session continues autonomously.

This fills a gap between `local` (everything on host) and `container` (isolated, ephemeral). SSH targets persistent remote machines with their own storage, network, and lifecycle.

## Core Concept

The `ssh` environment type is the remote analog of `local`. Where `local` manages Zellij sessions on the host, `ssh` manages Zellij sessions on a remote machine over SSH. The remote machine runs Zellij, Claude Code, and the cc-deck plugin independently.

```
┌─────────────────┐         SSH          ┌─────────────────────┐
│  Local machine   │ ──────────────────> │  Remote machine      │
│                  │                      │                      │
│  cc-deck CLI     │   attach/detach     │  Zellij server       │
│  (orchestrator)  │ <────────────────── │  ├── cc-deck plugin  │
│                  │                      │  ├── Claude Code     │
│                  │   exec/push/pull    │  └── workspace/      │
│                  │ ──────────────────> │                      │
└─────────────────┘                      └─────────────────────┘
```

## Environment Definition

```yaml
# ~/.config/cc-deck/environments.yaml
version: 1
environments:
  - name: hetzner-dev
    type: ssh
    host: dev@10.0.0.42          # Required: user@host
    ssh-config: ~/.ssh/config    # Optional: explicit config file (default: system SSH config)
    port: 22                     # Optional: override SSH port
    identity-file: ~/.ssh/id_ed25519  # Optional: override key
    jump-host: bastion@jump.example.com  # Optional: ProxyJump equivalent
    workspace: /home/dev/work    # Optional: remote working directory (default: ~)
    auth: auto                   # Credential forwarding mode (auto|api|vertex|bedrock|none)
    credentials:                 # Optional: explicit credential env vars to forward
      - ANTHROPIC_API_KEY
      - GOOGLE_APPLICATION_CREDENTIALS
    env:                         # Optional: arbitrary env vars set on remote
      CLAUDE_MODEL: claude-sonnet-4-20250514
```

The SSH connection respects `~/.ssh/config` by default. Definition fields like `port`, `identity-file`, and `jump-host` are overrides for cases where the user does not want to (or cannot) modify their SSH config.

## Interface Implementation

### Supported Operations

| Method | SSH Behavior |
|--------|-------------|
| `Create` | Pre-flight checklist, bootstrap remote, register in state store |
| `Start` | No-op with warning ("remote machine lifecycle is managed externally") |
| `Stop` | No-op with warning (same reason) |
| `Delete` | Remove state record, optionally kill remote Zellij session |
| `Status` | SSH to remote, check Zellij session status, read pane map |
| `Attach` | SSH to remote, attach to remote Zellij session |
| `Exec` | Run command on remote via `ssh user@host cmd` |
| `Push` | `rsync` local files to remote workspace |
| `Pull` | `rsync` remote files to local |
| `Harvest` | Git operations: fetch commits from remote repo |

### Start/Stop Semantics

The remote machine is assumed to be always-on and managed externally (cloud provider console, systemd, etc.). `Start` and `Stop` print a warning explaining that machine lifecycle is outside cc-deck scope. They do NOT fail, they return nil after the warning.

If the user wants to manage the remote Zellij session specifically:
- Creating the Zellij session happens during `Attach` (like `local`)
- Killing the remote Zellij session can be done via `Exec` or as part of `Delete --force`

### Attach Flow

```
1. Check $ZELLIJ (refuse if already inside Zellij)
2. Update LastAttached timestamp in state store
3. Build SSH command: ssh [opts] user@host
4. On remote: zellij attach --create-background cc-deck-<name> --layout cc-deck
5. On remote: zellij attach cc-deck-<name>
6. syscall.Exec("ssh", ...) to replace process
   → user is now in remote Zellij
   → Ctrl+o d detaches from Zellij, SSH connection closes
   → remote Zellij keeps running
```

The SSH command runs `zellij attach cc-deck-<name>` on the remote. If the session does not exist, it is created with the cc-deck layout first. This is a single SSH invocation using a shell snippet:

```bash
ssh user@host 'zellij attach --create-background cc-deck-hetzner-dev --layout cc-deck 2>/dev/null; zellij attach cc-deck-hetzner-dev'
```

### Status (Always Live)

Status always queries the remote machine over SSH. No caching.

```
1. ssh user@host 'zellij list-sessions -n'
2. Parse output for cc-deck-<name> session
3. If found: state=running, read pane map for session details
4. If not found: state=unknown
5. If SSH fails: state=error, message=connection details
```

This is slower than local status but accurate. The user accepts the latency trade-off for a remote environment.

### Exec

```bash
ssh user@host 'cd /home/dev/work && <command>'
```

Runs in the configured workspace directory. Non-interactive by default. For interactive commands, allocate a PTY with `ssh -t`.

### Push/Pull (rsync)

```bash
# Push
rsync -avz --exclude='.git' local/path/ user@host:/home/dev/work/

# Pull
rsync -avz --exclude='.git' user@host:/home/dev/work/ local/path/
```

Uses `rsync` over SSH. Respects `SyncOpts.Excludes`. Falls back to `scp` if `rsync` is not available on either end (pre-flight checks this).

### Harvest (Git over SSH)

```bash
# On remote: ensure commits are pushed to a branch
ssh user@host 'cd /workspace && git push origin feature-branch'

# Locally: fetch and create PR
git fetch origin feature-branch
# Optionally: gh pr create
```

Alternatively, for repos not connected to a shared remote:
```bash
# Pull commits via git bundle
ssh user@host 'cd /workspace && git bundle create /tmp/harvest.bundle main..HEAD'
scp user@host:/tmp/harvest.bundle .
git bundle unbundle harvest.bundle
```

## Pre-flight Checklist (Create)

The `Create` operation runs an interactive pre-flight checklist. Each step checks a prerequisite, reports pass/fail, and offers to fix failures.

```
Pre-flight checks for SSH environment "hetzner-dev"

  [1/7] SSH connectivity .............. ✓ connected (dev@10.0.0.42)
  [2/7] OS/architecture detection ..... ✓ linux/amd64 (Fedora 41)
  [3/7] Zellij installed .............. ✗ not found
        → Install zellij? [Y/n/skip]
        → Installing... ✓ zellij 0.43.2
  [4/7] Claude Code installed ......... ✗ not found
        → Install claude? [Y/n/skip]
        → Installing... ✓ claude 1.0.16
  [5/7] cc-deck CLI installed ......... ✗ not found
        → Install cc-deck? [Y/n/skip]
        → Installing... ✓ cc-deck 0.8.0
  [6/7] cc-deck plugin ............... ✗ not installed
        → Running cc-deck plugin install... ✓ cc_deck.wasm (v0.8.0)
  [7/7] Credentials .................. ✓ ANTHROPIC_API_KEY forwarded

  Environment "hetzner-dev" created successfully.
```

### Check Details

**1. SSH connectivity**: `ssh -o ConnectTimeout=10 -o BatchMode=yes user@host 'echo ok'`. Verifies the connection works without interactive prompts. If it fails, show the SSH error and suggest checking keys/config.

**2. OS/architecture**: `ssh user@host 'uname -sm'`. Needed to select the right binaries. Supported: `linux/amd64`, `linux/arm64`. Others: warn and skip installation offers.

**3. Zellij**: `ssh user@host 'which zellij && zellij --version'`. If missing, offer to install. Installation method depends on OS:
- Download binary from GitHub releases for the detected arch
- `scp` to remote or `ssh user@host 'curl -fsSL ... | sh'`

**4. Claude Code**: `ssh user@host 'which claude && claude --version'`. If missing, offer to install via the official installer script.

**5. cc-deck CLI**: `ssh user@host 'which cc-deck && cc-deck version'`. If missing, offer to install. Download the right binary from the latest release for the detected OS/arch.

**6. cc-deck plugin**: `ssh user@host 'cc-deck plugin status'`. Checks plugin is installed and version matches the remote cc-deck version. If wrong version or missing, run `ssh user@host 'cc-deck plugin install'`.

**7. Credentials**: Verify that the configured auth mode works. For `auto`, detect which credentials are available locally and test that they can be forwarded. For `vertex`, check that `GOOGLE_APPLICATION_CREDENTIALS` or `gcloud` auth is available.

### Skip and Manual Install

If the user skips an installation step, show what needs to be done manually:

```
  [3/7] Zellij installed .............. ✗ not found
        → Install zellij? [Y/n/skip] skip
        ℹ Install manually on the remote:
          curl -fsSL https://github.com/zellij-org/zellij/releases/download/v0.43.2/zellij-x86_64-unknown-linux-musl.tar.gz | tar xz -C /usr/local/bin/
        Press Enter when ready, or 's' to skip remaining checks...
```

The "Press Enter when ready" flow allows the user to install in a parallel SSH session and then continue the checklist.

## Credential Forwarding

Credentials are forwarded as environment variables via SSH. The `auth` field controls detection:

| Mode | Behavior |
|------|----------|
| `auto` | Detect from local environment: check for `ANTHROPIC_API_KEY`, `GOOGLE_APPLICATION_CREDENTIALS`, AWS credential chain |
| `api` | Forward `ANTHROPIC_API_KEY` |
| `vertex` | Forward `GOOGLE_APPLICATION_CREDENTIALS`, `GOOGLE_CLOUD_PROJECT`, `GOOGLE_CLOUD_REGION` |
| `bedrock` | Forward `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_REGION` |
| `none` | No credential forwarding (credentials already on remote) |

Implementation: use `ssh -o SendEnv=...` combined with the remote sshd `AcceptEnv` config, or pass via command prefix:

```bash
ssh user@host 'ANTHROPIC_API_KEY=sk-... zellij attach cc-deck-hetzner-dev'
```

The command-prefix approach is more reliable since it does not require sshd configuration changes. Credentials are read from the local environment at connection time.

**Security note**: Credentials appear in the process list on the remote briefly. For sensitive deployments, consider writing credentials to a temporary file on the remote and sourcing it:

```bash
ssh user@host 'cat > /tmp/.cc-deck-creds && chmod 600 /tmp/.cc-deck-creds && source /tmp/.cc-deck-creds && rm /tmp/.cc-deck-creds && zellij attach ...'
```

## SSH Helper Package

New internal package: `internal/ssh`

```go
package ssh

// Client wraps SSH operations for a remote host.
type Client struct {
    Host         string   // user@host
    Port         int      // SSH port (0 = default 22)
    IdentityFile string   // Key file override
    JumpHost     string   // ProxyJump
    ConfigFile   string   // SSH config file override
    Env          map[string]string // Env vars to set on remote
}

// Run executes a command on the remote host and returns output.
func (c *Client) Run(ctx context.Context, cmd string) (string, error)

// RunInteractive replaces the current process with an interactive SSH session.
func (c *Client) RunInteractive(cmd string) error

// Upload copies a local file/directory to the remote host via rsync.
func (c *Client) Upload(ctx context.Context, localPath, remotePath string, excludes []string) error

// Download copies a remote file/directory to the local host via rsync.
func (c *Client) Download(ctx context.Context, remotePath, localPath string, excludes []string) error

// Check tests SSH connectivity. Returns nil if connection succeeds.
func (c *Client) Check(ctx context.Context) error

// RemoteInfo returns OS and architecture of the remote host.
func (c *Client) RemoteInfo(ctx context.Context) (os string, arch string, err error)
```

All methods build SSH commands using the system `ssh` binary (not a Go SSH library). This ensures full compatibility with the user's SSH config, agent, keys, and jump hosts.

## State & Types

### New Type

```go
// In types.go
const EnvironmentTypeSSH EnvironmentType = "ssh"

// SSHFields holds SSH-specific fields for state records.
type SSHFields struct {
    Host         string `yaml:"host"`
    Port         int    `yaml:"port,omitempty"`
    IdentityFile string `yaml:"identity_file,omitempty"`
    JumpHost     string `yaml:"jump_host,omitempty"`
    Workspace    string `yaml:"workspace,omitempty"`
}
```

### EnvironmentInstance Extension

```go
type EnvironmentInstance struct {
    // ... existing fields ...
    SSH *SSHFields `yaml:"ssh,omitempty"`
}
```

### Factory Update

```go
case EnvironmentTypeSSH:
    return &SSHEnvironment{name: name, store: store, defs: defs}, nil
```

## New Error

```go
var ErrSSHNotFound = errors.New("ssh binary not found in PATH")
```

## Definition Fields

```go
// In EnvironmentDefinition
type EnvironmentDefinition struct {
    // ... existing fields ...
    Host         string `yaml:"host,omitempty"`          // SSH: user@host
    Port         int    `yaml:"port,omitempty"`          // SSH: port override
    IdentityFile string `yaml:"identity-file,omitempty"` // SSH: key override
    JumpHost     string `yaml:"jump-host,omitempty"`     // SSH: ProxyJump
    Workspace    string `yaml:"workspace,omitempty"`     // SSH: remote working directory
}
```

## Reconciliation

Unlike `local` which reconciles against Zellij session list, SSH environments reconcile by querying the remote:

```go
func ReconcileSSHEnvs(store *FileStateStore, defs *DefinitionStore) error {
    // For each SSH environment in store:
    //   1. Build SSH client from definition
    //   2. Run: ssh user@host 'zellij list-sessions -n'
    //   3. Update state based on result
    //   4. If SSH fails: mark as error
}
```

This is called during `cc-deck env list` and `cc-deck env status`. Since it involves network calls, it may be slow. Consider:
- Timeout per host (5 seconds)
- Parallel reconciliation across multiple SSH environments
- A `--cached` flag to skip live checks

## File Layout

```
cc-deck/internal/
├── env/
│   ├── ssh.go           # SSHEnvironment implementation
│   └── ssh_test.go      # Unit tests
├── ssh/
│   ├── client.go        # SSH client wrapper
│   ├── client_test.go   # Tests
│   ├── bootstrap.go     # Pre-flight checks and installation
│   └── bootstrap_test.go
```

## Open Questions

1. **Credential persistence on remote**: Should credentials be persisted in the remote Zellij session environment so they survive detach/reattach? Or re-injected on every attach? Persisting is more convenient but less secure. Re-injecting is safer but requires the local machine to have credentials available.

2. **Multi-session support**: The `local` environment maps 1:1 with a Zellij session. Should `ssh` support multiple Zellij sessions on the same remote host (one per environment name)? The current design does this via `cc-deck-<name>` session naming.

3. **Port forwarding**: Should the definition support SSH port forwarding (`-L`, `-R`) for accessing remote services (e.g., a web app Claude is building)? This would be useful but adds complexity.

4. **Connection multiplexing**: Use SSH ControlMaster to keep a persistent connection and speed up repeated SSH calls (status checks, exec)? This is a performance optimization that could be added later.

5. **Remote hook integration**: The local cc-deck plugin receives hooks via pipes. On the remote, hooks fire within the remote Zellij session. Should there be a mechanism to forward hook events back to a local cc-deck instance for monitoring? This would enable a "remote dashboard" use case.

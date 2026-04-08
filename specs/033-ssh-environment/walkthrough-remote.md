# Walkthrough: SSH Environment on a Remote Linux Host

**Target:** `ningaloo` (Linux aarch64, Zellij pre-installed)
**Feature branch:** `033-ssh-environment`

This walkthrough validates every user story from the SSH environment specification against a real remote machine. Each section includes the command to run, what to observe, and the expected outcome.

## Prerequisites

Before starting, verify the following:

1. The cc-deck CLI is built and installed from the feature branch:

```bash
cd cc-deck && make install
```

2. SSH access to the target host works without a password prompt:

```bash
ssh ningaloo "echo ok"
```

3. The `ANTHROPIC_API_KEY` environment variable is set locally (or another auth mode is configured):

```bash
echo $ANTHROPIC_API_KEY | head -c 10
```

4. No existing cc-deck SSH environment named `remote-test` is registered:

```bash
cc-deck env list
```

## Step 1: Define the Environment

Add an SSH environment definition to the environments file. You can use the definition file directly or pass CLI flags during creation.

**Option A: Definition file**

```bash
cat >> ~/.config/cc-deck/environments.yaml << 'EOF'
  - name: remote-test
    type: ssh
    host: root@ningaloo
    workspace: ~/workspace
    auth: auto
EOF
```

**Option B: CLI flags only (skip the definition file)**

We will use CLI flags in Step 2 instead.

**Expected:** The definition file contains an entry for `remote-test` with type `ssh`.

## Step 2: Create the Environment (Pre-flight Bootstrap)

This step validates **User Story 1** (connect) and **User Story 2** (pre-flight bootstrap).

```bash
cc-deck env create remote-test --type ssh --host root@ningaloo --workspace ~/workspace --auth auto
```

**Observe:**

- Pre-flight checks run in sequence (7 checks).
- SSH connectivity check passes (ningaloo is reachable).
- OS/architecture detection reports `linux aarch64`.
- Zellij check passes (ningaloo has Zellij installed).
- Claude Code check reports MISSING and offers installation. Choose `y` to install, `n` to skip, or `m` for manual instructions.
- cc-deck CLI check reports MISSING. Same prompt.
- Plugin check reports MISSING. Same prompt.
- Credential verification passes.

**Expected:** Environment `remote-test` created successfully. The state store contains an SSH instance.

```bash
cc-deck env list
```

The output should show `remote-test` with type `ssh` and status `running`.

## Step 3: Attach and Detach

This step validates **User Story 1** (attach/detach cycle).

### Attach

```bash
cc-deck attach remote-test
```

**Observe:**

- Credentials are written to the remote at `~/.config/cc-deck/credentials.env`.
- A Zellij session named `cc-deck-remote-test` is created on ningaloo with the `cc-deck` layout.
- The terminal switches to the remote Zellij session.
- You can open panes, run commands, and interact normally.

### Detach

Press `Ctrl+o d` to detach from Zellij.

**Observe:**

- The SSH connection closes.
- You return to your local terminal.
- The remote Zellij session continues running.

### Verify remote session persists

```bash
ssh ningaloo "zellij list-sessions -n"
```

**Expected:** The output includes `cc-deck-remote-test` (not marked as EXITED).

### Nested Zellij guard

If you are already inside a local Zellij session:

```bash
cc-deck attach remote-test
```

**Expected:** A warning message about nested Zellij sessions. The attach is refused.

## Step 4: Check Status

This step validates **User Story 4** (status and monitoring).

```bash
cc-deck status remote-test
```

**Expected:** Status shows `running` with environment details.

```bash
cc-deck status remote-test -o json
```

**Expected:** JSON output with `"state": "running"`.

### Status in the list view

```bash
cc-deck env list
```

**Expected:** `remote-test` appears with status `running` alongside any local or container environments.

## Step 5: Credential Refresh

This step validates **User Story 3** (credential forwarding) and **User Story 5** (refresh without attaching).

### Verify credentials on the remote

```bash
ssh ningaloo "cat ~/.config/cc-deck/credentials.env"
```

**Expected:** The file contains `export ANTHROPIC_API_KEY=...` (or the appropriate auth mode variables).

### Refresh credentials

Change a local credential value temporarily and push it:

```bash
# Note the current key
echo $ANTHROPIC_API_KEY | head -c 20

# Refresh
cc-deck env refresh-creds remote-test
```

**Expected:** Output confirms credentials were refreshed. The remote credential file is updated.

### Verify the refresh

```bash
ssh ningaloo "cat ~/.config/cc-deck/credentials.env"
```

**Expected:** The credential file contains the current local values.

### Auth mode: none

If you want to test the `none` guard, temporarily edit the definition:

```bash
# This should report that credential management is disabled
cc-deck env refresh-creds remote-test
```

## Step 6: Reattach

This step confirms that credentials persist across detach/reattach cycles (**User Story 3**).

```bash
cc-deck attach remote-test
```

**Observe:**

- Credentials are rewritten to the remote before attaching.
- The existing Zellij session `cc-deck-remote-test` is detected and reused (no new session created).
- Open a new pane inside Zellij and verify credentials are available:

```bash
echo $ANTHROPIC_API_KEY | head -c 10
```

Detach again with `Ctrl+o d`.

## Step 7: File Synchronization

This step validates **User Story 6** (push/pull).

### Push files to remote

Create a test directory locally:

```bash
mkdir -p /tmp/ssh-test-push
echo "hello from local" > /tmp/ssh-test-push/test.txt
```

Push it:

```bash
cc-deck env push remote-test /tmp/ssh-test-push ~/workspace/
```

**Expected:** Files are transferred via rsync (or scp if rsync is unavailable).

### Verify on remote

```bash
ssh ningaloo "cat ~/workspace/ssh-test-push/test.txt"
```

**Expected:** Output is `hello from local`.

### Pull files from remote

```bash
ssh ningaloo "echo 'hello from remote' > ~/workspace/remote-file.txt"
cc-deck env pull remote-test ~/workspace/remote-file.txt /tmp/ssh-test-pull
```

**Expected:** `/tmp/ssh-test-pull` contains the file with `hello from remote`.

### Cleanup test files

```bash
rm -rf /tmp/ssh-test-push /tmp/ssh-test-pull
ssh ningaloo "rm -rf ~/workspace/ssh-test-push ~/workspace/remote-file.txt"
```

## Step 8: Remote Command Execution

This step validates **User Story 7** (exec).

```bash
cc-deck env exec remote-test -- uname -a
```

**Expected:** Output shows the remote kernel information (Linux, aarch64).

```bash
cc-deck env exec remote-test -- ls -la ~/workspace/
```

**Expected:** Directory listing of the remote workspace.

```bash
cc-deck env exec remote-test -- pwd
```

**Expected:** The output is the configured workspace path (`/root/workspace` or `~/workspace` expanded).

## Step 9: Harvest Git Commits

This step validates **User Story 8** (harvest). This requires a git repository on both ends.

### Setup (if not already a git repo on the remote)

```bash
ssh ningaloo "cd ~/workspace && git init && echo test > file.txt && git add . && git commit -m 'remote commit'"
```

### Harvest

From a local git repository:

```bash
cd /tmp && mkdir harvest-test && cd harvest-test && git init
cc-deck env harvest remote-test
```

**Expected:** A temporary git remote is added, commits are fetched, and the temporary remote is removed.

### Cleanup

```bash
rm -rf /tmp/harvest-test
ssh ningaloo "rm -rf ~/workspace/.git ~/workspace/file.txt"
```

## Step 10: Delete the Environment

This step validates **User Story 1** (delete).

### Without force (should fail if running)

```bash
cc-deck env delete remote-test
```

**Expected:** Error message indicating the environment is running. Use `--force` to delete.

### With force

```bash
cc-deck env delete remote-test --force
```

**Expected:**

- The remote Zellij session `cc-deck-remote-test` is killed (best-effort).
- The state record is removed.
- `cc-deck env list` no longer shows `remote-test`.

### Verify remote session is gone

```bash
ssh ningaloo "zellij list-sessions -n 2>/dev/null || echo 'no sessions'"
```

**Expected:** No session named `cc-deck-remote-test`.

## Step 11: Cleanup

Remove the definition from the environments file:

```bash
# Edit ~/.config/cc-deck/environments.yaml and remove the remote-test entry
```

Remove any remote artifacts:

```bash
ssh ningaloo "rm -rf ~/.config/cc-deck/credentials.env ~/workspace"
```

## Summary Checklist

| User Story | What to Verify | Status |
|------------|----------------|--------|
| US1: Connect | Create, attach, detach, session persists | |
| US2: Bootstrap | Pre-flight checks run, tool installation offered | |
| US3: Credentials | Written on attach, persist across reattach | |
| US4: Status | Reports running/stopped/error accurately | |
| US5: Refresh-creds | Updates remote file without attaching | |
| US6: File sync | Push and pull transfer files correctly | |
| US7: Exec | Commands run in workspace, output returned | |
| US8: Harvest | Git commits fetched from remote | |
| Nested Zellij | Attach refused inside existing Zellij | |
| Delete without force | Refused when running | |
| Delete with force | Kills session, removes state | |

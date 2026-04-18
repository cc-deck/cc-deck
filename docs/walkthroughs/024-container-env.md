# Manual Test: Container Environment (024)

Walkthrough for verifying the container environment feature end-to-end.
Uses `fedora:latest` for basic lifecycle tests and the demo image for full Zellij attach.

## Prerequisites

```bash
# Verify podman is installed and running
podman info --format '{{.Host.Security.Rootless}}'

# Build the CLI (from project root)
cd cc-deck && go build -o /tmp/cc-deck-test ./cmd/cc-deck
alias ccd=/tmp/cc-deck-test
```

## 1. Create a Container Environment (US1)

### 1a. Create with default (demo) image

```bash
ccd ws new test-basic --type container
# Expected: WARNING about using default image, then "Environment created"
# Verify:
podman ps -a --filter name=cc-deck-test-basic --format '{{.Names}} {{.Status}}'
podman volume ls --filter name=cc-deck-test-basic-data
```

### 1b. Create with explicit image and named volume

```bash
ccd ws new test-fedora --type container --image fedora:latest
# Expected: "Environment created" (no warning)
# Verify container running:
podman inspect cc-deck-test-fedora --format '{{.State.Status}}'
# Expected: running
```

### 1c. Create with host bind mount

```bash
mkdir -p ~/cc-deck-test-workspace
ccd ws new test-bind --type container --image fedora:latest --storage host-path --path ~/cc-deck-test-workspace
# Verify mount:
podman inspect cc-deck-test-bind --format '{{range .Mounts}}{{.Source}}:{{.Destination}}{{end}}'
# Expected: ~/cc-deck-test-workspace:/workspace
```

### 1d. Create with host-path, no --path (should use cwd)

```bash
cd ~/cc-deck-test-workspace
ccd ws new test-cwd --type container --image fedora:latest --storage host-path
podman inspect cc-deck-test-cwd --format '{{range .Mounts}}{{.Source}}{{end}}'
# Expected: ~/cc-deck-test-workspace
cd -
```

### 1e. Create with port mapping

```bash
ccd ws new test-ports --type container --image fedora:latest --port 8080:8080 --port 9090:9090
podman inspect cc-deck-test-ports --format '{{.HostConfig.PortBindings}}'
# Expected: port bindings for 8080 and 9090
```

### 1f. Verify naming conventions

```bash
# Container: cc-deck-<name>
podman ps -a --format '{{.Names}}' | grep cc-deck-test
# Volume: cc-deck-<name>-data
podman volume ls --format '{{.Name}}' | grep cc-deck-test
```

### 1g. Verify definition and state files

```bash
cat ~/.config/cc-deck/environments.yaml
# Expected: entries for test-basic, test-fedora, test-bind, etc.

cat ~/.local/state/cc-deck/state.yaml
# Expected: version 2, instances with container fields
```

### 1h. Duplicate name rejected

```bash
ccd ws new test-fedora --type container --image fedora:latest
# Expected: error "environment with this name already exists"
```

## 2. Stop and Start (US2)

```bash
# Stop
ccd ws stop test-fedora
podman inspect cc-deck-test-fedora --format '{{.State.Status}}'
# Expected: exited

# Verify state updated
ccd ws list
# Expected: test-fedora shows "stopped"

# Start
ccd ws start test-fedora
podman inspect cc-deck-test-fedora --format '{{.State.Status}}'
# Expected: running

# Verify workspace data survives stop/start
podman exec cc-deck-test-fedora touch /workspace/survivor.txt
ccd ws stop test-fedora
ccd ws start test-fedora
podman exec cc-deck-test-fedora ls /workspace/survivor.txt
# Expected: /workspace/survivor.txt (file persists)
```

## 3. List and Status (US3)

```bash
# List all environments
ccd ws list
# Expected: table with NAME, TYPE, STATUS, STORAGE, LAST ATTACHED, AGE
# Should show both local and container types

# Filter by type
ccd ws list --type container
# Expected: only container environments

# JSON output
ccd ws list -o json | jq '.[].name'

# Detailed status
ccd ws status test-fedora
# Expected: Environment, Type, Status, Storage, Uptime, Attached fields
```

### 3a. Reconciliation: externally stopped container

```bash
# Stop container outside cc-deck
podman stop cc-deck-test-fedora
# List should reconcile
ccd ws list
# Expected: test-fedora shows "stopped" (reconciled from podman)
# Restart for subsequent tests
ccd ws start test-fedora
```

### 3b. Reconciliation: externally deleted container

```bash
# Create a throwaway env
ccd ws new test-orphan --type container --image fedora:latest
# Delete container outside cc-deck
podman rm -f cc-deck-test-orphan
# List should show error state
ccd ws list
# Expected: test-orphan shows "error"
# Clean up stale record
ccd ws delete test-orphan --force
```

## 4. File Transfer (US4)

```bash
# Create test files
mkdir -p ~/cc-deck-push-test
echo "hello from host" > ~/cc-deck-push-test/greeting.txt

# Push files into container
ccd ws push test-fedora ~/cc-deck-push-test
# Verify inside container
podman exec cc-deck-test-fedora cat /workspace/cc-deck-push-test/greeting.txt
# Expected: hello from host

# Create file inside container
podman exec cc-deck-test-fedora bash -c 'echo "hello from container" > /workspace/response.txt'

# Pull files from container
ccd ws pull test-fedora /workspace/response.txt ~/cc-deck-pull-test
cat ~/cc-deck-pull-test
# Expected: hello from container
```

### 4a. Push/pull on stopped container

```bash
ccd ws stop test-fedora
ccd ws push test-fedora ~/cc-deck-push-test
# Expected: error "container is not running"
ccd ws start test-fedora
```

## 5. Credentials (US5)

```bash
# Create with explicit credential
ccd ws new test-creds --type container --image fedora:latest \
  --credential TEST_API_KEY=sk-test-12345

# Verify secret exists
podman secret ls --format '{{.Name}}' | grep cc-deck-test-creds
# Expected: cc-deck-test-creds-test-api-key

# Verify credential available inside container
podman exec cc-deck-test-creds env | grep TEST_API_KEY
# Expected: TEST_API_KEY=sk-test-12345

# Verify NOT visible in podman inspect
podman inspect cc-deck-test-creds --format '{{.Config.Env}}' | grep -c "sk-test-12345"
# Expected: 0 (not found)
```

### 5a. Auto-detect host env var

```bash
export ANTHROPIC_API_KEY=sk-ant-test-auto-detect
ccd ws new test-autodetect --type container --image fedora:latest
podman exec cc-deck-test-autodetect env | grep ANTHROPIC_API_KEY
# Expected: ANTHROPIC_API_KEY=sk-ant-test-auto-detect
unset ANTHROPIC_API_KEY
```

## 6. Exec (US6)

```bash
# Run a command inside the container
ccd ws exec test-fedora -- ls /workspace
# Expected: list of files in /workspace

ccd ws exec test-fedora -- cat /etc/os-release
# Expected: Fedora release info

# Exec on stopped container
ccd ws stop test-fedora
ccd ws exec test-fedora -- ls
# Expected: error "container is not running"
ccd ws start test-fedora
```

## 7. Hand-Edit Definitions (US7)

```bash
# Check current definition
cat ~/.config/cc-deck/environments.yaml

# Edit the image field for test-fedora to ubuntu:latest
# (use your editor of choice)

# Delete and recreate to pick up new definition
ccd ws delete test-fedora --force
ccd ws new test-fedora
# The new image from the definition should be used

podman inspect cc-deck-test-fedora --format '{{.Config.Image}}'
# Expected: ubuntu:latest (or whatever you changed it to)
```

## 8. Attach to Zellij Session (US1 + FR-006 + FR-018)

Attach requires an image with Zellij and the cc-deck config plugin installed
(e.g., the demo image). It creates a Zellij session with the cc-deck
layout (sidebar plugin) on first attach.

```bash
# test-basic uses the demo image, so attach works
ccd ws attach test-basic
# Expected:
#   1. Checks for existing Zellij session inside container
#   2. If none: runs 'zellij -n cc-deck' (creates session with cc-deck layout)
#   3. If exists: runs 'zellij attach' (reconnects)
#   4. You see the cc-deck sidebar on the left
# Detach with: Ctrl+o d

# Verify session persists after detach
podman exec cc-deck-test-basic zellij list-sessions -n
# Expected: cc-deck (session still running inside container)

# Re-attach (session already exists, skips creation)
ccd ws attach test-basic
# Expected: attaches directly to existing session (no layout recreation)
# Detach with: Ctrl+o d
```

### 8a. Auto-start on attach (FR-018)

```bash
ccd ws stop test-basic
ccd ws list
# Expected: test-basic shows "stopped"

ccd ws attach test-basic
# Expected: container auto-starts first, then attaches to Zellij
# Detach with: Ctrl+o d
```

### 8b. Nested Zellij check

```bash
# If you are already inside a Zellij session on the host:
ccd ws attach test-basic
# Expected: "Already inside Zellij. Detach first (Ctrl+o d), then run:
#            cc-deck ws attach test-basic"
```

### 8c. Attach to non-Zellij image (expected failure)

```bash
# test-fedora uses fedora:latest (no Zellij installed)
ccd ws attach test-fedora
# Expected: error from podman exec (zellij: command not found)
# This is expected: only images with Zellij support attach
```

## 9. Delete with Cleanup (US1)

### 9a. Delete with volume removal (default)

```bash
ccd ws delete test-basic --force
podman ps -a --filter name=cc-deck-test-basic --format '{{.Names}}'
# Expected: empty (container removed)
podman volume ls --filter name=cc-deck-test-basic-data --format '{{.Name}}'
# Expected: empty (volume removed)
```

### 9b. Delete with --keep-volumes

```bash
ccd ws delete test-fedora --force --keep-volumes
podman volume ls --filter name=cc-deck-test-fedora-data --format '{{.Name}}'
# Expected: cc-deck-test-fedora-data (volume preserved)
# Clean up manually
podman volume rm cc-deck-test-fedora-data
```

## 10. Config Defaults (FR-017)

```bash
# Set a default image in config
mkdir -p ~/.config/cc-deck
cat >> ~/.config/cc-deck/config.yaml << 'YAML'
defaults:
  container:
    image: fedora:latest
    storage: named-volume
YAML

# Create without --image, should use config default (no warning)
ccd ws new test-config --type container
podman inspect cc-deck-test-config --format '{{.Config.Image}}'
# Expected: fedora:latest (from config, no "using default" warning)

# Clean up
ccd ws delete test-config --force
```

## 11. Auth Auto-Detection

### 11a. API key mode (default for most users)

```bash
export ANTHROPIC_API_KEY=sk-ant-test-walkthrough
ccd ws new test-auth-api --type container --image fedora:latest
# Expected: auto-detects API mode, injects ANTHROPIC_API_KEY
podman exec cc-deck-test-auth-api env | grep ANTHROPIC_API_KEY
# Expected: ANTHROPIC_API_KEY=sk-ant-test-walkthrough
ccd ws delete test-auth-api --force
unset ANTHROPIC_API_KEY
```

### 11b. Vertex AI mode

```bash
export CLAUDE_CODE_USE_VERTEX=1
export ANTHROPIC_VERTEX_PROJECT_ID=my-test-project
export CLOUD_ML_REGION=us-east5
# Note: GOOGLE_APPLICATION_CREDENTIALS is auto-detected from
# ~/.config/gcloud/application_default_credentials.json if not set
ccd ws new test-auth-vertex --type container --image fedora:latest
podman exec cc-deck-test-auth-vertex env | grep -E "VERTEX|CLOUD_ML|GOOGLE_APP"
# Expected: CLAUDE_CODE_USE_VERTEX=1, ANTHROPIC_VERTEX_PROJECT_ID, CLOUD_ML_REGION,
#           GOOGLE_APPLICATION_CREDENTIALS=/run/secrets/... (if ADC file exists)
ccd ws delete test-auth-vertex --force
unset CLAUDE_CODE_USE_VERTEX ANTHROPIC_VERTEX_PROJECT_ID CLOUD_ML_REGION
```

### 11d. Vertex AI full setup (zero flags)

```bash
# With Vertex env vars set on the host (e.g., in .zshrc):
# CLAUDE_CODE_USE_VERTEX=1
# ANTHROPIC_VERTEX_PROJECT_ID=my-project
# CLOUD_ML_REGION=europe-west1
# And 'gcloud auth application-default login' done

ccd ws new test-vertex --type container
# Expected: auto-detects Vertex mode, injects all env vars,
#           auto-detects ADC file from ~/.config/gcloud/ and mounts as secret
ccd ws attach test-vertex
# Expected: Claude Code shows "Vertex AI" (not "API Usage Billing")
# Detach with: Ctrl+o d
ccd ws delete test-vertex --force
```

### 11c. Explicit auth mode override

```bash
export ANTHROPIC_API_KEY=sk-ant-test
ccd ws new test-auth-none --type container --image fedora:latest --auth none
podman exec cc-deck-test-auth-none env | grep ANTHROPIC_API_KEY
# Expected: empty (no auth passthrough with --auth none)
ccd ws delete test-auth-none --force
unset ANTHROPIC_API_KEY
```

## 12. Bind Mounts

```bash
# Mount a host directory into the container
mkdir -p ~/cc-deck-mount-test
echo "mounted" > ~/cc-deck-mount-test/marker.txt
ccd ws new test-mount --type container --image fedora:latest \
  --mount ~/cc-deck-mount-test:/mnt/test:ro
podman exec cc-deck-test-mount cat /mnt/test/marker.txt
# Expected: mounted
ccd ws delete test-mount --force
rm -rf ~/cc-deck-mount-test
```

## 13. Error Cases

### Podman not available

```bash
PATH=/usr/bin ccd ws new test-nopodman --type container
# Expected: error "podman binary not found in PATH"
```

### Invalid name

```bash
ccd ws new "INVALID NAME" --type container
# Expected: error about invalid environment name
```

### Harvest not supported

```bash
ccd ws harvest test-bind
# Expected: error suggesting push/pull and compose
```

## Cleanup

```bash
# Remove all test environments
for name in test-bind test-cwd test-ports test-creds test-autodetect test-auth-api test-auth-vertex test-auth-none test-mount; do
  ccd ws delete "$name" --force 2>/dev/null
done

# Remove test files
rm -rf ~/cc-deck-test-workspace ~/cc-deck-push-test ~/cc-deck-pull-test

# Remove test config additions (edit manually if needed)
# Remove podman volumes
podman volume prune -f

# Unset alias
unalias ccd
```

## Verification Summary

| Test | US | FR | Result |
|------|----|----|--------|
| Create with default image | US1 | FR-001, FR-015 | |
| Create with explicit image | US1 | FR-001 | |
| Create with named volume | US1 | FR-004 | |
| Create with bind mount | US1 | FR-004 | |
| Create with cwd fallback | US1 | FR-004 | |
| Create with ports | US1 | FR-012 | |
| sleep infinity entrypoint | US1 | FR-005 | |
| Definition/state files written | US1 | FR-002 | |
| Duplicate name rejected | US1 | FR-002 | |
| Stop container | US2 | FR-001 | |
| Start container | US2 | FR-001 | |
| Data survives stop/start | US2 | FR-004 | |
| List environments | US3 | FR-010 | |
| Status detail | US3 | FR-010 | |
| Reconcile external stop | US3 | FR-010 | |
| Reconcile external delete | US3 | FR-010 | |
| Push files | US4 | FR-009 | |
| Pull files | US4 | FR-009 | |
| Push on stopped (error) | US4 | FR-009 | |
| Explicit credential | US5 | FR-007 | |
| Auto-detect credential | US5 | FR-008 | |
| Credential not in inspect | US5 | FR-007 | |
| Exec command | US6 | FR-001 | |
| Exec on stopped (error) | US6 | FR-001 | |
| Hand-edit definition | US7 | FR-017 | |
| Auto-start on attach | US1 | FR-018 | |
| Delete with volume cleanup | US1 | FR-013, FR-014 | |
| Delete with --keep-volumes | US1 | FR-013 | |
| Config defaults | - | FR-017 | |
| Podman not available | - | FR-001 | |
| Invalid name | - | FR-001 | |
| Harvest not supported | - | FR-016 | |
| Auth auto-detect API | - | - | |
| Auth auto-detect Vertex | - | - | |
| Auth none (opt-out) | - | - | |
| Bind mount (--mount) | - | - | |

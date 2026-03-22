# Manual Test: Compose Environment (025)

Walkthrough for verifying the compose environment feature end-to-end.
Uses `fedora:latest` for basic lifecycle tests and the demo image for full Zellij attach.

## Prerequisites

```bash
# Verify podman and podman-compose are installed
podman info --format '{{.Host.Security.Rootless}}'
podman-compose version

# Build the CLI (from project root)
cd cc-deck && go build -o /tmp/cc-deck-test ./cmd/cc-deck
alias ccd=/tmp/cc-deck-test

# Create a test project directory
mkdir -p ~/cc-deck-compose-test/src
echo 'package main; func main() {}' > ~/cc-deck-compose-test/src/main.go
echo 'module test-project' > ~/cc-deck-compose-test/go.mod
cd ~/cc-deck-compose-test
```

## 1. Create a Compose Environment (US1)

### 1a. Create with default image (bind mount)

```bash
ccd env create test-compose --type compose
# Expected: WARNING about using default image, then "Environment created (type: compose)"

# Verify .cc-deck/ directory was created
ls -la .cc-deck/
# Expected: compose.yaml, env (no dot prefix on env file)

# Verify compose.yaml contains session service
grep -A 2 'session:' .cc-deck/compose.yaml
# Expected: image: quay.io/cc-deck/cc-deck-demo:latest

# Verify workspace bind mount with :U ownership mapping
grep '/workspace' .cc-deck/compose.yaml
# Expected: ./..:/workspace:U  (the :U flag fixes UID mismatch on macOS)

# Verify stdin_open and tty
grep -E 'stdin_open|tty' .cc-deck/compose.yaml
# Expected: stdin_open: true, tty: true

# Verify container is running
podman ps --filter name=cc-deck-test-compose --format '{{.Names}} {{.Status}}'
# Expected: cc-deck-test-compose Up ...
```

### 1b. Verify bind mount works

```bash
# Check project files are visible inside container
podman exec cc-deck-test-compose ls /workspace/
# Expected: go.mod  src

# Create a file inside the container
podman exec cc-deck-test-compose touch /workspace/from-container.txt

# Verify it appears on the host
ls ~/cc-deck-compose-test/from-container.txt
# Expected: file exists

# Create a file on the host
echo "hello" > ~/cc-deck-compose-test/from-host.txt

# Verify it appears inside the container
podman exec cc-deck-test-compose cat /workspace/from-host.txt
# Expected: hello
```

### 1c. Verify definition and state files

```bash
cat ~/.config/cc-deck/environments.yaml | grep -A 5 'test-compose'
# Expected: type: compose, project-dir: ~/cc-deck-compose-test

cat ~/.local/state/cc-deck/state.yaml | grep -A 5 'test-compose'
# Expected: type: compose, compose: { project_dir: ..., container_name: cc-deck-test-compose }
```

### 1d. Duplicate name rejected (fast-fail)

```bash
ccd env create test-compose --type compose
# Expected: error "already exists" - fails immediately before creating any resources
# Verify no second container was started:
podman ps --filter name=cc-deck-test-compose --format '{{.Names}}' | wc -l
# Expected: 1 (only the original)
```

### 1e. Create with explicit project path

```bash
mkdir -p /tmp/other-project
echo "other" > /tmp/other-project/README.md
ccd env create test-path --type compose --path /tmp/other-project
# Expected: "Environment created"

# Verify project dir mounted
podman exec cc-deck-test-path cat /workspace/README.md
# Expected: other

# Verify .cc-deck/ created in the specified path
ls /tmp/other-project/.cc-deck/compose.yaml
# Expected: file exists

ccd env delete test-path --force
rm -rf /tmp/other-project
```

### 1f. Create with named volume instead of bind mount

```bash
ccd env create test-volume --type compose --image fedora:latest --storage named-volume

# Verify volume exists
podman volume ls --filter name=cc-deck-test-volume
# Expected: cc-deck-test-volume-data

# Verify compose.yaml declares the volume as external
grep -A 1 'external' .cc-deck/compose.yaml
# Expected: external: true (volume is pre-created by cc-deck)

ccd env delete test-volume --force
```

### 1g. Create with port mapping

```bash
ccd env create test-ports --type compose --image fedora:latest --port 8080:8080
grep 'ports' .cc-deck/compose.yaml
# Expected: - 8080:8080
ccd env delete test-ports --force
```

## 2. Stop and Start (US3)

```bash
# Stop
ccd env stop test-compose
podman inspect cc-deck-test-compose --format '{{.State.Status}}'
# Expected: exited

# Verify state updated
ccd env list
# Expected: test-compose shows "stopped"

# Start
ccd env start test-compose
podman inspect cc-deck-test-compose --format '{{.State.Status}}'
# Expected: running

# Verify workspace data survives stop/start
podman exec cc-deck-test-compose cat /workspace/from-host.txt
# Expected: hello (file persists across stop/start)
```

### 2a. Graceful fallback when .cc-deck/ is missing

```bash
# Simulate missing compose files (e.g., after manual cleanup)
mv .cc-deck .cc-deck-backup
ccd env stop test-compose
# Expected: falls back to direct podman stop (no chdir error)
podman inspect cc-deck-test-compose --format '{{.State.Status}}'
# Expected: exited
ccd env start test-compose
# Expected: falls back to direct podman start
podman inspect cc-deck-test-compose --format '{{.State.Status}}'
# Expected: running
mv .cc-deck-backup .cc-deck
```

## 3. List and Status (US3)

```bash
# List all environments (compose should appear)
ccd env list
# Expected: test-compose with type=compose, status=running, storage=host-path

# Filter by type
ccd env list --type compose
# Expected: only compose environments

# JSON output
ccd env list -o json | jq '.[] | select(.type == "compose")'

# Detailed status
ccd env status test-compose
# Expected: Environment, Type (compose), Status, Storage, Uptime, Attached fields
```

### 3a. Reconciliation: externally stopped container

```bash
# Stop container outside cc-deck
podman stop cc-deck-test-compose
# List should reconcile
ccd env list
# Expected: test-compose shows "stopped" (reconciled from podman)
# Restart for subsequent tests
ccd env start test-compose
```

## 4. Credentials (US4)

### 4a. Auto-detect API key

```bash
export ANTHROPIC_API_KEY=sk-ant-test-compose
ccd env create test-creds --type compose --image fedora:latest
# Verify credential in env file
cat .cc-deck/env
# Expected: ANTHROPIC_API_KEY=sk-ant-test-compose

# Verify credential available inside container
podman exec cc-deck-test-creds env | grep ANTHROPIC_API_KEY
# Expected: ANTHROPIC_API_KEY=sk-ant-test-compose

ccd env delete test-creds --force
unset ANTHROPIC_API_KEY
```

### 4b. Explicit credential

```bash
ccd env create test-explicit --type compose --image fedora:latest \
  --credential MY_SECRET=super-secret-value
cat .cc-deck/env | grep MY_SECRET
# Expected: MY_SECRET=super-secret-value

podman exec cc-deck-test-explicit env | grep MY_SECRET
# Expected: MY_SECRET=super-secret-value

ccd env delete test-explicit --force
```

### 4c. File-based credential (Vertex ADC) via volume mount

```bash
# Create a fake ADC file
mkdir -p /tmp/test-cred-dir
echo '{"type":"authorized_user","client_id":"test"}' > /tmp/test-cred-dir/adc.json

export CLAUDE_CODE_USE_VERTEX=1
export ANTHROPIC_VERTEX_PROJECT_ID=my-test-project
export CLOUD_ML_REGION=us-east5
export GOOGLE_APPLICATION_CREDENTIALS=/tmp/test-cred-dir/adc.json

ccd env create test-vertex --type compose --image fedora:latest

# Verify NO .cc-deck/secrets/ directory (files are not copied)
ls .cc-deck/secrets/ 2>&1
# Expected: No such file or directory

# Verify env var points to secret mount path
cat .cc-deck/env | grep GOOGLE_APPLICATION_CREDENTIALS
# Expected: GOOGLE_APPLICATION_CREDENTIALS=/run/secrets/google-application-credentials

# Verify compose.yaml uses :ro,U volume mount (not compose secrets)
grep 'google-application-credentials' .cc-deck/compose.yaml
# Expected: /tmp/test-cred-dir/adc.json:/run/secrets/google-application-credentials:ro,U

# Verify the credential is readable inside the container (non-root user)
podman exec cc-deck-test-vertex cat /run/secrets/google-application-credentials
# Expected: {"type":"authorized_user","client_id":"test"}

# Verify the mount is live (no copy, reads from original file)
echo '{"type":"updated"}' > /tmp/test-cred-dir/adc.json
podman exec cc-deck-test-vertex cat /run/secrets/google-application-credentials
# Expected: {"type":"updated"} (live file, not a stale copy)

ccd env delete test-vertex --force
unset CLAUDE_CODE_USE_VERTEX ANTHROPIC_VERTEX_PROJECT_ID CLOUD_ML_REGION GOOGLE_APPLICATION_CREDENTIALS
rm -rf /tmp/test-cred-dir
```

### 4d. Vertex + API key both included

```bash
export CLAUDE_CODE_USE_VERTEX=1
export ANTHROPIC_VERTEX_PROJECT_ID=my-project
export CLOUD_ML_REGION=europe-west1
export ANTHROPIC_API_KEY=sk-ant-also-included
ccd env create test-both --type compose --image fedora:latest
cat .cc-deck/env
# Expected: contains BOTH Vertex vars AND ANTHROPIC_API_KEY
#           (API key is always included as fallback)
ccd env delete test-both --force
unset CLAUDE_CODE_USE_VERTEX ANTHROPIC_VERTEX_PROJECT_ID CLOUD_ML_REGION ANTHROPIC_API_KEY
```

### 4e. Auth mode override (none)

```bash
export ANTHROPIC_API_KEY=sk-ant-should-be-ignored
ccd env create test-noauth --type compose --image fedora:latest --auth none
cat .cc-deck/env
# Expected: empty (no credentials injected)
ccd env delete test-noauth --force
unset ANTHROPIC_API_KEY
```

## 5. Network Filtering (US2)

### 5a. Create with allowed domains

```bash
ccd env create test-filter --type compose --image fedora:latest \
  --allowed-domains anthropic,github

# Verify proxy config files generated
ls .cc-deck/proxy/
# Expected: tinyproxy.conf  whitelist

# Verify whitelist contains domain patterns
cat .cc-deck/proxy/whitelist
# Expected: regex patterns for anthropic.com and github.com

# Verify tinyproxy config
grep 'FilterDefaultDeny' .cc-deck/proxy/tinyproxy.conf
# Expected: FilterDefaultDeny Yes

# Verify compose.yaml has proxy service
grep -A 2 'proxy:' .cc-deck/compose.yaml
# Expected: proxy service definition

# Verify session container has proxy env vars
grep 'HTTP_PROXY' .cc-deck/compose.yaml
# Expected: HTTP_PROXY: http://proxy:8888

# Verify internal network exists
grep 'internal:' .cc-deck/compose.yaml
# Expected: internal network with internal: true

# Verify proxy container is running
podman ps --filter name=cc-deck-test-filter-proxy --format '{{.Names}}'
# Expected: cc-deck-test-filter-proxy

# Verify proxy name recorded in state
cat ~/.local/state/cc-deck/state.yaml | grep proxy_name
# Expected: proxy_name: cc-deck-test-filter-proxy

ccd env delete test-filter --force
```

### 5b. Create without domains (no proxy)

```bash
ccd env create test-nofilter --type compose --image fedora:latest
ls .cc-deck/proxy/ 2>/dev/null
# Expected: No such file or directory (no proxy dir)
grep 'proxy' .cc-deck/compose.yaml
# Expected: no matches (no proxy service)
ccd env delete test-nofilter --force
```

### 5c. Domain group expansion with literal domains

```bash
ccd env create test-literal --type compose --image fedora:latest \
  --allowed-domains anthropic,custom.example.com
cat .cc-deck/proxy/whitelist
# Expected: patterns for anthropic domains AND custom.example.com
ccd env delete test-literal --force
```

## 6. Gitignore Handling (US5)

### 6a. Warning when .gitignore missing

```bash
# Initialize a git repo (if not already)
cd ~/cc-deck-compose-test
git init 2>/dev/null

# Remove .cc-deck/ from .gitignore if present
grep -v '.cc-deck' .gitignore > .gitignore.tmp 2>/dev/null; mv .gitignore.tmp .gitignore 2>/dev/null || true

ccd env create test-gitignore --type compose --image fedora:latest 2>&1 | grep -i gitignore
# Expected: WARNING: Add '.cc-deck/' to your .gitignore ...
#           Use --gitignore to add it automatically.

ccd env delete test-gitignore --force
```

### 6b. Auto-add with --gitignore

```bash
# Remove .cc-deck/ from .gitignore if present
grep -v '.cc-deck' .gitignore > .gitignore.tmp 2>/dev/null; mv .gitignore.tmp .gitignore 2>/dev/null || true

ccd env create test-autogit --type compose --image fedora:latest --gitignore
cat .gitignore
# Expected: contains .cc-deck/

ccd env delete test-autogit --force
```

### 6c. No duplicate when already present

```bash
echo ".cc-deck/" >> .gitignore
ccd env create test-nodup --type compose --image fedora:latest --gitignore
grep -c '.cc-deck/' .gitignore
# Expected: 1 (not duplicated)
ccd env delete test-nodup --force
```

## 7. Exec and File Transfer (US1)

```bash
# Exec command
ccd env exec test-compose -- ls /workspace
# Expected: go.mod  src  from-container.txt  from-host.txt

ccd env exec test-compose -- cat /etc/os-release
# Expected: OS release info

# Push files
echo "push test" > /tmp/push-me.txt
ccd env push test-compose /tmp/push-me.txt /workspace/pushed.txt
podman exec cc-deck-test-compose cat /workspace/pushed.txt
# Expected: push test

# Pull files
podman exec cc-deck-test-compose bash -c 'echo "pull test" > /workspace/pull-me.txt'
ccd env pull test-compose /workspace/pull-me.txt /tmp/pulled.txt
cat /tmp/pulled.txt
# Expected: pull test

# Harvest not supported
ccd env harvest test-compose
# Expected: error "compose environments do not support harvest"
```

## 8. Attach to Zellij Session (US1)

Attach requires an image with Zellij and the cc-deck plugin installed
(e.g., the demo image).

```bash
# test-compose uses the demo image, so attach works
ccd env attach test-compose
# Expected:
#   1. Checks for existing Zellij session inside container
#   2. If none: runs 'zellij -n cc-deck' (creates session with cc-deck layout)
#   3. If exists: runs 'zellij attach' (reconnects)
#   4. You see the cc-deck sidebar on the left
#   5. /workspace contains your project files
# Detach with: Ctrl+o d

# Verify LastAttached timestamp updated
ccd env status test-compose
# Expected: Attached field shows recent timestamp
```

### 8a. Auto-start on attach

```bash
ccd env stop test-compose
ccd env list
# Expected: test-compose shows "stopped"

ccd env attach test-compose
# Expected: container auto-starts first, then attaches to Zellij
# Detach with: Ctrl+o d
```

### 8b. Nested Zellij check

```bash
# If you are already inside a Zellij session on the host:
ccd env attach test-compose
# Expected: "Already inside Zellij. Detach first (Ctrl+o d), then run:
#            cc-deck env attach test-compose"
```

## 9. Delete with Cleanup (US3)

### 9a. Delete refuses running env

```bash
ccd env delete test-compose
# Expected: error "environment is running; use --force to delete"
```

### 9b. Delete with --force

```bash
ccd env delete test-compose --force
# Expected: "Environment deleted"

# Verify container removed
podman ps -a --filter name=cc-deck-test-compose --format '{{.Names}}'
# Expected: empty

# Verify .cc-deck/ removed
ls ~/cc-deck-compose-test/.cc-deck/ 2>&1
# Expected: No such file or directory

# Verify instance removed from state
ccd env list | grep test-compose
# Expected: no match
```

## 10. Existing .cc-deck/ Directory (FR-014)

```bash
# Pre-create .cc-deck/ with stale files
mkdir -p ~/cc-deck-compose-test/.cc-deck
echo "stale" > ~/cc-deck-compose-test/.cc-deck/old-file.txt

cd ~/cc-deck-compose-test
ccd env create test-regen --type compose --image fedora:latest 2>&1 | grep -i regenerat
# Expected: WARNING: regenerating compose files in .../cc-deck-compose-test/.cc-deck

# Verify fresh compose.yaml was generated
grep 'session:' .cc-deck/compose.yaml
# Expected: session service present (freshly generated)

# Old stale file may still be present (MkdirAll preserves existing dir)
# but compose.yaml is regenerated

ccd env delete test-regen --force
```

## 11. Runtime Detection (FR-015)

```bash
# Verify podman-compose is detected
ccd env create test-runtime --type compose --image fedora:latest
# Expected: succeeds (runtime detected)
ccd env delete test-runtime --force

# Simulate no compose runtime
PATH=/usr/bin ccd env create test-nocompose --type compose 2>&1
# Expected: error "compose runtime not available" or "compose runtime not found"
```

## 12. Error Cases

### Invalid name

```bash
ccd env create "INVALID!" --type compose
# Expected: error about invalid environment name
```

### Non-existent path

```bash
ccd env create test-badpath --type compose --path /nonexistent/path
# Expected: error creating .cc-deck directory
```

## Cleanup

```bash
# Remove all test environments
for name in test-compose test-creds test-explicit test-vertex test-both test-noauth test-filter test-nofilter test-literal test-gitignore test-autogit test-nodup test-regen test-runtime; do
  ccd env delete "$name" --force 2>/dev/null
done

# Remove test project
rm -rf ~/cc-deck-compose-test

# Remove test files
rm -f /tmp/push-me.txt /tmp/pulled.txt

# Remove podman volumes
podman volume prune -f

# Unset alias
unalias ccd
```

## Verification Summary

| Test | US | FR | Result |
|------|----|----|--------|
| Create with default image | US1 | FR-001, FR-015 | |
| .cc-deck/ directory created | US1 | FR-002 | |
| No dotfile nesting (env not .env) | US1 | FR-002 | |
| Bind mount at /workspace with :U | US1 | FR-003 | |
| Non-root write permissions | US1 | FR-003 | |
| Bidirectional file sync | US1 | FR-003 | |
| stdin_open and tty set | US1 | FR-011 | |
| Named volume (external decl) | US1 | FR-004 | |
| Port mapping | US1 | FR-001 | |
| Explicit project path | US1 | FR-018 | |
| Definition/state written | US1 | FR-001 | |
| Duplicate name fast-fail | US1 | FR-019 | |
| Stop container | US3 | FR-008 | |
| Start container | US3 | FR-008 | |
| Data survives stop/start | US3 | FR-008 | |
| Stop/start fallback (no .cc-deck/) | US3 | FR-008 | |
| List with compose type | US3 | FR-016 | |
| Status detail | US3 | FR-016 | |
| Reconcile external stop | US3 | FR-016 | |
| Auto-detect API key | US4 | FR-010 | |
| Explicit credential | US4 | FR-010 | |
| File credential (:ro,U mount) | US4 | FR-010 | |
| Live mount (no copy drift) | US4 | FR-010 | |
| Vertex + API key both included | US4 | FR-010 | |
| Auth mode none (opt-out) | US4 | FR-010 | |
| Allowed domains + proxy | US2 | FR-005, FR-006, FR-007 | |
| No proxy without domains | US2 | FR-005 | |
| Literal domain patterns | US2 | FR-006 | |
| Gitignore warning | US5 | FR-013 | |
| Gitignore auto-add | US5 | FR-013 | |
| Gitignore no duplicate | US5 | FR-013 | |
| Exec command | US1 | FR-017 | |
| Push files | US1 | FR-017 | |
| Pull files | US1 | FR-017 | |
| Harvest not supported | US1 | FR-023 | |
| Attach with Zellij layout | US1 | FR-011 | |
| Auto-start on attach | US1 | FR-012 | |
| Nested Zellij check | US1 | FR-011 | |
| LastAttached updated | US1 | FR-021 | |
| Delete refuses running | US3 | FR-022 | |
| Delete with --force | US3 | FR-009 | |
| .cc-deck/ removed on delete | US3 | FR-009 | |
| Existing .cc-deck/ regenerate | - | FR-014 | |
| Runtime detection | - | FR-015 | |
| Runtime not available error | - | FR-019 | |
| Invalid name error | - | FR-019 | |
| Cleanup on failure | - | FR-020 | |

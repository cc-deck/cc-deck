# Workspace-Centric Session Sharing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the unshipped standalone `cc-deck share` commands with local-workspace sharing integrated into `ws new`, `ws start`, `ws attach`, `ws invite`, `ws revoke`, `ws unshare`, `ws list`, and `ws status`.

**Architecture:** Add one reusable workspace readiness operation that independently converges infrastructure and the canonical Zellij session. Keep provider, invitation, locking, guard, and teardown logic in `internal/share`, but make it workspace-aware, support multiple named invitations, and create share-enabled Zellij sessions at session creation through `zellij attach -b NAME options --web-sharing on`. Cobra commands become thin adapters over readiness and sharing services.

**Tech Stack:** Go 1.22+, Cobra, YAML state, Zellij 0.44.3 CLI, Cloudflare Quick Tunnel, Testify, existing cc-deck workspace/provider abstractions.

---

## File Structure

### New files

- `cc-deck/internal/ws/readiness.go` — target-state reconciliation for infrastructure plus one canonical session.
- `cc-deck/internal/ws/readiness_test.go` — state-table tests for readiness and rollback.
- `cc-deck/internal/share/labels.go` — memorable random invitation-label generation with collision handling.
- `cc-deck/internal/share/labels_test.go` — deterministic label and collision tests.
- `cc-deck/internal/cmd/ws_share.go` — workspace sharing service construction and `invite`, `revoke`, `unshare`, and hidden guard commands.
- `cc-deck/internal/cmd/ws_share_test.go` — Cobra-level workspace sharing tests.

### Modified files

- `cc-deck/internal/ws/interface.go` — add `SessionManager`, `SessionStartOptions`, and `SessionStartResult` capability types.
- `cc-deck/internal/ws/local.go` / `local_test.go` — extract idempotent canonical-session creation and support creation-time web sharing.
- `cc-deck/internal/ws/container.go`, `compose.go`, `ssh.go`, `k8s_deploy.go`, `openshell.go` and corresponding tests — extract existing background session creation behind `SessionManager` without enabling sharing.
- `cc-deck/internal/ws/types.go` — expose reconciled sharing state in workspace status.
- `cc-deck/internal/cmd/ws.go` and workspace command tests — add lifecycle flags, call readiness, and render sharing state.
- `cc-deck/internal/cmd/ws_promote.go` / `ws_promote_test.go` — propagate `--share` through the promoted `attach` command.
- `cc-deck/internal/share/model.go`, `provider.go`, `service.go`, `guard.go` and tests — use workspace identity, invitation collections, add/revoke APIs, and session-health watching.
- `cc-deck/internal/share/zellij.go` / `zellij_test.go` — remove the invalid existing-session sharing mutation and expose session-health checks.
- `cc-deck/internal/share/invitation.go` / `invitation_test.go` — build one role-specific invitation at a time.
- `cc-deck/internal/cmd/share.go` / `share_test.go` — remove public standalone commands; retain only reusable runner code if still needed.
- `cc-deck/cmd/cc-deck/main.go` — stop registering `NewShareCmd`.
- `specs/084-zellij-session-sharing/{spec.md,plan.md,tasks.md,data-model.md,quickstart.md,contracts/cli.md}` — align formal artifacts and acceptance evidence.
- `README.md`, `docs/modules/reference/pages/{cli.adoc,configuration.adoc}`, `docs/modules/using/pages/sharing.adoc` — document workspace-centric sharing.

## Task 1: Align Feature 084 Artifacts With the Approved Design

**Files:**
- Modify: `specs/084-zellij-session-sharing/spec.md`
- Modify: `specs/084-zellij-session-sharing/plan.md`
- Modify: `specs/084-zellij-session-sharing/tasks.md`
- Modify: `specs/084-zellij-session-sharing/data-model.md`
- Modify: `specs/084-zellij-session-sharing/contracts/cli.md`
- Modify: `specs/084-zellij-session-sharing/quickstart.md`

- [ ] **Step 1: Replace the command contract**

Replace every public `cc-deck share ...` example with this exact command set:

```text
cc-deck ws new NAME [--share | --no-start]
cc-deck ws start NAME [--share]
cc-deck ws attach NAME [--share]
cc-deck ws invite NAME --role interactive|observer [--name LABEL]
cc-deck ws revoke NAME INVITATION_LABEL
cc-deck ws unshare NAME
cc-deck ws list
cc-deck ws status NAME
```

Record that sharing is local-only, one workspace may be shared per host, `--share` never replaces a private running session, raw secrets print once, and plain restart after session death is private.

- [ ] **Step 2: Replace the data model**

Define `WorkspaceSharingState` as `private|shared|degraded` and replace the two fixed token-label fields with a list containing `label`, `role`, `state`, and `created_at`. State explicitly that no raw token is persisted.

- [ ] **Step 3: Rewrite remaining tasks**

Mark already-reusable provider, locking, escaping, and guard work complete. Replace obsolete standalone-CLI tasks with the implementation tasks in this plan. Keep live acceptance and deferred repository verification as the final two tasks.

- [ ] **Step 4: Check artifact consistency**

Run:

```bash
rg -n 'cc-deck share|InteractiveTokenLabel|ObserverTokenLabel' specs/084-zellij-session-sharing
```

Expected: only historical/deprecation explanations, with no active command or data-model requirements.

- [ ] **Step 5: Commit**

```bash
git add specs/084-zellij-session-sharing
git commit -m "docs(share): align feature with workspace UX"
```

## Task 2: Introduce Canonical-Session Readiness and Implement It Locally

**Files:**
- Modify: `cc-deck/internal/ws/interface.go`
- Create: `cc-deck/internal/ws/readiness.go`
- Create: `cc-deck/internal/ws/readiness_test.go`
- Modify: `cc-deck/internal/ws/local.go`
- Modify: `cc-deck/internal/ws/local_test.go`

- [ ] **Step 1: Write failing readiness state-table tests**

Add fakes and cases equivalent to:

```go
type readinessFake struct {
    Workspace
    kind WorkspaceType
    infra *InfraStateValue
    session SessionStateValue
    infraStarts, sessionStarts int
}
func (f *readinessFake) Type() WorkspaceType { return f.kind }
func (f *readinessFake) Name() string { return "demo" }
func (f *readinessFake) Status(context.Context) (*WorkspaceStatus, error) {
    return &WorkspaceStatus{InfraState: f.infra, SessionState: f.session}, nil
}
func (f *readinessFake) Start(context.Context) error { f.infraStarts++; return nil }
func (f *readinessFake) Stop(context.Context) error { return nil }
func (f *readinessFake) EnsureSession(context.Context, SessionStartOptions) (SessionStartResult, error) {
    f.sessionStarts++
    return SessionStartResult{Created: true, Name: "cc-deck-demo"}, nil
}
func boolToInt(value bool) int {
    if value { return 1 }
    return 0
}
func readinessInfraPtr(value InfraStateValue) *InfraStateValue { return &value }

func TestEnsureReadyConvergesOnlyMissingDimensions(t *testing.T) {
    tests := []struct {
        name       string
        infra      *InfraStateValue
        session    SessionStateValue
        wantInfra  bool
        wantSession bool
    }{
        {"stopped and absent", readinessInfraPtr(InfraStateStopped), SessionStateNone, true, true},
        {"running and absent", readinessInfraPtr(InfraStateRunning), SessionStateNone, false, true},
        {"already ready", readinessInfraPtr(InfraStateRunning), SessionStateExists, false, false},
        {"local absent", nil, SessionStateNone, false, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            workspace := &readinessFake{kind: WorkspaceTypeContainer, infra: tt.infra, session: tt.session}
            _, err := EnsureReady(context.Background(), workspace, ReadyOptions{})
            require.NoError(t, err)
            require.Equal(t, boolToInt(tt.wantInfra), workspace.infraStarts)
            require.Equal(t, boolToInt(tt.wantSession), workspace.sessionStarts)
        })
    }
}

func TestEnsureReadyRejectsSharingForNonLocalWorkspace(t *testing.T) {
    _, err := EnsureReady(context.Background(), &readinessFake{kind: WorkspaceTypeContainer, session: SessionStateNone}, ReadyOptions{Share: true})
    require.ErrorContains(t, err, "sharing is currently supported for local workspaces only")
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
cd cc-deck && go test ./internal/ws -run 'TestEnsureReady' -count=1
```

Expected: FAIL because readiness types and `EnsureReady` do not exist.

- [ ] **Step 3: Add capability and result types**

Add to `interface.go`:

```go
type SessionStartOptions struct {
    WebSharing bool
}

type SessionStartResult struct {
    Created bool
    Name    string
}

type SessionManager interface {
    EnsureSession(context.Context, SessionStartOptions) (SessionStartResult, error)
}

type ReadyOptions struct {
    Share bool
}

type ReadyResult struct {
    InfrastructureStarted bool
    SessionCreated        bool
    SessionName           string
}
```

- [ ] **Step 4: Implement readiness convergence**

Create `readiness.go` with this control flow:

```go
func EnsureReady(ctx context.Context, workspace Workspace, opts ReadyOptions) (ReadyResult, error) {
    if opts.Share && workspace.Type() != WorkspaceTypeLocal {
        return ReadyResult{}, fmt.Errorf("sharing is currently supported for local workspaces only (workspace %q is %s)", workspace.Name(), workspace.Type())
    }
    status, err := workspace.Status(ctx)
    if err != nil { return ReadyResult{}, err }
    var result ReadyResult
    if status.InfraState != nil && *status.InfraState != InfraStateRunning {
        infra, ok := workspace.(InfraManager)
        if !ok { return result, fmt.Errorf("workspace %q cannot start infrastructure", workspace.Name()) }
        if err := infra.Start(ctx); err != nil { return result, err }
        result.InfrastructureStarted = true
    }
    if status.SessionState == SessionStateNone {
        sessions, ok := workspace.(SessionManager)
        if !ok { return result, fmt.Errorf("workspace %q cannot create its canonical session", workspace.Name()) }
        created, err := sessions.EnsureSession(ctx, SessionStartOptions{WebSharing: opts.Share})
        if err != nil { return result, err }
        result.SessionCreated, result.SessionName = created.Created, created.Name
    }
    return result, nil
}
```

- [ ] **Step 5: Write the failing local shared-session command test**

Inject or reuse a command runner and assert that a missing shared local session executes:

```go
[]string{"--layout", "cc-deck", "attach", "-b", "cc-deck-demo", "options", "--web-sharing", "on"}
```

Also assert that a running session returns `Created:false` and performs no command.

- [ ] **Step 6: Extract `LocalWorkspace.EnsureSession`**

Move stale-session deletion and background creation out of `Attach`. Use:

```go
func (e *LocalWorkspace) EnsureSession(ctx context.Context, opts SessionStartOptions) (SessionStartResult, error) {
    name := e.zellijSessionName()
    if ZellijSessionState(name) == "running" { return SessionStartResult{Name: name}, nil }
    if ZellijSessionState(name) == "exited" { _ = DeleteZellijSession(name, true) }
    args := []string{"--layout", "cc-deck", "attach", "-b", name}
    if opts.WebSharing { args = append(args, "options", "--web-sharing", "on") }
    if out, err := exec.CommandContext(ctx, "zellij", args...).CombinedOutput(); err != nil {
        return SessionStartResult{}, fmt.Errorf("creating canonical session: %w: %s", err, out)
    }
    e.setSessionState(SessionStateExists)
    return SessionStartResult{Created: true, Name: name}, nil
}
```

Make `Attach` call `EnsureSession(ctx, SessionStartOptions{})` before `syscall.Exec`.

- [ ] **Step 7: Run local and readiness tests**

Run:

```bash
cd cc-deck && go test ./internal/ws -run 'TestEnsureReady|TestLocalWorkspace' -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add cc-deck/internal/ws/interface.go cc-deck/internal/ws/readiness.go cc-deck/internal/ws/readiness_test.go cc-deck/internal/ws/local.go cc-deck/internal/ws/local_test.go
git commit -m "feat(ws): add canonical session readiness"
```

## Task 3: Make Every Backend Conform to Ready-Workspace Semantics

**Files:**
- Modify: `cc-deck/internal/ws/container.go`
- Modify: `cc-deck/internal/ws/container_test.go`
- Modify: `cc-deck/internal/ws/compose.go`
- Modify: `cc-deck/internal/ws/compose_test.go`
- Modify: `cc-deck/internal/ws/ssh.go`
- Modify: `cc-deck/internal/ws/ssh_test.go`
- Modify: `cc-deck/internal/ws/k8s_deploy.go`
- Modify: `cc-deck/internal/ws/k8s_deploy_test.go`
- Modify: `cc-deck/internal/ws/openshell.go`
- Modify: `cc-deck/internal/ws/openshell_test.go`

- [ ] **Step 1: Add failing contract tests for `SessionManager`**

For each backend, assert:

```go
func TestBackendEnsureSessionIsIdempotent(t *testing.T) {
    first, err := workspace.EnsureSession(context.Background(), SessionStartOptions{})
    require.NoError(t, err)
    require.True(t, first.Created)
    second, err := workspace.EnsureSession(context.Background(), SessionStartOptions{})
    require.NoError(t, err)
    require.False(t, second.Created)
}
```

Add one assertion per backend that `WebSharing:true` returns the local-only unsupported error before executing Podman, SSH, Kubernetes, or OpenShell commands.

- [ ] **Step 2: Run backend tests to verify failure**

Run:

```bash
cd cc-deck && go test ./internal/ws -run 'EnsureSession' -count=1
```

Expected: FAIL because non-local workspaces do not implement `SessionManager`.

- [ ] **Step 3: Extract session creation from each `Attach` implementation**

Implement the same signature on all five backends:

```go
func (e *ContainerWorkspace) EnsureSession(ctx context.Context, opts SessionStartOptions) (SessionStartResult, error)
func (e *ComposeWorkspace) EnsureSession(ctx context.Context, opts SessionStartOptions) (SessionStartResult, error)
func (e *SSHWorkspace) EnsureSession(ctx context.Context, opts SessionStartOptions) (SessionStartResult, error)
func (e *K8sDeployWorkspace) EnsureSession(ctx context.Context, opts SessionStartOptions) (SessionStartResult, error)
func (e *OpenShellWorkspace) EnsureSession(ctx context.Context, opts SessionStartOptions) (SessionStartResult, error)
```

Each implementation must begin with:

```go
if opts.WebSharing {
    return SessionStartResult{}, fmt.Errorf("sharing is currently supported for local workspaces only")
}
```

Move only the existing non-interactive “does a session exist / create it in the background” commands into these methods. Leave terminal replacement or interactive transport in `Attach`, which first calls `EnsureSession` and then connects.

- [ ] **Step 4: Run all workspace tests**

Run:

```bash
cd cc-deck && go test ./internal/ws -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cc-deck/internal/ws
git commit -m "refactor(ws): separate session readiness from attach"
```

## Task 4: Generalize Sharing State to Named Invitations

**Files:**
- Modify: `cc-deck/internal/share/model.go`
- Modify: `cc-deck/internal/share/model_test.go`
- Create: `cc-deck/internal/share/labels.go`
- Create: `cc-deck/internal/share/labels_test.go`
- Modify: `cc-deck/internal/share/invitation.go`
- Modify: `cc-deck/internal/share/invitation_test.go`
- Modify: `cc-deck/internal/share/state_test.go`

- [ ] **Step 1: Write failing model and label tests**

Add tests for multiple invitations per role, label uniqueness, and secret-free YAML:

```go
func TestSharingOperationPersistsInvitationMetadataWithoutSecrets(t *testing.T) {
    op := SharingOperation{Workspace: "demo", Invitations: []InvitationRecord{{Label: "brave-otter", Role: RoleInteractive, State: InvitationActive}}}
    raw, err := yaml.Marshal(op)
    require.NoError(t, err)
    require.Contains(t, string(raw), "brave-otter")
    require.NotContains(t, string(raw), "token")
}

func TestLabelGeneratorRetriesCollisions(t *testing.T) {
    generator := NewLabelGenerator(strings.NewReader(collisionThenUniqueBytes))
    got, err := generator.Next(map[string]bool{"brave-otter": true})
    require.NoError(t, err)
    require.NotEqual(t, "brave-otter", got)
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
cd cc-deck && go test ./internal/share -run 'InvitationMetadata|LabelGenerator|BuildInvitation' -count=1
```

Expected: FAIL because invitation records and label generator do not exist.

- [ ] **Step 3: Replace fixed labels with invitation records**

Define:

```go
type InvitationRole string
const (
    RoleInteractive InvitationRole = "interactive"
    RoleObserver    InvitationRole = "observer"
)
type InvitationState string
const (
    InvitationActive  InvitationState = "active"
    InvitationRevoked InvitationState = "revoked"
)
type InvitationRecord struct {
    Label     string          `yaml:"label"`
    Role      InvitationRole  `yaml:"role"`
    State     InvitationState `yaml:"state"`
    CreatedAt time.Time       `yaml:"created_at"`
}
```

Add `Workspace string` and `Invitations []InvitationRecord` to `SharingOperation`; remove the two fixed token-label fields.

- [ ] **Step 4: Implement memorable random labels**

Use fixed adjective and noun tables plus `crypto/rand`-backed indices. `Next(existing)` must try at most 64 combinations and return `generate unique invitation label` on exhaustion. Accept an injected `io.Reader` in tests.

- [ ] **Step 5: Build one invitation at a time**

Replace the four-field builder with:

```go
type Invitation struct {
    Label, Browser, Terminal string
    Role InvitationRole
    Warnings []string
}

func BuildInvitation(endpoint, session, label, token string, role InvitationRole) (Invitation, error) {
    remote, err := sessionURL(endpoint, session)
    if err != nil { return Invitation{}, err }
    return Invitation{Label: label, Role: role, Browser: browserInvitation(remote, token), Terminal: terminalInvitation(remote, token), Warnings: []string{TerminalTLSWarning}}, nil
}
```

Interactive invitations additionally include `TrustedControlWarning`.

- [ ] **Step 6: Run sharing model tests**

Run:

```bash
cd cc-deck && go test ./internal/share -run 'Model|Invitation|Label|State' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add cc-deck/internal/share
git commit -m "feat(share): support named invitations"
```

## Task 5: Add Workspace-Aware Start, Invite, Revoke, and Session-Death Cleanup

**Files:**
- Modify: `cc-deck/internal/share/provider.go`
- Modify: `cc-deck/internal/share/service.go`
- Modify: `cc-deck/internal/share/service_test.go`
- Modify: `cc-deck/internal/share/provider_contract_test.go`
- Modify: `cc-deck/internal/share/zellij.go`
- Modify: `cc-deck/internal/share/zellij_test.go`
- Modify: `cc-deck/internal/share/guard.go`
- Modify: `cc-deck/internal/share/guard_test.go`
- Modify: `cc-deck/internal/share/test_fakes_test.go`

- [ ] **Step 1: Write failing service behavior tests**

Cover these exact behaviors:

```go
func TestStartRecordsWorkspaceAndCreatesTwoInitialInvitations(t *testing.T)
func TestStartRejectsDifferentWorkspaceWhileActive(t *testing.T)
func TestInviteAddsIndependentCredential(t *testing.T)
func TestRevokeOnlyRevokesNamedCredential(t *testing.T)
func TestStopRevokesEveryActiveCredential(t *testing.T)
func TestGuardStopsSharingWhenCanonicalSessionDisappears(t *testing.T)
```

Assert that `Start` rejects a missing canonical session before creating a web server, token, or endpoint. Session-creation rollback belongs to the workspace command orchestration in Task 6.

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
cd cc-deck && go test ./internal/share -run 'Test(StartRecordsWorkspace|StartRejectsDifferent|InviteAdds|RevokeOnly|StopRevokesEvery|GuardStopsSharing)' -count=1
```

Expected: FAIL because the new API is absent.

- [ ] **Step 3: Replace the Zellij interface**

Use this contract:

```go
type Zellij interface {
    ValidateCapabilities(context.Context) error
    SessionExists(context.Context, string) (bool, error)
    CreateToken(context.Context, string, bool) (string, error)
    RevokeToken(context.Context, string) error
    EnsureWebServer(context.Context) (string, bool, error)
    StopWebServer(context.Context) error
}
```

Delete `ShareSession`, `UnshareSession`, and the invalid `--session ... options` calls. Shared-session creation is owned by `LocalWorkspace.EnsureSession` from Task 2; this adapter only validates that the expected canonical session still exists.

- [ ] **Step 4: Expand the service API**

Define:

```go
type StartRequest struct { Workspace, Session, Provider string }
type InviteRequest struct { Label string; Role InvitationRole }
type Service interface {
    Start(context.Context, StartRequest) ([]Invitation, error)
    Invite(context.Context, InviteRequest) (Invitation, error)
    Revoke(context.Context, string) (SharingStatus, error)
    Status(context.Context) (SharingStatus, error)
    Stop(context.Context) (SharingStatus, error)
}

type SharingStatus struct {
    State LifecycleState
    Workspace, Session, Provider, EndpointURL string
    Invitations []InvitationRecord
    GuardReady bool
    Residuals []string
}
```

Start requires the workspace readiness layer to have created the shared canonical session. It verifies session existence before creating the web server, tokens, or provider, generates one interactive and one observer label, saves only metadata, and returns both invitations only after the guard is ready.

- [ ] **Step 5: Implement independent invitation creation and revocation**

Under the existing lifecycle lock, `Invite` validates active/healthy state, generates or validates a unique label, creates one role-specific Zellij token, appends metadata, persists it, and returns the one-time invitation. If persistence fails, revoke the newly created token before returning.

`Revoke` finds the label, calls Zellij revocation only when active, marks it revoked, and persists the tombstone so repeated revocation remains idempotent.

- [ ] **Step 6: Generalize teardown and guard watching**

Iterate all active invitation records during teardown. Add `session string` to guard watch state and on every 250 ms tick check both provider status and `SessionExists`. Missing session invokes the same `Service.Stop` path. Do not reacquire the lifecycle lock recursively.

- [ ] **Step 7: Run the complete sharing package**

Run:

```bash
cd cc-deck && go test ./internal/share -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add cc-deck/internal/share
git commit -m "feat(share): manage workspace invitations"
```

## Task 6: Integrate Sharing Into Workspace Commands

**Files:**
- Create: `cc-deck/internal/cmd/ws_share.go`
- Create: `cc-deck/internal/cmd/ws_share_test.go`
- Modify: `cc-deck/internal/cmd/ws.go`
- Modify: `cc-deck/internal/cmd/ws_new_test.go`
- Modify: `cc-deck/internal/cmd/ws_integration_test.go`
- Modify: `cc-deck/internal/cmd/ws_promote.go`
- Modify: `cc-deck/internal/cmd/ws_promote_test.go`

- [ ] **Step 1: Write failing Cobra tests**

Test these invocations through injected workspace/readiness/sharing fakes:

```go
func TestWsNewDefaultsToReadyPrivate(t *testing.T)
func TestWsNewNoStartSkipsReadiness(t *testing.T)
func TestWsNewRejectsNoStartWithShare(t *testing.T)
func TestWsStartSharePrintsTwoInvitations(t *testing.T)
func TestWsAttachAutoStartsPrivateAndAnnouncesWork(t *testing.T)
func TestWsAttachShareRejectsExistingPrivateSession(t *testing.T)
func TestWsInviteRequiresRoleAndPrintsOneSecret(t *testing.T)
func TestWsRevokeUsesInvitationLabel(t *testing.T)
func TestWsUnshareKeepsSessionRunning(t *testing.T)
func TestWsStopUnsharesBeforeStoppingInfrastructure(t *testing.T)
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
cd cc-deck && go test ./internal/cmd -run 'TestWs(NewDefaults|NewNoStart|NewRejects|StartShare|AttachAuto|AttachShare|Invite|Revoke|Unshare)' -count=1
```

Expected: FAIL because flags and subcommands are absent.

- [ ] **Step 3: Add flags and common lifecycle orchestration**

Add `share bool` and `noStart bool` to `newFlags`. Register `--share`, `--no-start`, and `--share` on start/attach. Validate:

```go
if cf.share && cf.noStart {
    return fmt.Errorf("--share and --no-start cannot be used together")
}
```

After successful create, call readiness unless `--no-start`. For infrastructure backends, `--no-start` calls `InfraManager.Stop` after creation and records session absent.

Use one helper that inspects both workspace and sharing state:

```go
type readyRunner func(context.Context, ws.Workspace, ws.ReadyOptions) (ws.ReadyResult, error)

func ensureWorkspaceReady(ctx context.Context, workspace ws.Workspace, share bool, current sharing.SharingStatus, run readyRunner) (ws.ReadyResult, error) {
    status, err := workspace.Status(ctx)
    if err != nil { return ws.ReadyResult{}, err }
    if share && status.SessionState == ws.SessionStateExists {
        if current.State == sharing.StateActive && current.Workspace == workspace.Name() {
            return ws.ReadyResult{SessionName: ws.ZellijSessionName(workspace.Name())}, nil
        }
        return ws.ReadyResult{}, fmt.Errorf("workspace %q already has a private session; restart it for sharing:\n  cc-deck ws kill-session %s\n  cc-deck ws start %s --share", workspace.Name(), workspace.Name(), workspace.Name())
    }
    return run(ctx, workspace, ws.ReadyOptions{Share: share})
}
```

For shared startup, call `ws.EnsureReady(..., ReadyOptions{Share:true})` first, then call sharing `Start`. If sharing startup fails and readiness reported `SessionCreated:true`, call `workspace.KillSession` to roll back the newly created share-enabled session. Private startup calls `ws.EnsureReady(..., ReadyOptions{})` only.

Change `runWsStop` to call sharing `Stop` before `KillSession` and `InfraManager.Stop`. It must continue with session and infrastructure shutdown when sharing teardown fails, aggregate all residual errors, and return nonzero after every cleanup action has been attempted.

- [ ] **Step 4: Add workspace sharing commands**

In `ws_share.go`, register:

```go
invite := &cobra.Command{Use: "invite [name]", Args: cobra.MaximumNArgs(1)}
invite.Flags().String("role", "", "Invitation role: interactive or observer")
invite.Flags().String("name", "", "Optional invitation label")
_ = invite.MarkFlagRequired("role")
revoke := &cobra.Command{Use: "revoke [name] INVITATION_LABEL", Args: cobra.RangeArgs(1, 2)}
unshare := &cobra.Command{Use: "unshare [name]", Args: cobra.MaximumNArgs(1)}
```

Resolve omitted workspace names through the existing resolver. Use the current configured provider when constructing the service. Print warnings before secrets and role/name labels before browser and terminal forms.

- [ ] **Step 5: Move the hidden guard command**

Register a hidden `ws share-guard` command (not a public `share` family) and change detached launch arguments accordingly. Preserve operation-ID, ready-file, fingerprint, and process-group validation.

- [ ] **Step 6: Make attach announce readiness work**

Before terminal replacement, print only performed actions:

```text
Workspace "demo" is stopped; starting infrastructure…
Canonical session was absent; starting it…
Workspace "demo" is ready.
```

Do not print these lines for an already-ready workspace.

- [ ] **Step 7: Run workspace CLI tests**

Run:

```bash
cd cc-deck && go test ./internal/cmd -run 'TestWs|TestPromoted' -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add cc-deck/internal/cmd/ws.go cc-deck/internal/cmd/ws_share.go cc-deck/internal/cmd/ws_share_test.go cc-deck/internal/cmd/ws_new_test.go cc-deck/internal/cmd/ws_integration_test.go cc-deck/internal/cmd/ws_promote.go cc-deck/internal/cmd/ws_promote_test.go
git commit -m "feat(ws): integrate workspace sharing commands"
```

## Task 7: Reconcile Workspace Status and Remove Standalone Sharing

**Files:**
- Modify: `cc-deck/internal/ws/types.go`
- Modify: `cc-deck/internal/cmd/ws.go`
- Modify: `cc-deck/internal/cmd/ws_integration_test.go`
- Delete: `cc-deck/internal/cmd/share_test.go`
- Modify or Delete: `cc-deck/internal/cmd/share.go`
- Modify: `cc-deck/cmd/cc-deck/main.go`

- [ ] **Step 1: Write failing list/status and root-command tests**

Assert the text table has `INFRA`, `SESSION`, and `SHARING`; structured output has `sharing_state`; active status contains endpoint and invitation labels/roles but no secret. Add:

```go
func TestRootDoesNotRegisterStandaloneShareCommand(t *testing.T) {
    root := newRootCmd()
    _, _, err := root.Find([]string{"share"})
    require.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
cd cc-deck && go test ./internal/cmd ./cmd/cc-deck -run 'Test.*(SharingColumn|SharingState|StandaloneShare)' -count=1
```

Expected: FAIL because status lacks sharing and root still registers `share`.

- [ ] **Step 3: Add safe sharing status fields**

Define in `ws/types.go`:

```go
type WorkspaceSharingState string
const (
    SharingPrivate  WorkspaceSharingState = "private"
    SharingShared   WorkspaceSharingState = "shared"
    SharingDegraded WorkspaceSharingState = "degraded"
    SharingUnsupported WorkspaceSharingState = "unsupported"
)
type InvitationSummary struct { Label string `json:"label"`; Role string `json:"role"` }
```

Extend command output DTOs, not persisted workspace instances, with sharing state, endpoint, invitation summaries, guard health, and residuals loaded from the sharing service during list/status reconciliation.

- [ ] **Step 4: Render the new state**

Add `SHARING` to text tables and `sharing_state` to JSON/YAML. For local workspaces with no active operation render `private`; for non-local backends render `unsupported`; render `degraded` whenever persisted residuals exist.

- [ ] **Step 5: Remove standalone command registration**

Delete `cmd.NewShareCmd(gf)` from `main.go`. Remove obsolete public Cobra construction and tests from `share.go`; retain `osCommandRunner` in `ws_share.go` or a focused runner file.

- [ ] **Step 6: Run command and main-package tests**

Run:

```bash
cd cc-deck && go test ./internal/cmd ./cmd/cc-deck -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add cc-deck/internal/ws/types.go cc-deck/internal/cmd cc-deck/cmd/cc-deck/main.go
git commit -m "feat(ws): expose workspace sharing status"
```

## Task 8: Update User Documentation and Acceptance Records

**Files:**
- Modify: `README.md`
- Modify: `docs/modules/reference/pages/cli.adoc`
- Modify: `docs/modules/reference/pages/configuration.adoc`
- Modify: `docs/modules/using/pages/sharing.adoc`
- Modify: `specs/084-zellij-session-sharing/quickstart.md`
- Modify: `specs/084-zellij-session-sharing/tasks.md`

- [ ] **Step 1: Rewrite examples around workspaces**

Document these primary flows exactly:

```bash
cc-deck ws new demo --share
cc-deck ws start demo --share
cc-deck ws attach demo --share
cc-deck ws invite demo --role interactive --name alice
cc-deck ws invite demo --role observer
cc-deck ws revoke demo alice
cc-deck ws unshare demo
```

Explain one local shared workspace, private-by-default recovery, one-time secrets, random labels, local-only backend support, terminal TLS risk, and complete/degraded teardown.

- [ ] **Step 2: Document state output**

Add the `INFRA`, `SESSION`, and `SHARING` model and the `Ctrl+q` transition to both the guide and CLI reference. State that plain attach after session death recreates privately.

- [ ] **Step 3: Record focused automated evidence**

Update quickstart with the exact focused commands and results from Tasks 2–7. Do not mark live T032 or deferred T034 complete.

- [ ] **Step 4: Run manual prose checks**

Run:

```bash
git diff --check
rg -n 'cc-deck share (start|status|stop)' README.md docs specs/084-zellij-session-sharing
```

Expected: no whitespace errors; standalone commands appear only in migration/history text.

- [ ] **Step 5: Commit**

```bash
git add README.md docs specs/084-zellij-session-sharing
git commit -m "docs(share): document workspace sharing UX"
```

## Task 9: Focused Verification and Live-Acceptance Handoff

**Files:**
- Modify: `specs/084-zellij-session-sharing/quickstart.md`
- Modify: `specs/084-zellij-session-sharing/tasks.md`

- [ ] **Step 1: Run focused Go suites**

Run:

```bash
cd cc-deck && go test ./internal/share ./internal/ws ./internal/cmd ./cmd/cc-deck -count=1
```

Expected: PASS with zero failed packages.

- [ ] **Step 2: Run lint**

Run from the repository root:

```bash
make lint
```

Expected: Go vet and Rust clippy complete with exit code 0.

- [ ] **Step 3: Verify command help**

Run through the repository's installed/test binary mechanism:

```bash
make install
cc-deck ws new --help
cc-deck ws start --help
cc-deck ws attach --help
cc-deck ws invite --help
cc-deck ws revoke --help
cc-deck ws unshare --help
```

Expected: flags and arguments match the approved design; `cc-deck share` is unknown.

- [ ] **Step 4: Record verification without hiding deferred failures**

Update quickstart and tasks with focused results. Keep T034 incomplete and link brainstorm 090 until repository-wide `make test` and `make verify` are repaired. Keep T032 incomplete until the ten-repetition four-client matrix succeeds.

- [ ] **Step 5: Commit verification evidence**

```bash
git add specs/084-zellij-session-sharing/quickstart.md specs/084-zellij-session-sharing/tasks.md
git commit -m "docs(share): record workspace sharing verification"
```

## Execution Notes

- Do not run `go build` or `cargo build` directly; repository policy requires Make targets for builds.
- Preserve unrelated user changes in the worktree.
- Never print or persist invitation secrets in test failure messages, YAML fixtures, list output, status output, or logs.
- Treat any existing private session as non-destructive: never kill it merely because `--share` was requested.
- Use a fresh temporary XDG state directory for live acceptance, but use the same directory for every command within one sharing operation so the detached guard can reconcile state.

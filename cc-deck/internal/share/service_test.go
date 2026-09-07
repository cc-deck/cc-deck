package share

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type startStore struct {
	op        *SharingOperation
	saveErr   error
	loadErr   error
	removeErr error
	lockRuns  int
	saves     int
}

func (s *startStore) WithLock(_ context.Context, fn func() error) error { s.lockRuns++; return fn() }
func (s *startStore) Load() (*SharingOperation, error)                  { return s.op, s.loadErr }
func (s *startStore) Save(op *SharingOperation) error {
	s.saves++
	if s.saveErr != nil {
		return s.saveErr
	}
	clone := *op
	s.op = &clone
	return nil
}
func (s *startStore) Remove() error {
	if s.removeErr != nil {
		return s.removeErr
	}
	s.op = nil
	return nil
}

type startZellij struct {
	calls              []string
	fail               string
	cleanupSawCanceled bool
	roles              map[string]bool
	credentialNames    map[string]string
	revokedNames       []string
	sessionMissing     bool
	sessionErr         error
	webStarted         bool
}

func (z *startZellij) call(name string) error {
	z.calls = append(z.calls, name)
	if z.fail == name {
		return errors.New(name + " failed")
	}
	return nil
}
func (z *startZellij) ValidateCapabilities(context.Context) error { return z.call("validate-zellij") }
func (z *startZellij) SessionExists(context.Context, string) (bool, error) {
	if err := z.call("session-exists"); err != nil {
		return false, err
	}
	if z.sessionErr != nil {
		return false, z.sessionErr
	}
	return !z.sessionMissing, nil
}
func (z *startZellij) CreateToken(_ context.Context, label string, readOnly bool) (TokenCredential, error) {
	if z.roles == nil {
		z.roles = map[string]bool{}
	}
	z.roles[label] = readOnly
	name := "interactive-token"
	if readOnly {
		name = "observer-token"
	}
	if err := z.call("create-" + name); err != nil {
		return TokenCredential{}, err
	}
	credentialName := label
	if z.credentialNames[label] != "" {
		credentialName = z.credentialNames[label]
	}
	z.roles[credentialName] = readOnly
	return TokenCredential{Name: credentialName, Secret: name + "-SECRET"}, nil
}
func (z *startZellij) RevokeToken(ctx context.Context, label string) error {
	z.cleanupSawCanceled = z.cleanupSawCanceled || ctx.Err() != nil
	z.revokedNames = append(z.revokedNames, label)
	if z.roles[label] || stringsContains(label, "observer") {
		return z.call("revoke-observer")
	}
	return z.call("revoke-interactive")
}

// EnsureWebServer reports whether it started the server. webStarted defaults to
// false, which models the common case of a user who is already running
// "zellij web" and must keep it.
func (z *startZellij) EnsureWebServer(context.Context) (string, bool, error) {
	if err := z.call("web"); err != nil {
		return "", false, err
	}
	return "http://127.0.0.1:8082", z.webStarted, nil
}
func (z *startZellij) StopWebServer(ctx context.Context) error {
	z.cleanupSawCanceled = z.cleanupSawCanceled || ctx.Err() != nil
	return z.call("stop-web")
}

// fakeEndpoint stands in for a reverse proxy the user runs. It owns no process
// and has no Start or Stop, which is the point.
type fakeEndpoint struct {
	calls      []string
	ref        EndpointRef
	resolveErr error
	probeFn    func(context.Context) (ProbeResult, error)
}

func (e *fakeEndpoint) Name() string { return "fake" }
func (e *fakeEndpoint) Resolve(context.Context) (EndpointRef, error) {
	e.calls = append(e.calls, "resolve")
	if e.resolveErr != nil {
		return EndpointRef{}, e.resolveErr
	}
	ref := e.ref
	if ref.BaseURL == "" {
		ref.BaseURL = "https://public.example"
	}
	return ref, nil
}
func (e *fakeEndpoint) Probe(ctx context.Context, _ EndpointRef, _ string) (ProbeResult, error) {
	e.calls = append(e.calls, "probe")
	if e.probeFn != nil {
		return e.probeFn(ctx)
	}
	return ProbeResult{OK: true, CheckedAt: time.Now().UTC()}, nil
}

// failingEndpoint returns an endpoint whose probe fails at the named stage.
func failingEndpoint(stage ProbeStage) *fakeEndpoint {
	return &fakeEndpoint{probeFn: func(context.Context) (ProbeResult, error) {
		result := ProbeResult{OK: false, FailedAt: stage, Diagnostic: string(stage) + " stage failed", CheckedAt: time.Now().UTC()}
		return result, &ProbeFailedError{Result: result, Endpoint: "https://public.example"}
	}}
}

func stringsContains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}

func TestStartRevealsBothRolesOnlyAfterReadinessAndPersistsNoSecrets(t *testing.T) {
	store, z, endpoint := &startStore{}, &startZellij{}, &fakeEndpoint{}
	got, err := NewService(store, z, endpoint).Start(context.Background(), StartRequest{Workspace: "demo", Session: "selected"})
	require.NoError(t, err)
	require.Contains(t, got[0].Browser, "interactive-token-SECRET")
	require.Contains(t, got[1].Browser, "observer-token-SECRET")
	require.Equal(t, StateActive, store.op.State)
	require.Equal(t, "selected", store.op.Session)
	require.Equal(t, "demo", store.op.Workspace)
	require.Len(t, store.op.Invitations, 2)
	require.NotContains(t, fmt.Sprintf("%+v", store.op), "SECRET")
	require.Equal(t, []string{"validate-zellij", "session-exists", "web", "create-interactive-token", "create-observer-token"}, z.calls)
	require.Equal(t, []string{"resolve", "probe"}, endpoint.calls)
}

func TestStartRejectsMissingCanonicalSessionBeforeResources(t *testing.T) {
	store, z, endpoint := &startStore{}, &startZellij{sessionMissing: true}, &fakeEndpoint{}
	got, err := NewService(store, z, endpoint).Start(context.Background(), StartRequest{Workspace: "demo", Session: "selected"})
	require.ErrorContains(t, err, "canonical session")
	require.Empty(t, got)
	require.Empty(t, endpoint.calls, "no endpoint work before the session is known to exist")
	require.Equal(t, []string{"validate-zellij", "session-exists"}, z.calls)
}

func TestStartRefusesWhenNoEndpointResolves(t *testing.T) {
	store, z := &startStore{}, &startZellij{}
	endpoint := &fakeEndpoint{resolveErr: errors.New("no sharing endpoint is configured")}
	got, err := NewService(store, z, endpoint).Start(context.Background(), StartRequest{Workspace: "demo", Session: "selected"})
	require.ErrorContains(t, err, "no sharing endpoint is configured")
	require.Empty(t, got)
	require.NotContains(t, z.calls, "create-interactive-token", "no credential is minted without an endpoint")
	require.Nil(t, store.op)
}

func TestInviteAddsIndependentCredentialAndRevokeOnlyNamedCredential(t *testing.T) {
	store, z := &startStore{op: activeOperation()}, &startZellij{credentialNames: map[string]string{"alice": "swift-seal"}}
	store.op.Workspace = "demo"
	service := NewService(store, z, &fakeEndpoint{})
	invitation, err := service.Invite(context.Background(), InviteRequest{Label: "alice", Role: RoleInteractive})
	require.NoError(t, err)
	require.Equal(t, "alice", invitation.Label)
	require.Len(t, store.op.Invitations, 3)
	require.Equal(t, "swift-seal", store.op.Invitations[2].CredentialName)

	status, err := service.Revoke(context.Background(), "", "alice")
	require.NoError(t, err)
	require.Len(t, status.Invitations, 3)
	require.Equal(t, InvitationRevoked, status.Invitations[2].State)
	require.Equal(t, InvitationActive, status.Invitations[0].State)
	require.Equal(t, []string{"swift-seal"}, z.revokedNames)
}

func TestStartRejectsExistingOperationWithoutCreatingCredentials(t *testing.T) {
	store := &startStore{op: &SharingOperation{Session: "existing", Workspace: "alpha", State: StateActive}}
	z, endpoint := &startZellij{}, &fakeEndpoint{}
	got, err := NewService(store, z, endpoint).Start(context.Background(), StartRequest{Session: "other"})
	require.ErrorContains(t, err, "already shared")
	require.Empty(t, got)
	require.Empty(t, z.calls)
	require.Empty(t, endpoint.calls)
}

func TestStartSameActiveSessionIsIdempotentWithoutSecretReissue(t *testing.T) {
	store := &startStore{op: &SharingOperation{Session: "selected", State: StateActive}}
	z, endpoint := &startZellij{}, &fakeEndpoint{}
	got, err := NewService(store, z, endpoint).Start(context.Background(), StartRequest{Session: "selected"})
	require.NoError(t, err)
	require.Empty(t, got)
	require.Empty(t, z.calls)
	require.Empty(t, endpoint.calls)
}

func TestStartWithoutSelectorRecognizesExistingActiveOperation(t *testing.T) {
	store := &startStore{op: &SharingOperation{Session: "selected", State: StateActive}}
	z, endpoint := &startZellij{}, &fakeEndpoint{}
	got, err := NewService(store, z, endpoint).Start(context.Background(), StartRequest{})
	require.NoError(t, err)
	require.Empty(t, got)
	require.Empty(t, z.calls)
	require.Empty(t, endpoint.calls)
}

func TestStartReconcilesResidualOperationBeforeIssuingNewInvitations(t *testing.T) {
	residual := activeOperation()
	residual.State = StateDegraded
	store, z := &startStore{op: residual}, &startZellij{}
	got, err := NewService(store, z, &fakeEndpoint{}).Start(context.Background(), StartRequest{Session: "replacement"})
	require.NoError(t, err)
	require.Contains(t, got[0].Browser, "SECRET")
	require.Equal(t, "replacement", store.op.Session)
	require.Contains(t, z.calls, "revoke-observer")
	require.Contains(t, z.calls, "revoke-interactive")
}

func TestStartRefusesWhenStaleReconciliationLeavesResidualExposure(t *testing.T) {
	residual := activeOperation()
	residual.State = StateDegraded
	store, z := &startStore{op: residual}, &startZellij{fail: "revoke-interactive"}
	got, err := NewService(store, z, &fakeEndpoint{}).Start(context.Background(), StartRequest{Session: "replacement"})
	require.ErrorContains(t, err, "stale sharing resources remain")
	require.Empty(t, got)
	require.Equal(t, StateDegraded, store.op.State)
	require.NotContains(t, z.calls, "create-interactive-token")
}

func TestStartRollbackIsReverseOrderedAndReturnsNoInvitation(t *testing.T) {
	store, z := &startStore{}, &startZellij{webStarted: true}
	got, err := NewService(store, z, failingEndpoint(StageWebSocket)).Start(context.Background(), StartRequest{Session: "selected"})
	require.Error(t, err)
	require.Empty(t, got)
	require.Equal(t, []string{"validate-zellij", "session-exists", "web", "create-interactive-token", "create-observer-token", "revoke-observer", "revoke-interactive", "stop-web"}, z.calls)
}

func TestStartCompensatesEveryMutationFailurePoint(t *testing.T) {
	for _, tc := range []struct {
		name     string
		zFail    string
		endpoint *fakeEndpoint
	}{
		{"web", "web", &fakeEndpoint{}},
		{"interactive token", "create-interactive-token", &fakeEndpoint{}},
		{"observer token", "create-observer-token", &fakeEndpoint{}},
		{"verification", "", failingEndpoint(StageHTTP)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, z := &startStore{}, &startZellij{fail: tc.zFail}
			got, err := NewService(store, z, tc.endpoint).Start(context.Background(), StartRequest{Session: "selected"})
			require.Error(t, err)
			require.Empty(t, got)
			require.Nil(t, store.op, "a failed start leaves no operation behind")
		})
	}
}

func TestStartSerializesThroughStoreLock(t *testing.T) {
	store := &startStore{}
	_, err := NewService(store, &startZellij{}, &fakeEndpoint{}).Start(context.Background(), StartRequest{Session: "selected"})
	require.NoError(t, err)
	require.Equal(t, 1, store.lockRuns)
}

func TestStartRollbackUsesIndependentContextAfterCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store, z := &startStore{}, &startZellij{webStarted: true}
	endpoint := &fakeEndpoint{probeFn: func(context.Context) (ProbeResult, error) {
		cancel()
		return ProbeResult{CheckedAt: time.Now().UTC()}, context.Canceled
	}}
	_, err := NewService(store, z, endpoint).Start(ctx, StartRequest{Session: "selected"})
	require.ErrorIs(t, err, context.Canceled)
	require.Contains(t, z.calls, "revoke-observer")
	require.Contains(t, z.calls, "revoke-interactive")
	require.Contains(t, z.calls, "stop-web")
	require.False(t, z.cleanupSawCanceled)
}

func activeOperation() *SharingOperation {
	return &SharingOperation{
		ID: "operation", Session: "selected",
		EndpointURL: "https://public.example",
		Invitations: []InvitationRecord{
			{Label: "interactive-label", Role: RoleInteractive, State: InvitationActive},
			{Label: "observer-label", Role: RoleObserver, State: InvitationActive},
		},
		State: StateActive,
	}
}

func TestStopAttemptsEverySafetyActionAndRemovesState(t *testing.T) {
	op := activeOperation()
	op.WebServerOwned = true
	store, z := &startStore{op: op}, &startZellij{}
	got, err := NewService(store, z, &fakeEndpoint{}).Stop(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, StateInactive, got.State)
	require.Nil(t, store.op)
	require.Equal(t, []string{"revoke-observer", "revoke-interactive", "stop-web"}, z.calls)
}

func TestStopIsIdempotentWhenNoOperationExists(t *testing.T) {
	store, z := &startStore{}, &startZellij{}
	got, err := NewService(store, z, &fakeEndpoint{}).Stop(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, StateInactive, got.State)
	require.Empty(t, z.calls)
}

func TestStopContinuesAfterFailuresAndPersistsSafeResiduals(t *testing.T) {
	op := activeOperation()
	op.WebServerOwned = true
	store := &startStore{op: op}
	z := &startZellij{fail: "revoke-observer"}
	got, err := NewService(store, z, &fakeEndpoint{}).Stop(context.Background(), "")
	require.ErrorContains(t, err, "cleanup incomplete")
	require.Equal(t, StateDegraded, got.State)
	require.Contains(t, got.Residuals[0], "observer credential")
	require.NotContains(t, fmt.Sprintf("%+v", got), "SECRET")
	require.Equal(t, []string{"revoke-observer", "revoke-interactive", "stop-web"}, z.calls)
	require.Equal(t, StateDegraded, store.op.State)
}

func countString(values []string, want string) int {
	count := 0
	for _, value := range values {
		if value == want {
			count++
		}
	}
	return count
}

func TestStatusReportsActiveWithoutSecrets(t *testing.T) {
	store, z := &startStore{op: activeOperation()}, &startZellij{}
	got, err := NewService(store, z, &fakeEndpoint{}).Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, StateActive, got.State)
	require.Equal(t, "https://public.example", got.EndpointURL)
	require.True(t, got.InteractiveAvailable)
	require.True(t, got.ObserverAvailable)
	require.NotContains(t, fmt.Sprintf("%+v", got), "SECRET")
	require.Len(t, got.Invitations, 2)
}

// An unresponsive Zellij server must never be mistaken for a session that
// ended. Tearing down here would revoke live credentials and close a working
// public endpoint, which is exactly what a wedged server once caused.
func TestStatusLeavesSharingIntactWhenZellijIsUnresponsive(t *testing.T) {
	op := activeOperation()
	store := &startStore{op: op}
	z := &startZellij{sessionErr: fmt.Errorf("probe: %w", ErrZellijUnresponsive)}

	got, err := NewService(store, z, &fakeEndpoint{}).Status(context.Background())

	require.Error(t, err)
	require.ErrorIs(t, err, ErrZellijUnresponsive)
	require.Equal(t, StateDegraded, got.State, "reported as degraded to the caller")
	require.Equal(t, "selected", got.Session, "the operation's identity is still reported")
	require.Equal(t, "https://public.example", got.EndpointURL)
	require.Contains(t, got.Residuals[0], "could not be verified")

	require.NotNil(t, store.op, "the operation must survive an inconclusive probe")
	require.Equal(t, StateActive, store.op.State, "degraded state must not be persisted")
	require.NotContains(t, z.calls, "stop-web")
	require.Empty(t, z.revokedNames, "credentials must not be revoked")
}

// The opposite case must keep working: a session positively reported absent is
// real evidence, and cleanup still runs.
func TestStatusStillReconcilesWhenSessionIsPositivelyGone(t *testing.T) {
	op := activeOperation()
	op.WebServerOwned = true
	store := &startStore{op: op}
	z := &startZellij{sessionMissing: true}

	got, err := NewService(store, z, &fakeEndpoint{}).Status(context.Background())

	require.NoError(t, err)
	require.Equal(t, StateInactive, got.State)
	require.Nil(t, store.op)
	require.Contains(t, z.calls, "stop-web")
}

func TestStatusReconcilesResidualOperationEvenWhenTheSessionIsAlive(t *testing.T) {
	op := activeOperation()
	op.State = StateDegraded
	op.WebServerOwned = true
	op.Residuals = []string{"previous cleanup failed"}
	store, z := &startStore{op: op}, &startZellij{}
	got, err := NewService(store, z, &fakeEndpoint{}).Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, StateInactive, got.State)
	require.Equal(t, []string{"session-exists", "revoke-observer", "revoke-interactive", "stop-web"}, z.calls)
}

func TestStopAttemptsEverySafetyActionWhenStoppingStateCannotBePersisted(t *testing.T) {
	op := activeOperation()
	op.WebServerOwned = true
	store := &startStore{op: op, saveErr: errors.New("disk unavailable")}
	z := &startZellij{}
	got, err := NewService(store, z, &fakeEndpoint{}).Stop(context.Background(), "")
	require.ErrorContains(t, err, "stopping state could not be persisted")
	require.Equal(t, StateDegraded, got.State)
	require.Equal(t, []string{"revoke-observer", "revoke-interactive", "stop-web"}, z.calls)
}

func TestWorkspaceScopedMutationsRejectDifferentWorkspace(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*SharingService, *startStore) error
	}{
		{"invite", func(service *SharingService, _ *startStore) error {
			_, err := service.Invite(context.Background(), InviteRequest{Workspace: "beta", Role: RoleObserver})
			return err
		}},
		{"revoke", func(service *SharingService, store *startStore) error {
			_, err := service.Revoke(context.Background(), "beta", store.op.Invitations[0].Label)
			return err
		}},
		{"stop", func(service *SharingService, _ *startStore) error {
			_, err := service.Stop(context.Background(), "beta")
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, z, endpoint := &startStore{op: activeOperation()}, &startZellij{}, &fakeEndpoint{}
			store.op.Workspace = "alpha"
			err := tc.run(NewService(store, z, endpoint), store)
			require.ErrorContains(t, err, `workspace "beta" is not shared`)
			require.Empty(t, z.calls)
			require.Empty(t, endpoint.calls)
		})
	}
}

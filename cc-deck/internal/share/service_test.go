package share

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	// A faithful copy: the real store serialises, so later appends to the
	// caller's slices must not show up in what was "written".
	clone := *op
	clone.Invitations = append([]InvitationRecord(nil), op.Invitations...)
	clone.Residuals = append([]string(nil), op.Residuals...)
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

// observingZellij records what the store holds at the moment each resource is
// about to be created, which is what an interrupted start would leave behind.
type observingZellij struct {
	*startZellij
	store *startStore
	seen  []string
}

func (z *observingZellij) observe() {
	op := z.store.op
	if op == nil {
		z.seen = append(z.seen, "nothing on disk")
		return
	}
	credentials := 0
	for _, invitation := range op.Invitations {
		if invitation.CredentialName != "" {
			credentials++
		}
	}
	// records and credentials are reported separately so that a record
	// persisted before its credential exists shows up as a mismatch.
	z.seen = append(z.seen, fmt.Sprintf("%s owned=%t records=%d credentials=%d", op.State, op.WebServerOwned, len(op.Invitations), credentials))
}

func (z *observingZellij) EnsureWebServer(ctx context.Context) (string, bool, error) {
	z.observe()
	return z.startZellij.EnsureWebServer(ctx)
}

func (z *observingZellij) CreateToken(ctx context.Context, label string, readOnly bool) (TokenCredential, error) {
	z.observe()
	return z.startZellij.CreateToken(ctx, label, readOnly)
}

// Everything Start creates is on disk before and after it is created. A start
// killed part way through then leaves an operation the next command
// reconciles, instead of live tokens and a web server nothing remembers.
func TestStartRecordsEachResourceOnDiskBeforeCreatingTheNext(t *testing.T) {
	store := &startStore{}
	z := &observingZellij{startZellij: &startZellij{webStarted: true}, store: store}

	_, err := NewService(store, z, &fakeEndpoint{}).Start(context.Background(), StartRequest{Workspace: "demo", Session: "selected"})

	require.NoError(t, err)
	starting := string(StateStarting)
	require.Equal(t, []string{
		starting + " owned=false records=0 credentials=0", // before the web server is started
		starting + " owned=true records=0 credentials=0",  // before the first token is minted
		starting + " owned=true records=1 credentials=1",  // before the second token is minted
	}, z.seen, "a persisted record never names a credential that was not minted")
	require.Equal(t, StateActive, store.op.State)
}

// Persisting early must not leave a record behind when a later step fails and
// rollback succeeds in full.
func TestStartRollbackRemovesTheRecordItPersisted(t *testing.T) {
	store := &startStore{}
	z := &startZellij{webStarted: true, fail: "create-observer-token"}

	_, err := NewService(store, z, &fakeEndpoint{}).Start(context.Background(), StartRequest{Workspace: "demo", Session: "selected"})

	require.Error(t, err)
	require.Equal(t, 3, store.saves, "checkpointed before the web server, after it, and after the first token")
	require.Nil(t, store.op, "a clean rollback leaves no record")
	require.Contains(t, z.calls, "stop-web")
	require.Len(t, z.revokedNames, 1, "the interactive credential is revoked")
}

// A record left by a start that was killed after minting one credential is
// reconciled by the next start: the credential it names is revoked, the web
// server it owned is stopped, and only then does the new share begin.
func TestStartReconcilesARecordLeftByAKilledStart(t *testing.T) {
	store := &startStore{op: &SharingOperation{
		ID: "killed", Workspace: "demo", Session: "selected", State: StateStarting,
		WebServerOwned: true,
		Invitations: []InvitationRecord{
			{Label: "first", CredentialName: "orphaned-interactive", Role: RoleInteractive, State: InvitationActive},
		},
	}}
	z := &startZellij{webStarted: true}

	got, err := NewService(store, z, &fakeEndpoint{}).Start(context.Background(), StartRequest{Workspace: "demo", Session: "selected"})

	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Contains(t, z.revokedNames, "orphaned-interactive", "the credential the killed start minted is revoked")
	require.Equal(t, []string{"revoke-interactive", "stop-web", "validate-zellij", "session-exists", "web", "create-interactive-token", "create-observer-token"}, z.calls,
		"cleanup of the old record completes before anything new is created")
	require.Equal(t, StateActive, store.op.State)
	require.Len(t, store.op.Invitations, 2)
	require.NotContains(t, []string{store.op.Invitations[0].CredentialName, store.op.Invitations[1].CredentialName}, "orphaned-interactive")
}

// When the record cannot be removed during rollback, the failure is reported
// as a residual and the record is left degraded rather than silently kept.
func TestStartRollbackReportsARecordItCouldNotRemove(t *testing.T) {
	store := &startStore{removeErr: errors.New("disk unavailable")}
	z := &startZellij{webStarted: true, fail: "create-observer-token"}

	_, err := NewService(store, z, &fakeEndpoint{}).Start(context.Background(), StartRequest{Workspace: "demo", Session: "selected"})

	require.Error(t, err)
	require.ErrorContains(t, err, "rollback residuals")
	require.ErrorContains(t, err, "could not be removed")
	require.NotNil(t, store.op)
	require.Equal(t, StateDegraded, store.op.State)
	// The degraded record reflects what the rollback already undid, so the
	// next reconcile does not revoke a token Zellij no longer has.
	require.True(t, store.op.WebServerStopped, "the web server stop is recorded")
	require.Len(t, store.op.Invitations, 1)
	require.Equal(t, InvitationRevoked, store.op.Invitations[0].State, "the revoked credential is recorded as revoked")
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
			{Label: "interactive-label", CredentialName: "interactive-label", Role: RoleInteractive, State: InvitationActive},
			{Label: "observer-label", CredentialName: "observer-label", Role: RoleObserver, State: InvitationActive},
		},
		State: StateActive,
	}
}

// A record without a credential name is never revoked by label: labels are not
// Zellij token names, and an unknown-token answer now counts as revoked, so
// that call would silently mark a live credential as gone.
func TestTeardownNeverRevokesByLabel(t *testing.T) {
	op := activeOperation()
	op.Invitations[1].CredentialName = ""
	store, z := &startStore{op: op}, &startZellij{}

	got, err := NewService(store, z, &fakeEndpoint{}).Stop(context.Background(), "")

	require.Error(t, err)
	require.ErrorContains(t, err, "observer-label")
	require.ErrorContains(t, err, "no credential name")
	require.Equal(t, []string{"interactive-label"}, z.revokedNames, "only the named credential is revoked")
	require.Equal(t, StateDegraded, got.State)
	require.Equal(t, InvitationActive, store.op.Invitations[1].State, "the unnamed record stays active for the user to resolve")

	_, err = NewService(store, z, &fakeEndpoint{}).Revoke(context.Background(), "", "observer-label")
	require.ErrorContains(t, err, "no recorded credential name")
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

// The same discipline applies to every other failure to ask. A binary missing
// from this shell's PATH, a cancelled context, or an unexpected exit says
// nothing about whether the session is alive, and each of them was once taken
// as proof that it had ended.
func TestStatusLeavesSharingIntactWhenZellijCannotBeAsked(t *testing.T) {
	for name, sessionErr := range map[string]error{
		"binary missing":    errors.New(`zellij list-sessions --no-formatting: exec: "zellij": executable file not found in $PATH`),
		"context cancelled": context.Canceled,
		"unexpected exit":   errors.New("zellij list-sessions --no-formatting: exit status 1"),
	} {
		t.Run(name, func(t *testing.T) {
			op := activeOperation()
			op.WebServerOwned = true
			store := &startStore{op: op}
			z := &startZellij{sessionErr: sessionErr}

			got, err := NewService(store, z, &fakeEndpoint{}).Status(context.Background())

			require.Error(t, err)
			require.ErrorIs(t, err, sessionErr)
			require.Equal(t, StateDegraded, got.State, "reported as degraded to the caller")
			require.Contains(t, got.Residuals[0], "could not be verified")

			require.NotNil(t, store.op, "the operation must survive an inconclusive check")
			require.Equal(t, StateActive, store.op.State, "degraded state must not be persisted")
			require.NotContains(t, z.calls, "stop-web")
			require.Empty(t, z.revokedNames, "credentials must not be revoked")
		})
	}
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

// --- US2: diagnosis must never destroy ---------------------------------------

// T033: a failing probe during Status revokes nothing, stops nothing, and
// deletes no state, at every stage and on timeout. This is the defect that
// mattered most: a single listing used to revoke live credentials.
func TestStatusFailingProbeChangesNothingAtAnyStage(t *testing.T) {
	timeout := &fakeEndpoint{probeFn: func(context.Context) (ProbeResult, error) {
		return ProbeResult{CheckedAt: time.Now().UTC()}, context.DeadlineExceeded
	}}
	cases := map[string]*fakeEndpoint{
		"dns":       failingEndpoint(StageDNS),
		"tls":       failingEndpoint(StageTLS),
		"http":      failingEndpoint(StageHTTP),
		"auth":      failingEndpoint(StageAuth),
		"websocket": failingEndpoint(StageWebSocket),
		"timeout":   timeout,
	}
	for name, endpoint := range cases {
		t.Run(name, func(t *testing.T) {
			op := activeOperation()
			op.WebServerOwned = true
			store, z := &startStore{op: op}, &startZellij{}

			status, err := NewService(store, z, endpoint).Status(context.Background())

			require.Error(t, err, "a broken endpoint is reported as an error so scripts can detect it")
			require.Equal(t, StateDegraded, status.State, "reported degraded, never a third value")

			require.NotNil(t, store.op, "the share must still exist afterwards")
			require.Equal(t, StateActive, store.op.State, "an observation must not move the persisted state")
			require.Empty(t, z.revokedNames, "no credential may be revoked by a failed check")
			require.NotContains(t, z.calls, "stop-web", "no web server may be stopped by a failed check")
			for _, invitation := range store.op.Invitations {
				require.Equal(t, InvitationActive, invitation.State, "every invitation stays live")
			}
		})
	}
}

// T038: the completed result is recorded on every check, pass or fail, without
// moving State and without triggering teardown.
func TestStatusRecordsTheProbeResultWithoutMovingState(t *testing.T) {
	t.Run("a failure is recorded", func(t *testing.T) {
		store := &startStore{op: activeOperation()}
		_, err := NewService(store, &startZellij{}, failingEndpoint(StageWebSocket)).Status(context.Background())
		require.Error(t, err)
		require.NotNil(t, store.op.LastProbe)
		require.False(t, store.op.LastProbe.OK)
		require.Equal(t, StageWebSocket, store.op.LastProbe.FailedAt)
		require.False(t, store.op.LastProbe.CheckedAt.IsZero())
		require.Equal(t, StateActive, store.op.State)
	})
	t.Run("a pass is recorded", func(t *testing.T) {
		store := &startStore{op: activeOperation()}
		status, err := NewService(store, &startZellij{}, &fakeEndpoint{}).Status(context.Background())
		require.NoError(t, err)
		require.NotNil(t, store.op.LastProbe)
		require.True(t, store.op.LastProbe.OK)
		require.Empty(t, store.op.LastProbe.FailedAt)
		require.Equal(t, StateActive, status.State)
	})
}

// T035: an endpoint that recovers returns to shared with no user intervention.
func TestStatusReturnsToSharedWhenTheEndpointRecovers(t *testing.T) {
	store, z := &startStore{op: activeOperation()}, &startZellij{}
	broken := true
	endpoint := &fakeEndpoint{probeFn: func(context.Context) (ProbeResult, error) {
		if broken {
			result := ProbeResult{OK: false, FailedAt: StageHTTP, Diagnostic: "endpoint is down", CheckedAt: time.Now().UTC()}
			return result, &ProbeFailedError{Result: result}
		}
		return ProbeResult{OK: true, CheckedAt: time.Now().UTC()}, nil
	}}
	service := NewService(store, z, endpoint)

	status, err := service.Status(context.Background())
	require.Error(t, err)
	require.Equal(t, StateDegraded, status.State)

	broken = false
	status, err = service.Status(context.Background())
	require.NoError(t, err, "recovery needs no user intervention")
	require.Equal(t, StateActive, status.State)
	require.True(t, status.LastProbe.OK)
	require.Empty(t, z.revokedNames, "nothing was destroyed while the endpoint was down")
}

// T053: Status re-probes the address recorded at share time, not whatever the
// configuration currently says, so editing configuration never silently
// retargets a live share.
func TestStatusProbesTheAddressRecordedAtShareTime(t *testing.T) {
	op := activeOperation()
	op.EndpointName = "work"
	op.EndpointURL = "https://recorded.example"
	store := &startStore{op: op}

	var probed EndpointRef
	endpoint := &fakeEndpoint{ref: EndpointRef{Name: "reconfigured", BaseURL: "https://reconfigured.example"}}
	endpoint.probeFn = func(context.Context) (ProbeResult, error) {
		return ProbeResult{OK: true, CheckedAt: time.Now().UTC()}, nil
	}
	recording := &refRecordingEndpoint{fakeEndpoint: endpoint, seen: &probed}

	status, err := NewService(store, &startZellij{}, recording).Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, "https://recorded.example", probed.BaseURL)
	require.Equal(t, "work", probed.Name)
	require.Equal(t, "work", status.EndpointName)
}

type refRecordingEndpoint struct {
	*fakeEndpoint
	seen *EndpointRef
}

func (e *refRecordingEndpoint) Probe(ctx context.Context, ref EndpointRef, session string) (ProbeResult, error) {
	*e.seen = ref
	return e.fakeEndpoint.Probe(ctx, ref, session)
}

// Snapshot is what a listing reads. It must contact nothing at all.
func TestSnapshotReadsStoredStateWithoutProbingOrAskingZellij(t *testing.T) {
	op := activeOperation()
	op.LastProbe = &ProbeResult{OK: true, CheckedAt: time.Now().UTC().Add(-12 * time.Minute)}
	store, z, endpoint := &startStore{op: op}, &startZellij{}, &fakeEndpoint{}

	status, err := NewService(store, z, endpoint).Snapshot(context.Background())

	require.NoError(t, err)
	require.Equal(t, StateActive, status.State)
	require.NotNil(t, status.LastProbe)
	require.True(t, status.LastProbe.OK)
	require.Empty(t, endpoint.calls, "a listing must never probe")
	require.Empty(t, z.calls, "a listing must not even ask zellij whether the session exists")
}

// --- US3: unsharing removes only what cc-deck created ------------------------

// T040: ownership decides. A web server cc-deck started is stopped; one the
// user was already running survives.
func TestTeardownStopsOnlyAWebServerCcDeckStarted(t *testing.T) {
	t.Run("cc-deck started it, so cc-deck stops it", func(t *testing.T) {
		op := activeOperation()
		op.WebServerOwned = true
		store, z := &startStore{op: op}, &startZellij{}
		_, err := NewService(store, z, &fakeEndpoint{}).Stop(context.Background(), "")
		require.NoError(t, err)
		require.Contains(t, z.calls, "stop-web")
	})
	t.Run("the user was already running it, so it survives", func(t *testing.T) {
		op := activeOperation()
		op.WebServerOwned = false
		store, z := &startStore{op: op}, &startZellij{}
		status, err := NewService(store, z, &fakeEndpoint{}).Stop(context.Background(), "")
		require.NoError(t, err)
		require.Equal(t, StateInactive, status.State)
		require.NotContains(t, z.calls, "stop-web", "a web server cc-deck did not start is not cc-deck's to stop")
		require.Contains(t, z.calls, "revoke-observer", "every invitation is still revoked")
		require.Contains(t, z.calls, "revoke-interactive")
	})
}

// T043: ownership is recorded when cc-deck acts, not inferred later.
func TestStartRecordsWebServerOwnershipWhenItActs(t *testing.T) {
	for _, started := range []bool{true, false} {
		t.Run(fmt.Sprintf("started=%v", started), func(t *testing.T) {
			store, z := &startStore{}, &startZellij{webStarted: started}
			_, err := NewService(store, z, &fakeEndpoint{}).Start(context.Background(), StartRequest{Session: "selected"})
			require.NoError(t, err)
			require.Equal(t, started, store.op.WebServerOwned)
		})
	}
}

// T041: teardown never ends the workspace session and never touches the user's
// endpoint. The endpoint has no stop step at all, which is why cc-deck cannot
// stop one even by mistake.
func TestTeardownNeverEndsTheSessionAndNeverTouchesTheEndpoint(t *testing.T) {
	op := activeOperation()
	op.WebServerOwned = true
	store, z, endpoint := &startStore{op: op}, &startZellij{}, &fakeEndpoint{}

	_, err := NewService(store, z, endpoint).Stop(context.Background(), "")
	require.NoError(t, err)

	require.Empty(t, endpoint.calls, "unshare must not contact the endpoint at all, healthy or not")
	for _, call := range z.calls {
		require.NotContains(t, call, "kill", "teardown must never end the workspace session")
		require.NotContains(t, call, "delete-session")
	}
	require.Equal(t, []string{"revoke-observer", "revoke-interactive", "stop-web"}, z.calls)
}

// T042: an interrupted teardown resumes on the next invocation and strands
// nothing. Work already completed is not repeated.
func TestInterruptedTeardownResumesAndStrandsNothing(t *testing.T) {
	op := activeOperation()
	op.WebServerOwned = true
	store := &startStore{op: op}

	// The first attempt revokes the observer credential, then fails to stop the
	// web server.
	first := &startZellij{fail: "stop-web"}
	_, err := NewService(store, first, &fakeEndpoint{}).Stop(context.Background(), "")
	require.Error(t, err)
	require.NotNil(t, store.op, "an incomplete teardown keeps its state so it can be resumed")
	require.Equal(t, StateDegraded, store.op.State)
	for _, invitation := range store.op.Invitations {
		require.Equal(t, InvitationRevoked, invitation.State, "revocation already done is persisted")
	}

	// The second attempt finishes the work, and does not re-revoke.
	second := &startZellij{}
	status, err := NewService(store, second, &fakeEndpoint{}).Stop(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, StateInactive, status.State)
	require.Nil(t, store.op, "nothing is stranded")
	require.Empty(t, second.revokedNames, "credentials already revoked are not revoked again")
	require.Equal(t, 1, countString(second.calls, "stop-web"))
}

// T044 and T045 together: teardown has exactly the steps it is entitled to.
func TestTeardownRetrySkipsWorkAlreadyCompleted(t *testing.T) {
	op := activeOperation()
	op.WebServerOwned = true
	store := &startStore{op: op}

	first := &startZellij{fail: "revoke-observer"}
	_, err := NewService(store, first, &fakeEndpoint{}).Stop(context.Background(), "")
	require.Error(t, err)
	require.Equal(t, 1, countString(first.calls, "stop-web"), "a failed revocation does not block the web server step")

	second := &startZellij{}
	_, err = NewService(store, second, &fakeEndpoint{}).Stop(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, 0, countString(second.calls, "stop-web"), "a web server already stopped is not stopped twice")
	require.Contains(t, second.calls, "revoke-observer", "the credential that failed is retried")
}

// T025: a --no-verify share is created with no recorded result, which is what
// makes a listing show no verification age. It introduces no third state.
func TestNoVerifyStartCreatesAShareWithNoRecordedResult(t *testing.T) {
	store, z, endpoint := &startStore{}, &startZellij{}, &fakeEndpoint{}
	invitations, err := NewService(store, z, endpoint).Start(context.Background(), StartRequest{Session: "selected", NoVerify: true})
	require.NoError(t, err)
	require.Len(t, invitations, 2)
	require.Nil(t, store.op.LastProbe, "a skipped check records no result, not a zero one")
	require.Equal(t, StateActive, store.op.State)
	require.Equal(t, []string{"resolve"}, endpoint.calls, "--no-verify skips the probe entirely")
}

// T024: invite runs the same gate, and leaves no credential behind when it fails.
func TestInviteRunsTheSameGateAndLeavesNoCredentialBehind(t *testing.T) {
	store := &startStore{op: activeOperation()}
	store.op.Workspace = "demo"
	z := &startZellij{}
	_, err := NewService(store, z, failingEndpoint(StageAuth)).Invite(context.Background(), InviteRequest{Label: "alice", Role: RoleObserver})
	require.Error(t, err)
	require.NotContains(t, z.calls, "create-observer-token", "no credential is minted before verification passes")
	require.Len(t, store.op.Invitations, 2, "no invitation is recorded")
}

func TestInviteSkipsTheGateWithNoVerify(t *testing.T) {
	store := &startStore{op: activeOperation()}
	store.op.Workspace = "demo"
	endpoint := failingEndpoint(StageWebSocket)
	invitation, err := NewService(store, &startZellij{}, endpoint).Invite(context.Background(),
		InviteRequest{Label: "alice", Role: RoleObserver, NoVerify: true})
	require.NoError(t, err)
	require.Equal(t, "alice", invitation.Label)
	require.Empty(t, endpoint.calls, "--no-verify skips the probe entirely")
}

// The verification budget is configurable, which is what makes the fifteen
// second default a default rather than a constant. The probe never receives
// more than the configured budget, and receives less only by the grace
// reserved for the revocation it must perform.
func TestVerifyTimeoutIsConfigurable(t *testing.T) {
	for _, budget := range []time.Duration{2 * time.Second, 30 * time.Second, DefaultVerifyTimeout} {
		t.Run(budget.String(), func(t *testing.T) {
			var seenDeadline time.Duration
			endpoint := &fakeEndpoint{probeFn: func(ctx context.Context) (ProbeResult, error) {
				deadline, ok := ctx.Deadline()
				require.True(t, ok, "the probe always runs under a deadline")
				seenDeadline = time.Until(deadline)
				return ProbeResult{OK: true, CheckedAt: time.Now().UTC()}, nil
			}}
			_, err := NewServiceWithTimeout(&startStore{}, &startZellij{}, endpoint, budget).
				Start(context.Background(), StartRequest{Session: "selected"})
			require.NoError(t, err)

			require.LessOrEqual(t, seenDeadline, budget,
				"the probe never gets more than the configured budget")
			require.GreaterOrEqual(t, seenDeadline, budget/2-time.Second,
				"the reservation must not starve the probe")
		})
	}
}

// SC-013 says no credential may be written to disk. Asserting that on a Go
// struct proves the model is right but not that the serializer is: a stray
// field or a custom marshaller would slip past it. This runs a real share
// through the real FileStore and reads the bytes back off disk.
func TestNoCredentialEverReachesTheStateFileOnDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "share.yaml")
	store := NewFileStore(path)
	z := &startZellij{}

	_, err := NewService(store, z, &fakeEndpoint{}).Start(context.Background(),
		StartRequest{Workspace: "demo", Session: "selected"})
	require.NoError(t, err)

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	contents := string(raw)

	// The labels and credential names are expected; the secrets never are.
	require.Contains(t, contents, "credential_name")
	require.NotContains(t, contents, "SECRET",
		"a credential must never appear in the persisted state file")
	require.NotContains(t, contents, "interactive-token-SECRET")
	require.NotContains(t, contents, "observer-token-SECRET")

	// The recorded observation is there, and carries no secret of its own.
	require.Contains(t, contents, "last_probe")
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"sharing state is readable only by its owner")
}

// A listing must not wait behind a concurrent status probe. Snapshot takes no
// lifecycle lock, so a listing stays cheap even while another command is
// holding the lock for the length of a network verification.
func TestSnapshotDoesNotWaitOnTheLifecycleLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "share.yaml")
	store := NewFileStore(path)
	require.NoError(t, store.Save(activeOperation()))

	held := make(chan struct{})
	released := make(chan struct{})
	go func() {
		_ = store.WithLock(context.Background(), func() error {
			close(held)
			<-released
			return nil
		})
	}()
	<-held
	defer close(released)

	done := make(chan SharingStatus, 1)
	go func() {
		status, err := NewService(store, &startZellij{}, &fakeEndpoint{}).Snapshot(context.Background())
		require.NoError(t, err)
		done <- status
	}()

	select {
	case status := <-done:
		require.Equal(t, StateActive, status.State)
	case <-time.After(2 * time.Second):
		t.Fatal("a listing blocked on the lifecycle lock held by another command")
	}
}

// The configured budget must bind whatever the caller's context says. Applying
// it only when the caller had no deadline let a long-lived caller opt out of
// the bound entirely, which is how a configurable timeout stops being one.
func TestVerifyBudgetBindsEvenWhenTheCallerHasALongerDeadline(t *testing.T) {
	var seen time.Duration
	endpoint := &fakeEndpoint{probeFn: func(ctx context.Context) (ProbeResult, error) {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		seen = time.Until(deadline)
		return ProbeResult{OK: true, CheckedAt: time.Now().UTC()}, nil
	}}

	callerCtx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	_, err := NewServiceWithTimeout(&startStore{}, &startZellij{}, endpoint, 4*time.Second).
		Start(callerCtx, StartRequest{Session: "selected"})
	require.NoError(t, err)
	require.Less(t, seen, 4*time.Second,
		"the configured budget must apply even when the caller's deadline is longer")
}

// A caller with a shorter deadline than the configured budget keeps its own.
func TestVerifyKeepsACallerDeadlineThatIsShorter(t *testing.T) {
	var seen time.Duration
	endpoint := &fakeEndpoint{probeFn: func(ctx context.Context) (ProbeResult, error) {
		deadline, _ := ctx.Deadline()
		seen = time.Until(deadline)
		return ProbeResult{OK: true, CheckedAt: time.Now().UTC()}, nil
	}}

	callerCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := NewServiceWithTimeout(&startStore{}, &startZellij{}, endpoint, time.Minute).
		Start(callerCtx, StartRequest{Session: "selected"})
	require.NoError(t, err)
	require.LessOrEqual(t, seen, 500*time.Millisecond,
		"a caller's shorter deadline must not be replaced by the configured budget")
}

// The whole verification, including the credential revocation that runs on
// every exit path, stays inside the configured budget. A bound that is quietly
// larger than it claims is not a bound.
func TestVerifyReservesBudgetForTheRevocationItMustPerform(t *testing.T) {
	var seen time.Duration
	endpoint := &fakeEndpoint{probeFn: func(ctx context.Context) (ProbeResult, error) {
		deadline, _ := ctx.Deadline()
		seen = time.Until(deadline)
		return ProbeResult{OK: true, CheckedAt: time.Now().UTC()}, nil
	}}
	_, err := NewServiceWithTimeout(&startStore{}, &startZellij{}, endpoint, DefaultVerifyTimeout).
		Start(context.Background(), StartRequest{Session: "selected"})
	require.NoError(t, err)
	require.LessOrEqual(t, seen+probeRevokeTimeout, DefaultVerifyTimeout+time.Second,
		"probe budget plus revocation grace must fit inside the configured timeout")
}

// FR-047: an invitation that fails verification records what it saw, so the
// next listing agrees with the failure the user was just shown.
func TestInviteRecordsAFailedVerification(t *testing.T) {
	store := &startStore{op: activeOperation()}
	store.op.Workspace = "demo"
	_, err := NewService(store, &startZellij{}, failingEndpoint(StageWebSocket)).
		Invite(context.Background(), InviteRequest{Label: "alice", Role: RoleObserver})
	require.Error(t, err)
	require.NotNil(t, store.op.LastProbe, "the observation must be recorded, not discarded")
	require.False(t, store.op.LastProbe.OK)
	require.Equal(t, StageWebSocket, store.op.LastProbe.FailedAt)
	require.Equal(t, StateActive, store.op.State, "recording an observation is not a state change")
}

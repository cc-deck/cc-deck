package share

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type startStore struct {
	op        *SharingOperation
	saveErr   error
	loadErr   error
	removeErr error
	lockRuns  int
}

func (s *startStore) WithLock(_ context.Context, fn func() error) error { s.lockRuns++; return fn() }
func (s *startStore) Load() (*SharingOperation, error)                  { return s.op, s.loadErr }
func (s *startStore) Save(op *SharingOperation) error {
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
	return !z.sessionMissing, nil
}
func (z *startZellij) ResolveSession(_ context.Context, requested string) (string, error) {
	if err := z.call("resolve"); err != nil {
		return "", err
	}
	return requested, nil
}
func (z *startZellij) ShareSession(context.Context, string) error {
	return z.call("share")
}
func (z *startZellij) UnshareSession(ctx context.Context, _ string) error {
	z.cleanupSawCanceled = z.cleanupSawCanceled || ctx.Err() != nil
	return z.call("unshare")
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
func (z *startZellij) EnsureWebServer(context.Context) (string, bool, error) {
	if err := z.call("web"); err != nil {
		return "", false, err
	}
	return "http://127.0.0.1:8082", true, nil
}
func (z *startZellij) StopWebServer(ctx context.Context) error {
	z.cleanupSawCanceled = z.cleanupSawCanceled || ctx.Err() != nil
	return z.call("stop-web")
}

type startProvider struct {
	calls              []string
	fail               string
	readyFn            func() error
	cleanupSawCanceled bool
	beforeStop         func() error
}

func (p *startProvider) call(name string) error {
	p.calls = append(p.calls, name)
	if p.fail == name {
		return errors.New(name + " failed")
	}
	return nil
}
func (p *startProvider) Name() string                   { return "cloudflare" }
func (p *startProvider) Validate(context.Context) error { return p.call("validate-provider") }
func (p *startProvider) Start(context.Context, string) (ProviderHandle, error) {
	if err := p.call("provider-start"); err != nil {
		return ProviderHandle{}, err
	}
	return ProviderHandle{PID: 42}, nil
}
func (p *startProvider) Ready(context.Context, ProviderHandle) (ProviderStatus, error) {
	if p.readyFn != nil {
		p.calls = append(p.calls, "provider-ready")
		if err := p.readyFn(); err != nil {
			return ProviderStatus{}, err
		}
	}
	if err := p.call("provider-ready"); err != nil {
		return ProviderStatus{}, err
	}
	return ProviderStatus{State: "ready", EndpointURL: "https://public.example"}, nil
}
func (p *startProvider) Status(context.Context, ProviderHandle) (ProviderStatus, error) {
	return ProviderStatus{}, nil
}
func (p *startProvider) Stop(ctx context.Context, _ ProviderHandle) error {
	p.cleanupSawCanceled = p.cleanupSawCanceled || ctx.Err() != nil
	if p.beforeStop != nil {
		if err := p.beforeStop(); err != nil {
			p.calls = append(p.calls, "provider-stop")
			return err
		}
	}
	return p.call("provider-stop")
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
	store, z, provider := &startStore{}, &startZellij{}, &startProvider{}
	got, err := NewService(store, z, provider).Start(context.Background(), StartRequest{Workspace: "demo", Session: "selected", Provider: "cloudflare"})
	require.NoError(t, err)
	require.Contains(t, got[0].Browser, "interactive-token-SECRET")
	require.Contains(t, got[1].Browser, "observer-token-SECRET")
	require.Equal(t, StateActive, store.op.State)
	require.Equal(t, "selected", store.op.Session)
	require.Equal(t, "demo", store.op.Workspace)
	require.Len(t, store.op.Invitations, 2)
	require.NotContains(t, fmt.Sprintf("%+v", store.op), "SECRET")
	require.Equal(t, []string{"validate-zellij", "session-exists", "web", "create-interactive-token", "create-observer-token"}, z.calls)
	require.Equal(t, []string{"validate-provider", "provider-start", "provider-ready"}, provider.calls)
}

func TestStartRejectsMissingCanonicalSessionBeforeResources(t *testing.T) {
	store, z, provider := &startStore{}, &startZellij{sessionMissing: true}, &startProvider{}
	got, err := NewService(store, z, provider).Start(context.Background(), StartRequest{Workspace: "demo", Session: "selected"})
	require.ErrorContains(t, err, "canonical session")
	require.Empty(t, got)
	require.Equal(t, []string{"validate-provider"}, provider.calls)
	require.Equal(t, []string{"validate-zellij", "session-exists"}, z.calls)
}

func TestInviteAddsIndependentCredentialAndRevokeOnlyNamedCredential(t *testing.T) {
	store, z := &startStore{op: activeOperation()}, &startZellij{credentialNames: map[string]string{"alice": "swift-seal"}}
	store.op.Workspace = "demo"
	service := NewService(store, z, &startProvider{})
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
	store := &startStore{op: &SharingOperation{Session: "existing", State: StateActive}}
	z, provider := &startZellij{}, &startProvider{}
	got, err := NewService(store, z, &statusProvider{startProvider: provider, status: ProviderStatus{State: "ready"}}).Start(context.Background(), StartRequest{Session: "other"})
	require.ErrorContains(t, err, "already shared")
	require.Empty(t, got)
	require.Empty(t, z.calls)
	require.Equal(t, []string{"provider-status"}, provider.calls)
}

func TestStartSameActiveSessionIsIdempotentWithoutSecretReissue(t *testing.T) {
	store := &startStore{op: &SharingOperation{Session: "selected", Provider: "cloudflare", State: StateActive}}
	z, provider := &startZellij{}, &startProvider{}
	got, err := NewService(store, z, &statusProvider{startProvider: provider, status: ProviderStatus{State: "ready"}}).Start(context.Background(), StartRequest{Session: "selected", Provider: "cloudflare"})
	require.NoError(t, err)
	require.Empty(t, got)
	require.Empty(t, z.calls)
	require.Equal(t, []string{"provider-status"}, provider.calls)
}

func TestStartWithoutSelectorRecognizesExistingActiveOperation(t *testing.T) {
	store := &startStore{op: &SharingOperation{Session: "selected", Provider: "cloudflare", State: StateActive}}
	z, provider := &startZellij{}, &startProvider{}
	got, err := NewService(store, z, &statusProvider{startProvider: provider, status: ProviderStatus{State: "ready"}}).Start(context.Background(), StartRequest{Provider: "cloudflare"})
	require.NoError(t, err)
	require.Empty(t, got)
	require.Empty(t, z.calls)
	require.Equal(t, []string{"provider-status"}, provider.calls)
}

func TestStartWithoutSelectorRejectsDifferentProvider(t *testing.T) {
	store := &startStore{op: &SharingOperation{Session: "selected", Provider: "cloudflare", State: StateActive}}
	provider := &startProvider{}
	got, err := NewService(store, &startZellij{}, &statusProvider{startProvider: provider, status: ProviderStatus{State: "ready"}}).Start(context.Background(), StartRequest{Provider: "other"})
	require.ErrorContains(t, err, "already shared")
	require.Empty(t, got)
}

func TestStartReconcilesStaleOperationBeforeIssuingNewInvitations(t *testing.T) {
	store, z := &startStore{op: activeOperation()}, &startZellij{}
	provider := &statusProvider{startProvider: &startProvider{}, status: ProviderStatus{State: "stopped"}}
	got, err := NewService(store, z, provider).Start(context.Background(), StartRequest{Session: "replacement"})
	require.NoError(t, err)
	require.Contains(t, got[0].Browser, "SECRET")
	require.Equal(t, "replacement", store.op.Session)
	require.Equal(t, []string{"provider-status", "provider-stop", "validate-provider", "provider-start", "provider-ready"}, provider.calls)
}

func TestStartRefusesWhenStaleReconciliationLeavesResidualExposure(t *testing.T) {
	store, z := &startStore{op: activeOperation()}, &startZellij{fail: "revoke-interactive"}
	provider := &statusProvider{startProvider: &startProvider{}, status: ProviderStatus{State: "stopped"}}
	got, err := NewService(store, z, provider).Start(context.Background(), StartRequest{Session: "replacement"})
	require.ErrorContains(t, err, "stale sharing resources remain")
	require.Empty(t, got)
	require.Equal(t, StateDegraded, store.op.State)
	require.NotContains(t, z.calls, "create-interactive-token")
}

func TestStartRollbackIsReverseOrderedAndReturnsNoInvitation(t *testing.T) {
	store, z, provider := &startStore{}, &startZellij{}, &startProvider{fail: "provider-ready"}
	got, err := NewService(store, z, provider).Start(context.Background(), StartRequest{Session: "selected"})
	require.Error(t, err)
	require.Empty(t, got)
	require.Equal(t, []string{"validate-provider", "provider-start", "provider-ready", "provider-stop"}, provider.calls)
	require.Equal(t, []string{"validate-zellij", "session-exists", "web", "create-interactive-token", "create-observer-token", "revoke-observer", "revoke-interactive", "stop-web"}, z.calls)
}

func TestStartCompensatesEveryMutationFailurePoint(t *testing.T) {
	tests := []struct{ name, zFail, providerFail string }{
		{"web", "web", ""}, {"interactive token", "create-interactive-token", ""},
		{"observer token", "create-observer-token", ""}, {"provider start", "", "provider-start"},
		{"provider readiness", "", "provider-ready"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store, z, provider := &startStore{}, &startZellij{fail: tc.zFail}, &startProvider{fail: tc.providerFail}
			got, err := NewService(store, z, provider).Start(context.Background(), StartRequest{Session: "selected"})
			require.Error(t, err)
			require.Empty(t, got)
			require.NotContains(t, z.calls, "unshare")
		})
	}
}

func TestStartSerializesThroughStoreLock(t *testing.T) {
	store := &startStore{}
	_, err := NewService(store, &startZellij{}, &startProvider{}).Start(context.Background(), StartRequest{Session: "selected"})
	require.NoError(t, err)
	require.Equal(t, 1, store.lockRuns)
}

func TestStartSuccessfulShareCommandOwnsSessionForRollback(t *testing.T) {
	store, z := &startStore{}, &startZellij{}
	provider := &startProvider{fail: "provider-ready"}
	_, err := NewService(store, z, provider).Start(context.Background(), StartRequest{Session: "selected"})
	require.Error(t, err)
	require.Contains(t, z.calls, "session-exists")
}

func TestStartRollbackUsesIndependentContextAfterCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store, z := &startStore{}, &startZellij{}
	provider := &startProvider{readyFn: func() error {
		cancel()
		return context.Canceled
	}}
	_, err := NewService(store, z, provider).Start(ctx, StartRequest{Session: "selected"})
	require.ErrorIs(t, err, context.Canceled)
	require.Contains(t, provider.calls, "provider-stop")
	require.Contains(t, z.calls, "revoke-observer")
	require.Contains(t, z.calls, "revoke-interactive")
	require.Contains(t, z.calls, "stop-web")
	require.False(t, provider.cleanupSawCanceled)
	require.False(t, z.cleanupSawCanceled)
}

func activeOperation() *SharingOperation {
	return &SharingOperation{
		ID: "operation", Session: "selected", Provider: "cloudflare",
		EndpointURL: "https://public.example",
		Invitations: []InvitationRecord{
			{Label: "interactive-label", Role: RoleInteractive, State: InvitationActive},
			{Label: "observer-label", Role: RoleObserver, State: InvitationActive},
		},
		ProviderHandle: ProviderHandle{PID: 42},
		State:          StateActive,
	}
}

func TestStopAttemptsEverySafetyActionAndRemovesState(t *testing.T) {
	store, z, provider := &startStore{op: activeOperation()}, &startZellij{}, &startProvider{}
	got, err := NewService(store, z, provider).Stop(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, StateInactive, got.State)
	require.Nil(t, store.op)
	require.Equal(t, []string{"provider-stop"}, provider.calls)
	require.Equal(t, []string{"revoke-observer", "revoke-interactive", "stop-web"}, z.calls)
}

func TestStopIsIdempotentWhenNoOperationExists(t *testing.T) {
	store, z, provider := &startStore{}, &startZellij{}, &startProvider{}
	got, err := NewService(store, z, provider).Stop(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, StateInactive, got.State)
	require.Empty(t, z.calls)
	require.Empty(t, provider.calls)
}

func TestStopContinuesAfterFailuresAndPersistsSafeResiduals(t *testing.T) {
	store := &startStore{op: activeOperation()}
	z, provider := &startZellij{fail: "revoke-observer"}, &startProvider{fail: "provider-stop"}
	got, err := NewService(store, z, provider).Stop(context.Background(), "")
	require.ErrorContains(t, err, "cleanup incomplete")
	require.Equal(t, StateDegraded, got.State)
	require.Contains(t, got.Residuals[0], "public endpoint")
	require.Contains(t, got.Residuals[1], "observer credential")
	require.NotContains(t, fmt.Sprintf("%+v", got), "SECRET")
	require.Equal(t, []string{"revoke-observer", "revoke-interactive", "stop-web"}, z.calls)
	require.Equal(t, StateDegraded, store.op.State)
}

func TestStopRetrySkipsPreviouslyCompletedEndpointAndWebCleanup(t *testing.T) {
	for _, tc := range []struct {
		name          string
		firstFailure  string
		secondFailure string
		wantProvider  []string
		wantWebCalls  int
	}{
		{name: "endpoint already stopped", firstFailure: "stop-web", secondFailure: "provider-stop", wantProvider: []string{"provider-stop"}, wantWebCalls: 2},
		{name: "web server already stopped", firstFailure: "provider-stop", secondFailure: "stop-web", wantProvider: []string{"provider-stop", "provider-stop"}, wantWebCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &startStore{op: activeOperation()}
			z, provider := &startZellij{}, &startProvider{}
			if tc.firstFailure == "stop-web" {
				z.fail = tc.firstFailure
			} else {
				provider.fail = tc.firstFailure
			}
			_, err := NewService(store, z, provider).Stop(context.Background(), "")
			require.Error(t, err)

			z.fail, provider.fail = "", ""
			if tc.secondFailure == "stop-web" {
				z.fail = tc.secondFailure
			} else {
				provider.fail = tc.secondFailure
			}
			_, err = NewService(store, z, provider).Stop(context.Background(), "")
			require.NoError(t, err)
			require.Equal(t, tc.wantProvider, provider.calls)
			require.Equal(t, tc.wantWebCalls, countString(z.calls, "stop-web"))
		})
	}
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
	store, z, provider := &startStore{op: activeOperation()}, &startZellij{}, &startProvider{}
	provider.readyFn = func() error { return nil }
	// Status has its own response, independent of readiness.
	providerStatus := provider
	_ = providerStatus
	got, err := NewService(store, z, &statusProvider{startProvider: provider, status: ProviderStatus{State: "ready", EndpointURL: "https://current.example"}}).Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, StateActive, got.State)
	require.Equal(t, "https://current.example", got.EndpointURL)
	require.True(t, got.InteractiveAvailable)
	require.True(t, got.ObserverAvailable)
	require.NotContains(t, fmt.Sprintf("%+v", got), "SECRET")
	require.Len(t, got.Invitations, 2)
}

type statusProvider struct {
	*startProvider
	status    ProviderStatus
	statusErr error
}

func (p *statusProvider) Status(context.Context, ProviderHandle) (ProviderStatus, error) {
	p.calls = append(p.calls, "provider-status")
	return p.status, p.statusErr
}

func TestStatusReconcilesStaleProviderAndReportsInactiveAfterCleanup(t *testing.T) {
	store, z := &startStore{op: activeOperation()}, &startZellij{}
	provider := &statusProvider{startProvider: &startProvider{}, status: ProviderStatus{State: "stopped", Diagnostic: "process exited"}}
	got, err := NewService(store, z, provider).Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, StateInactive, got.State)
	require.Nil(t, store.op)
	require.Equal(t, []string{"provider-status", "provider-stop"}, provider.calls)
	require.Equal(t, 1, store.lockRuns)
}

func TestStatusRetainsDegradedStateWhenStaleReconciliationIsIncomplete(t *testing.T) {
	store, z := &startStore{op: activeOperation()}, &startZellij{fail: "revoke-observer"}
	provider := &statusProvider{startProvider: &startProvider{}, statusErr: errors.New("provider disappeared")}
	got, err := NewService(store, z, provider).Status(context.Background())
	require.Error(t, err)
	require.Equal(t, StateDegraded, got.State)
	require.Contains(t, got.Residuals[0], "observer credential")
	require.Equal(t, 1, store.lockRuns)
}

func TestStatusReconcilesDegradedOperationEvenWhenProviderLooksReady(t *testing.T) {
	op := activeOperation()
	op.State = StateDegraded
	op.Residuals = []string{"previous cleanup failed"}
	store, z := &startStore{op: op}, &startZellij{}
	provider := &statusProvider{startProvider: &startProvider{}, status: ProviderStatus{State: "ready"}}
	got, err := NewService(store, z, provider).Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, StateInactive, got.State)
	require.Equal(t, []string{"provider-status", "provider-stop"}, provider.calls)
	require.Equal(t, []string{"session-exists", "revoke-observer", "revoke-interactive", "stop-web"}, z.calls)
}

func TestStatusReconcilesStoppingOperationEvenWhenProviderLooksStarting(t *testing.T) {
	op := activeOperation()
	op.State = StateStopping
	store, z := &startStore{op: op}, &startZellij{}
	provider := &statusProvider{startProvider: &startProvider{}, status: ProviderStatus{State: "starting"}}
	got, err := NewService(store, z, provider).Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, StateInactive, got.State)
	require.Contains(t, provider.calls, "provider-stop")
}

func TestStopAttemptsEverySafetyActionWhenStoppingStateCannotBePersisted(t *testing.T) {
	store := &startStore{op: activeOperation(), saveErr: errors.New("disk unavailable")}
	z, provider := &startZellij{}, &startProvider{}
	got, err := NewService(store, z, provider).Stop(context.Background(), "")
	require.ErrorContains(t, err, "stopping state could not be persisted")
	require.Equal(t, StateDegraded, got.State)
	require.Equal(t, []string{"provider-stop"}, provider.calls)
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
			store, z, provider := &startStore{op: activeOperation()}, &startZellij{}, &startProvider{}
			store.op.Workspace = "alpha"
			err := tc.run(NewService(store, z, provider), store)
			require.ErrorContains(t, err, `workspace "beta" is not shared`)
			require.Empty(t, z.calls)
			require.Empty(t, provider.calls)
		})
	}
}

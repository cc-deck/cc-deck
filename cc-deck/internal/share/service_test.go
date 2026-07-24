package share

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type startStore struct {
	op       *SharingOperation
	saveErr  error
	lockRuns int
}

func (s *startStore) WithLock(_ context.Context, fn func() error) error { s.lockRuns++; return fn() }
func (s *startStore) Load() (*SharingOperation, error)                  { return s.op, nil }
func (s *startStore) Save(op *SharingOperation) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	clone := *op
	s.op = &clone
	return nil
}
func (s *startStore) Remove() error { s.op = nil; return nil }

type startZellij struct {
	calls              []string
	fail               string
	cleanupSawCanceled bool
}

func (z *startZellij) call(name string) error {
	z.calls = append(z.calls, name)
	if z.fail == name {
		return errors.New(name + " failed")
	}
	return nil
}
func (z *startZellij) ValidateCapabilities(context.Context) error { return z.call("validate-zellij") }
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
func (z *startZellij) CreateToken(_ context.Context, _ string, readOnly bool) (string, error) {
	name := "interactive-token"
	if readOnly {
		name = "observer-token"
	}
	if err := z.call("create-" + name); err != nil {
		return "", err
	}
	return name + "-SECRET", nil
}
func (z *startZellij) RevokeToken(ctx context.Context, label string) error {
	z.cleanupSawCanceled = z.cleanupSawCanceled || ctx.Err() != nil
	if stringsContains(label, "observer") {
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
	got, err := NewService(store, z, provider).Start(context.Background(), StartRequest{Session: "selected", Provider: "cloudflare"})
	require.NoError(t, err)
	require.Contains(t, got.InteractiveBrowser, "interactive-token-SECRET")
	require.Contains(t, got.ObserverBrowser, "observer-token-SECRET")
	require.Equal(t, StateActive, store.op.State)
	require.Equal(t, "selected", store.op.Session)
	require.NotContains(t, fmt.Sprintf("%+v", store.op), "SECRET")
	require.Equal(t, []string{"validate-zellij", "resolve", "share", "web", "create-interactive-token", "create-observer-token"}, z.calls)
	require.Equal(t, []string{"validate-provider", "provider-start", "provider-ready"}, provider.calls)
}

func TestStartRejectsExistingOperationWithoutCreatingCredentials(t *testing.T) {
	store := &startStore{op: &SharingOperation{Session: "existing", State: StateActive}}
	z, provider := &startZellij{}, &startProvider{}
	got, err := NewService(store, z, provider).Start(context.Background(), StartRequest{Session: "other"})
	require.ErrorContains(t, err, "already exists")
	require.Empty(t, got)
	require.Empty(t, z.calls)
	require.Empty(t, provider.calls)
}

func TestStartSameActiveSessionIsIdempotentWithoutSecretReissue(t *testing.T) {
	store := &startStore{op: &SharingOperation{Session: "selected", Provider: "cloudflare", State: StateActive}}
	z, provider := &startZellij{}, &startProvider{}
	got, err := NewService(store, z, provider).Start(context.Background(), StartRequest{Session: "selected", Provider: "cloudflare"})
	require.NoError(t, err)
	require.Empty(t, got.InteractiveBrowser)
	require.Empty(t, got.InteractiveTerminal)
	require.Empty(t, got.ObserverBrowser)
	require.Empty(t, got.ObserverTerminal)
	require.Contains(t, got.Warnings[0], "credentials are not redisplayed")
	require.Empty(t, z.calls)
	require.Empty(t, provider.calls)
}

func TestStartWithoutSelectorRecognizesExistingActiveOperation(t *testing.T) {
	store := &startStore{op: &SharingOperation{Session: "selected", Provider: "cloudflare", State: StateActive}}
	z, provider := &startZellij{}, &startProvider{}
	got, err := NewService(store, z, provider).Start(context.Background(), StartRequest{Provider: "cloudflare"})
	require.NoError(t, err)
	require.Empty(t, got.InteractiveBrowser)
	require.Contains(t, got.Warnings[0], "credentials are not redisplayed")
	require.Empty(t, z.calls)
	require.Empty(t, provider.calls)
}

func TestStartWithoutSelectorRejectsDifferentProvider(t *testing.T) {
	store := &startStore{op: &SharingOperation{Session: "selected", Provider: "cloudflare", State: StateActive}}
	got, err := NewService(store, &startZellij{}, &startProvider{}).Start(context.Background(), StartRequest{Provider: "other"})
	require.ErrorContains(t, err, "already exists")
	require.Empty(t, got)
}

func TestStartRollbackIsReverseOrderedAndReturnsNoInvitation(t *testing.T) {
	store, z, provider := &startStore{}, &startZellij{}, &startProvider{fail: "provider-ready"}
	got, err := NewService(store, z, provider).Start(context.Background(), StartRequest{Session: "selected"})
	require.Error(t, err)
	require.Empty(t, got)
	require.Equal(t, []string{"validate-provider", "provider-start", "provider-ready", "provider-stop"}, provider.calls)
	require.Equal(t, []string{"validate-zellij", "resolve", "share", "web", "create-interactive-token", "create-observer-token", "revoke-observer", "revoke-interactive", "stop-web", "unshare"}, z.calls)
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
			require.Contains(t, z.calls, "unshare")
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
	require.Contains(t, z.calls, "share")
	require.Contains(t, z.calls, "unshare")
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
	require.Contains(t, z.calls, "unshare")
	require.False(t, provider.cleanupSawCanceled)
	require.False(t, z.cleanupSawCanceled)
}

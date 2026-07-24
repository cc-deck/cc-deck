package share

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type guardService struct {
	stops atomic.Int32
	done  chan struct{}
}

func (*guardService) Start(context.Context, StartRequest) (InvitationSet, error) {
	return InvitationSet{}, nil
}
func (*guardService) Status(context.Context) (SharingStatus, error) { return SharingStatus{}, nil }
func (s *guardService) Stop(context.Context) (SharingStatus, error) {
	s.stops.Add(1)
	close(s.done)
	return SharingStatus{State: StateInactive}, nil
}

type lockAwareProvider struct {
	store *guardLockStore
	state string
}

func (*lockAwareProvider) Name() string                   { return "fake" }
func (*lockAwareProvider) Validate(context.Context) error { return nil }
func (*lockAwareProvider) Start(context.Context, string) (ProviderHandle, error) {
	return ProviderHandle{}, nil
}
func (*lockAwareProvider) Ready(context.Context, ProviderHandle) (ProviderStatus, error) {
	return ProviderStatus{}, nil
}
func (p *lockAwareProvider) Status(context.Context, ProviderHandle) (ProviderStatus, error) {
	if p.store.locked.Load() {
		return ProviderStatus{}, errors.New("watch occurred under lifecycle lock")
	}
	return ProviderStatus{State: p.state}, nil
}
func (*lockAwareProvider) Stop(context.Context, ProviderHandle) error { return nil }

type guardLockStore struct {
	*startStore
	locked atomic.Bool
}

func (s *guardLockStore) WithLock(_ context.Context, fn func() error) error {
	s.lockRuns++
	s.locked.Store(true)
	defer s.locked.Store(false)
	return fn()
}

func TestGuardReleasesValidationLockBeforeProviderWatchAndStopsOnce(t *testing.T) {
	store := &guardLockStore{startStore: &startStore{op: activeOperation()}}
	provider := &lockAwareProvider{store: store, state: "stopped"}
	service := &guardService{done: make(chan struct{})}
	ready := filepath.Join(t.TempDir(), "ready")
	err := RunGuard(context.Background(), store, provider, service, store.op.ID, ready)
	require.NoError(t, err)
	require.Equal(t, int32(1), service.stops.Load())
	require.Equal(t, 1, store.lockRuns)
}

func TestGuardSignalStopsThroughOnceLockingServiceAfterValidationLockRelease(t *testing.T) {
	store := &guardLockStore{startStore: &startStore{op: activeOperation()}}
	provider := &lockAwareProvider{store: store, state: "ready"}
	service := &guardService{done: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	ready := filepath.Join(t.TempDir(), "ready")
	done := make(chan error, 1)
	go func() { done <- RunGuard(ctx, store, provider, service, store.op.ID, ready) }()
	require.Eventually(t, func() bool { return fileExists(ready) }, time.Second, 10*time.Millisecond)
	cancel()
	require.NoError(t, <-done)
	require.Equal(t, int32(1), service.stops.Load())
	require.False(t, store.locked.Load())
}

func TestGuardTeardownUsesNormalServiceAndAcquiresLifecycleLockOnlyAfterWatch(t *testing.T) {
	store := &guardLockStore{startStore: &startStore{op: activeOperation()}}
	provider := &lockAwareProvider{store: store, state: "stopped"}
	service := NewService(store, &startZellij{}, provider)
	ready := filepath.Join(t.TempDir(), "ready")
	require.NoError(t, RunGuard(context.Background(), store, provider, service, store.op.ID, ready))
	require.Equal(t, 2, store.lockRuns, "one identity-validation lock and one Service.Stop lock")
	require.Nil(t, store.op)
}

func TestDetachedGuardWaitsForBoundedReadinessHandshake(t *testing.T) {
	process := &fakeProcess{pid: 77}
	runner := &fakeRunner{process: process}
	runner.startFn = func(_ string, args []string) (Process, error) {
		for i, arg := range args {
			if arg == "--ready-file" && i+1 < len(args) {
				require.NoError(t, os.WriteFile(args[i+1], []byte("operation"), 0600))
			}
		}
		return process, nil
	}
	guard := NewDetachedGuard(runner, "cc-deck")
	handle, err := guard.Start(context.Background(), "operation")
	require.NoError(t, err)
	require.Equal(t, GuardHandle{PID: 77, OperationID: "operation", Ready: true}, handle)
}

func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }

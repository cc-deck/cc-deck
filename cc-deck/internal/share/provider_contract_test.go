package share

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"os"
	"sync"
	"testing"
)

func TestProviderRegistrySelectionAndSortedNames(t *testing.T) {
	cloudflare := &namedProvider{contractProvider: contractProvider{}, name: "cloudflare"}
	fake := &namedProvider{contractProvider: contractProvider{}, name: "fake"}
	registry, err := NewProviderRegistry(fake, cloudflare)
	require.NoError(t, err)
	require.Equal(t, []string{"cloudflare", "fake"}, registry.Names())
	selected, err := registry.Get("fake")
	require.NoError(t, err)
	require.Same(t, fake, selected)
	_, err = registry.Get("missing")
	require.ErrorContains(t, err, "available")
}

func TestProviderRegistryRejectsDuplicateNames(t *testing.T) {
	_, err := NewProviderRegistry(&contractProvider{}, &contractProvider{})
	require.ErrorContains(t, err, "more than once")
}

type namedProvider struct {
	contractProvider
	name string
}

func (p *namedProvider) Name() string { return p.name }

type contractProvider struct {
	stopped                                             bool
	validateErr, startErr, readyErr, statusErr, stopErr error
	readyState                                          string
}

func (*contractProvider) Name() string                     { return "fake" }
func (p *contractProvider) Validate(context.Context) error { return p.validateErr }
func (p *contractProvider) Start(context.Context, string) (ProviderHandle, error) {
	return ProviderHandle{PID: 1}, p.startErr
}
func (p *contractProvider) Ready(context.Context, ProviderHandle) (ProviderStatus, error) {
	state := p.readyState
	if state == "" {
		state = "ready"
	}
	return ProviderStatus{State: state, EndpointURL: "https://example.test"}, p.readyErr
}
func (p *contractProvider) Status(context.Context, ProviderHandle) (ProviderStatus, error) {
	return ProviderStatus{State: "ready"}, p.statusErr
}
func (p *contractProvider) Stop(context.Context, ProviderHandle) error {
	p.stopped = true
	return p.stopErr
}
func TestProviderContract(t *testing.T) {
	p := &contractProvider{}
	ctx := context.Background()
	require.NotEmpty(t, p.Name())
	require.NoError(t, p.Validate(ctx))
	h, e := p.Start(ctx, "http://127.0.0.1")
	require.NoError(t, e)
	st, e := p.Ready(ctx, h)
	require.NoError(t, e)
	require.Equal(t, "ready", st.State)
	require.Contains(t, st.EndpointURL, "https://")
	require.NoError(t, p.Stop(ctx, h))
	require.NoError(t, p.Stop(ctx, h))
}

func TestIsolatedProviderFailureContractMatrix(t *testing.T) {
	boom := errors.New("boom")
	ctx := context.Background()
	cases := []struct {
		name string
		p    *contractProvider
		step func(*contractProvider) error
	}{
		{"missing prerequisite", &contractProvider{validateErr: boom}, func(p *contractProvider) error { return p.Validate(ctx) }},
		{"startup failure", &contractProvider{startErr: boom}, func(p *contractProvider) error { _, e := p.Start(ctx, "local"); return e }},
		{"readiness timeout or malformed", &contractProvider{readyErr: boom, readyState: "failed"}, func(p *contractProvider) error { _, e := p.Ready(ctx, ProviderHandle{}); return e }},
		{"unexpected exit status", &contractProvider{statusErr: boom}, func(p *contractProvider) error { _, e := p.Status(ctx, ProviderHandle{}); return e }},
		{"stop failure", &contractProvider{stopErr: boom}, func(p *contractProvider) error { return p.Stop(ctx, ProviderHandle{}) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { require.Error(t, tc.step(tc.p)) })
	}
}

func TestSharedProviderHappyPathContract(t *testing.T) {
	t.Run("isolated fake", func(t *testing.T) { runHappyProviderContract(t, &contractProvider{}) })
	t.Run("cloudflare runner fake", func(t *testing.T) {
		wait := make(chan error, 1)
		var once sync.Once
		proc := &fakeProcess{pid: 88, waitCh: wait}
		proc.onSignalWith = func(signal os.Signal) {
			if signal == os.Interrupt {
				once.Do(func() { wait <- nil })
			}
		}
		r := &fakeRunner{outputs: map[string][]byte{key("cloudflared", []string{"--version"}): []byte("cloudflared 1")}, errors: map[string]error{}, process: proc}
		r.startFn = func(_ string, args []string) (Process, error) {
			for i, a := range args {
				if a == "--logfile" && i+1 < len(args) {
					require.NoError(t, os.WriteFile(args[i+1], []byte("https://contract.trycloudflare.com"), 0600))
				}
			}
			return proc, nil
		}
		runHappyProviderContract(t, NewCloudflareProvider(r))
	})
}

func TestSharedProviderServiceIntegrationContract(t *testing.T) {
	t.Run("isolated fake", func(t *testing.T) {
		runProviderServiceIntegrationContract(t, &contractProvider{})
	})
	t.Run("cloudflare runner fake", func(t *testing.T) {
		wait := make(chan error, 1)
		var once sync.Once
		proc := &fakeProcess{pid: 89, waitCh: wait}
		proc.onSignalWith = func(signal os.Signal) {
			if signal == os.Interrupt {
				once.Do(func() { wait <- nil })
			}
		}
		runner := &fakeRunner{outputs: map[string][]byte{key("cloudflared", []string{"--version"}): []byte("cloudflared 1")}, errors: map[string]error{}, process: proc}
		runner.startFn = func(_ string, args []string) (Process, error) {
			for i, arg := range args {
				if arg == "--logfile" && i+1 < len(args) {
					require.NoError(t, os.WriteFile(args[i+1], []byte("https://integration.trycloudflare.com"), 0600))
				}
			}
			return proc, nil
		}
		provider := NewCloudflareProvider(runner)
		provider.findProcess = func(int) (Process, error) { return proc, nil }
		runProviderServiceIntegrationContract(t, provider)
	})
}

func runProviderServiceIntegrationContract(t *testing.T, provider Provider) {
	t.Helper()
	store, z := &startStore{}, &startZellij{}
	service := NewService(store, z, provider)
	invitations, err := service.Start(context.Background(), StartRequest{Session: "selected", Provider: provider.Name()})
	require.NoError(t, err)
	require.NotEmpty(t, invitations.InteractiveBrowser)
	require.NotEmpty(t, invitations.ObserverBrowser)
	status, err := service.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, StateActive, status.State)
	require.True(t, status.InteractiveAvailable)
	require.True(t, status.ObserverAvailable)
	status, err = service.Stop(context.Background())
	require.NoError(t, err)
	require.Equal(t, StateInactive, status.State)
	status, err = service.Stop(context.Background())
	require.NoError(t, err)
	require.Equal(t, StateInactive, status.State)
}

func runHappyProviderContract(t *testing.T, p Provider) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, p.Validate(ctx))
	h, err := p.Start(ctx, "http://127.0.0.1:8082")
	require.NoError(t, err)
	st, err := p.Ready(ctx, h)
	require.NoError(t, err)
	require.Equal(t, "ready", st.State)
	require.Contains(t, st.EndpointURL, "https://")
	require.NoError(t, p.Stop(ctx, h))
	require.NoError(t, p.Stop(ctx, h))
}

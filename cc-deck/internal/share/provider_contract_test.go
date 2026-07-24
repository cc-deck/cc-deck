package share

import (
	"context"
	"github.com/stretchr/testify/require"
	"os"
	"sync"
	"testing"
)

type contractProvider struct{ stopped bool }

func (*contractProvider) Name() string                   { return "fake" }
func (*contractProvider) Validate(context.Context) error { return nil }
func (*contractProvider) Start(context.Context, string) (ProviderHandle, error) {
	return ProviderHandle{PID: 1}, nil
}
func (*contractProvider) Ready(context.Context, ProviderHandle) (ProviderStatus, error) {
	return ProviderStatus{State: "ready", EndpointURL: "https://example.test"}, nil
}
func (*contractProvider) Status(context.Context, ProviderHandle) (ProviderStatus, error) {
	return ProviderStatus{State: "ready"}, nil
}
func (p *contractProvider) Stop(context.Context, ProviderHandle) error { p.stopped = true; return nil }
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

func TestSharedProviderHappyPathContract(t *testing.T) {
	t.Run("isolated fake", func(t *testing.T) { runHappyProviderContract(t, &contractProvider{}) })
	t.Run("cloudflare runner fake", func(t *testing.T) {
		wait := make(chan error, 1)
		var once sync.Once
		proc := &fakeProcess{pid: 88, waitCh: wait}
		proc.onSignal = func() { once.Do(func() { wait <- nil }) }
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

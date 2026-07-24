package share

import (
	"context"
	"github.com/stretchr/testify/require"
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

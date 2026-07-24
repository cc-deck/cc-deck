package share

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func TestCloudflareValidateReportsMissingBinary(t *testing.T) {
	r := &fakeRunner{outputs: map[string][]byte{}, errors: map[string]error{key("cloudflared", []string{"--version"}): errors.New("not found")}}
	require.ErrorContains(t, NewCloudflareProvider(r).Validate(context.Background()), "required")
}

func TestCloudflareStartFailureIsSafe(t *testing.T) {
	r := &fakeRunner{startFn: func(string, []string) (Process, error) { return nil, errors.New("boom") }}
	_, err := NewCloudflareProvider(r).Start(context.Background(), "http://127.0.0.1:8082")
	require.ErrorContains(t, err, "start Cloudflare")
}

func TestCloudflareValidateAndStartEncryptedTunnel(t *testing.T) {
	r := &fakeRunner{outputs: map[string][]byte{key("cloudflared", []string{"--version"}): []byte("cloudflared 1")}, errors: map[string]error{}, process: &fakeProcess{pid: 42}}
	p := NewCloudflareProvider(r)
	require.NoError(t, p.Validate(context.Background()))
	h, e := p.Start(context.Background(), "http://127.0.0.1:8082")
	require.NoError(t, e)
	require.Equal(t, 42, h.PID)
	require.Contains(t, r.calls[1].args, "tunnel")
	require.Contains(t, r.calls[1].args, "--url")
}
func TestCloudflareReadyParsesHTTPSOnly(t *testing.T) {
	f, e := os.CreateTemp(t.TempDir(), "log")
	require.NoError(t, e)
	require.NoError(t, os.WriteFile(f.Name(), []byte("Visit https://three-words.trycloudflare.com now"), 0600))
	p := NewCloudflareProvider(&fakeRunner{})
	p.readyTimeout = time.Second
	st, e := p.Ready(context.Background(), ProviderHandle{Metadata: map[string]string{"log_path": f.Name()}})
	require.NoError(t, e)
	require.Equal(t, "ready", st.State)
	require.Equal(t, "https://three-words.trycloudflare.com", st.EndpointURL)
}
func TestCloudflareReadyRejectsMalformedAndTimesOut(t *testing.T) {
	f, e := os.CreateTemp(t.TempDir(), "log")
	require.NoError(t, e)
	require.NoError(t, os.WriteFile(f.Name(), []byte("http://unsafe.trycloudflare.com"), 0600))
	p := NewCloudflareProvider(&fakeRunner{})
	p.readyTimeout = 35 * time.Millisecond
	p.pollInterval = 5 * time.Millisecond
	_, e = p.Ready(context.Background(), ProviderHandle{Metadata: map[string]string{"log_path": f.Name()}})
	require.ErrorContains(t, e, "timeout")
}

func TestCloudflareStopWithoutProcessIsIdempotent(t *testing.T) {
	p := NewCloudflareProvider(&fakeRunner{})
	require.NoError(t, p.Stop(context.Background(), ProviderHandle{}))
	require.NoError(t, p.Stop(context.Background(), ProviderHandle{}))
}

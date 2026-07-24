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

func TestCloudflareReadyDetectsUnexpectedExit(t *testing.T) {
	wait := make(chan error, 1)
	p := NewCloudflareProvider(&fakeRunner{process: &fakeProcess{pid: 51, waitCh: wait}})
	h, err := p.Start(context.Background(), "http://127.0.0.1:8082")
	require.NoError(t, err)
	wait <- errors.New("exited 1")
	_, err = p.Ready(context.Background(), h)
	require.ErrorContains(t, err, "before readiness")
}

func TestCloudflareStopWaitsThenEscalates(t *testing.T) {
	wait := make(chan error, 1)
	proc := &fakeProcess{pid: 52, waitCh: wait}
	proc.onKill = func() { wait <- errors.New("killed") }
	p := NewCloudflareProvider(&fakeRunner{process: proc})
	p.stopTimeout = 10 * time.Millisecond
	h, err := p.Start(context.Background(), "http://127.0.0.1:8082")
	require.NoError(t, err)
	require.NoError(t, p.Stop(context.Background(), h))
}

func TestCloudflareRefusesMismatchedProcessIdentity(t *testing.T) {
	p := NewCloudflareProvider(&fakeRunner{outputs: map[string][]byte{}, errors: map[string]error{}})
	err := p.Stop(context.Background(), ProviderHandle{PID: 123, Metadata: map[string]string{"log_path": "expected", "identity": "other"}})
	require.ErrorContains(t, err, "identity")
}

func TestCloudflareStopIsIdempotentWhenPersistedProcessAlreadyGone(t *testing.T) {
	h := ProviderHandle{PID: 321, Metadata: map[string]string{"log_path": "marker", "identity": "marker", "process_fingerprint": "fingerprint"}}
	r := &fakeRunner{outputs: map[string][]byte{}, errors: map[string]error{key("ps", []string{"-p", "321", "-o", "lstart=,comm=,command="}): errors.New("no such process")}}
	require.NoError(t, NewCloudflareProvider(r).Stop(context.Background(), h))
}

func TestCloudflareStopReconstructsPersistedVerifiedProcess(t *testing.T) {
	wait := make(chan error, 1)
	stopped := false
	proc := &fakeProcess{pid: 322, waitCh: wait}
	proc.onSignal = func() { stopped = true; wait <- nil }
	fingerprint := "Fri Jul 24 cloudflared tunnel --logfile marker"
	r := &fakeRunner{runFn: func(name string, args []string) ([]byte, error) {
		if name == "ps" && stopped {
			return nil, errors.New("no such process")
		}
		return []byte(fingerprint), nil
	}}
	p := NewCloudflareProvider(r)
	p.findProcess = func(int) (Process, error) { return proc, nil }
	h := ProviderHandle{PID: 322, Metadata: map[string]string{"log_path": "marker", "identity": "marker", "process_fingerprint": fingerprint}}
	require.NoError(t, p.Stop(context.Background(), h))
}

func TestCloudflareStopFailureIsReported(t *testing.T) {
	wait := make(chan error)
	proc := &fakeProcess{pid: 53, waitCh: wait, signalErr: errors.New("denied")}
	p := NewCloudflareProvider(&fakeRunner{process: proc})
	h, err := p.Start(context.Background(), "http://127.0.0.1:8082")
	require.NoError(t, err)
	require.ErrorContains(t, p.Stop(context.Background(), h), "denied")
}

package share

import (
	"context"
	"errors"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/stretchr/testify/require"
)

// The behavioural contract every Endpoint implementation must satisfy, stated
// in contracts/endpoint-contract.md. Each test names the contract it covers, so
// a future implementation has a suite to run rather than a description to read.

// C-1: No lifecycle. An implementation must not start, stop, signal, restart,
// or supervise any process, and must not hold a process handle or PID.
func TestContractC1EndpointHasNoLifecycleSurface(t *testing.T) {
	endpointType := reflect.TypeOf((*Endpoint)(nil)).Elem()
	var methods []string
	for i := 0; i < endpointType.NumMethod(); i++ {
		methods = append(methods, endpointType.Method(i).Name)
	}
	require.ElementsMatch(t, []string{"Name", "Probe", "Resolve"}, methods,
		"Endpoint must expose exactly Name, Resolve, and Probe; anything else is a lifecycle handle in disguise")

	// The production implementation must not carry process identity either.
	staticType := reflect.TypeOf(StaticEndpoint{})
	for i := 0; i < staticType.NumField(); i++ {
		name := strings.ToLower(staticType.Field(i).Name)
		require.NotContains(t, name, "pid", "StaticEndpoint must hold no process identity")
		require.NotContains(t, name, "process", "StaticEndpoint must hold no process handle")
	}
}

// C-2: Resolve is pure with respect to the network, so a caller that only needs
// an address never pays for a probe.
func TestContractC2ResolvePerformsNoNetworkAccess(t *testing.T) {
	endpoint := NewStaticEndpoint(config.SharingConfig{Endpoint: "https://dev.example.com"}, "", "", nil)
	endpoint.systemLookup = func(context.Context, string) ([]net.IPAddr, error) {
		t.Fatal("Resolve performed a DNS lookup")
		return nil, nil
	}
	endpoint.publicLookup = endpoint.systemLookup

	// A nil Zellij proves the point twice over: Resolve cannot mint anything.
	ref, err := endpoint.Resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, "https://dev.example.com", ref.BaseURL)
}

// C-3: five stages in order, stopping at the first failure, with the result
// invariants that make the outcome unambiguous.
func TestContractC3ProbeResultInvariantsHold(t *testing.T) {
	t.Run("success carries no stage", func(t *testing.T) {
		fake := newEndpointFake(t, endpointBehaviour{})
		result, err := probeEndpoint(fake.server.URL, &probeZellij{}).Probe(context.Background(), fake.ref(), "demo")
		require.NoError(t, err)
		require.True(t, result.OK)
		require.Empty(t, result.FailedAt)
		require.False(t, result.CheckedAt.IsZero())
	})
	t.Run("failure names exactly one stage", func(t *testing.T) {
		fake := newEndpointFake(t, endpointBehaviour{refuseUpgrade: true})
		result, err := probeEndpoint(fake.server.URL, &probeZellij{}).Probe(context.Background(), fake.ref(), "demo")
		require.Error(t, err)
		require.False(t, result.OK)
		require.Equal(t, StageWebSocket, result.FailedAt)
		require.False(t, result.CheckedAt.IsZero())
	})
	t.Run("a later stage is never reached after an earlier one fails", func(t *testing.T) {
		// The HTTP stage fails, so no credential is ever minted for auth.
		fake := newEndpointFake(t, endpointBehaviour{rootStatus: 530})
		zellij := &probeZellij{}
		result, err := probeEndpoint(fake.server.URL, zellij).Probe(context.Background(), fake.ref(), "demo")
		require.Error(t, err)
		require.Equal(t, StageHTTP, result.FailedAt)
		minted, _, _ := zellij.snapshot()
		require.Empty(t, minted, "the auth stage must not run after the http stage failed")
	})
}

// C-4: one deadline for the whole probe, honoured even when the endpoint
// accepts connections and never answers.
func TestContractC4ProbeReturnsWithinTheCallersDeadline(t *testing.T) {
	fake := newEndpointFake(t, endpointBehaviour{silent: true})
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	started := time.Now()
	_, err := probeEndpoint(fake.server.URL, &probeZellij{}).Probe(ctx, fake.ref(), "demo")
	require.Error(t, err)
	require.Less(t, time.Since(started), 3*time.Second)
}

// C-5: a filtered name and a missing name are different problems and must be
// reported differently, and the second opinion never breaks a passing lookup.
func TestContractC5ProbeDistinguishesFilteredFromMissing(t *testing.T) {
	missing := probeEndpoint("https://gone.example", &probeZellij{})
	missing.systemLookup = func(context.Context, string) ([]net.IPAddr, error) { return nil, errNoSuchHost }
	missing.publicLookup = missing.systemLookup
	result, err := missing.Probe(context.Background(), EndpointRef{BaseURL: "https://gone.example"}, "demo")
	require.Error(t, err)
	require.Equal(t, StageDNS, result.FailedAt)
	require.Contains(t, result.Diagnostic, "does not resolve")
	require.NotContains(t, result.Diagnostic, "filtered")

	filtered := probeEndpoint("https://blocked.example", &probeZellij{})
	filtered.systemLookup = func(context.Context, string) ([]net.IPAddr, error) { return nil, errNoSuchHost }
	filtered.publicLookup = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.IPv4(203, 0, 113, 7)}}, nil
	}
	result, err = filtered.Probe(context.Background(), EndpointRef{BaseURL: "https://blocked.example"}, "demo")
	require.Error(t, err)
	require.Equal(t, StageDNS, result.FailedAt)
	require.Contains(t, result.Diagnostic, "filtered")
}

// C-6: the probe mints its own least-privileged credential and revokes it on
// every exit path, and never touches a credential issued to a person.
func TestContractC6ProbeOwnsItsCredentialAndTouchesNoOthers(t *testing.T) {
	fake := newEndpointFake(t, endpointBehaviour{refuseUpgrade: true})
	zellij := &probeZellij{}
	_, err := probeEndpoint(fake.server.URL, zellij).Probe(context.Background(), fake.ref(), "demo")
	require.Error(t, err)

	minted, revoked, readOnly := zellij.snapshot()
	require.Len(t, minted, 1)
	require.Equal(t, []bool{true}, readOnly, "observer role is the least privilege that completes a login")
	require.Equal(t, minted, revoked, "the probe revokes exactly what it minted, by the name Zellij returned")
}

// C-7: the control channel only. The probe must never open a terminal socket,
// attach, or write to the session it is verifying.
func TestContractC7ProbeNeverTouchesATerminalSocket(t *testing.T) {
	fake := newEndpointFake(t, endpointBehaviour{})
	_, err := probeEndpoint(fake.server.URL, &probeZellij{}).Probe(context.Background(), fake.ref(), "demo")
	require.NoError(t, err)
	require.False(t, fake.sawTerminalSocket())
}

// C-8: the probe never mutates sharing state, whatever its outcome. Recording
// the result is the caller's job and is an observation only.
func TestContractC8ProbeMutatesNoSharingState(t *testing.T) {
	for _, behaviour := range []endpointBehaviour{
		{},
		{rootStatus: 530},
		{loginStatus: 400},
		{refuseUpgrade: true},
	} {
		fake := newEndpointFake(t, behaviour)
		zellij := &probeZellij{}
		_, _ = probeEndpoint(fake.server.URL, zellij).Probe(context.Background(), fake.ref(), "demo")

		_, revoked, _ := zellij.snapshot()
		require.False(t, zellij.stoppedWeb, "a probe must never stop the web server")
		for _, name := range revoked {
			require.Contains(t, name, "zellij-chosen-", "a probe must revoke only the credential it minted")
		}
	}
}

// C-9: diagnostics carry no token, no cookie value, and no authorization header.
func TestContractC9DiagnosticsCarryNoSecrets(t *testing.T) {
	fake := newEndpointFake(t, endpointBehaviour{loginStatus: 400, echoCredential: true})
	zellij := &probeZellij{secret: probeSentinelSecret}
	result, err := probeEndpoint(fake.server.URL, zellij).Probe(context.Background(), fake.ref(), "demo")
	require.Error(t, err)
	require.NotContains(t, result.Diagnostic, probeSentinelSecret)
	require.NotContains(t, err.Error(), probeSentinelSecret)
}

// errNoSuchHost stands in for what a resolver reports for a name it cannot find.
var errNoSuchHost = errors.New("no such host")

func TestStaticEndpointSatisfiesTheEndpointInterface(t *testing.T) {
	var endpoint Endpoint = NewStaticEndpoint(config.SharingConfig{Endpoint: "https://dev.example.com"}, "", "", &startZellij{})
	require.NotEmpty(t, endpoint.Name())
	ref, err := endpoint.Resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, "https://dev.example.com", ref.BaseURL)
}

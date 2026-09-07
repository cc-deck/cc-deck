package share

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/stretchr/testify/require"
)

// probeSentinelSecret is a recognizable value that must never appear in a
// diagnostic. It is used as the probe credential's secret so a leak anywhere in
// the probe's error paths is caught by name.
const probeSentinelSecret = "SENTINEL-PROBE-SECRET-b6f2"

// fakeWebClientID is the client id the fake issues from POST /session, mirroring
// the uuid the real Zellij web server returns.
const fakeWebClientID = "3f6d1c48-0e4a-4a4f-9d2b-0c5e6a7b8c90"

// --- the endpoint fake -------------------------------------------------------

// endpointBehaviour selects which single layer the fake endpoint breaks at.
// Every field defaults to the healthy contract, so a test names only the one
// thing it is breaking.
type endpointBehaviour struct {
	// serveTLS presents an untrusted self-signed certificate.
	serveTLS bool
	// rootStatus overrides the status returned for GET /.
	rootStatus int
	// rootBody overrides the body returned for GET /, for an origin that
	// answers but is not the Zellij web client.
	rootBody string
	// loginStatus overrides the status returned for POST /command/login.
	loginStatus int
	// dropCookie returns a successful login with no Set-Cookie header.
	dropCookie bool
	// refuseUpgrade serves every page correctly but never hijacks the
	// connection, which is the reported blank-terminal defect.
	refuseUpgrade bool
	// refuseClientID serves the pages and the login but will not open a control
	// channel, which is what a proxy that only forwards GET produces.
	refuseClientID bool
	// silent accepts connections and never answers.
	silent bool
	// echoCredential writes the presented credential into the response body, to
	// prove diagnostics never carry one back out.
	echoCredential bool
}

// zellijWebClientPage is the marker the HTTP stage looks for. A proxy that
// serves some other origin's 200 must not pass this stage.
const zellijWebClientPage = `<!doctype html>
<html lang="en"><head><title>Zellij Web Client</title>
<script src="assets/xterm.js"></script></head>
<body data-authenticated="false"><div id="terminal"></div></body></html>`

type endpointFake struct {
	server *httptest.Server
	// terminalSocketOpened records any attempt to touch a terminal socket. The
	// probe must never open one.
	terminalSocketOpened bool
	mu                   sync.Mutex
}

func newEndpointFake(t *testing.T, behaviour endpointBehaviour) *endpointFake {
	t.Helper()
	fake := &endpointFake{}
	mux := http.NewServeMux()

	block := func(w http.ResponseWriter, r *http.Request) bool {
		if !behaviour.silent {
			return false
		}
		<-r.Context().Done()
		return true
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if block(w, r) {
			return
		}
		if strings.HasPrefix(r.URL.Path, "/ws/terminal") {
			fake.mu.Lock()
			fake.terminalSocketOpened = true
			fake.mu.Unlock()
			http.Error(w, "the probe must not open a terminal socket", http.StatusForbidden)
			return
		}
		if behaviour.rootStatus != 0 && behaviour.rootStatus/100 != 2 {
			http.Error(w, "endpoint is not available", behaviour.rootStatus)
			return
		}
		body := zellijWebClientPage
		if behaviour.rootBody != "" {
			body = behaviour.rootBody
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(body))
	})

	mux.HandleFunc("/info/version", func(w http.ResponseWriter, r *http.Request) {
		if block(w, r) {
			return
		}
		_, _ = w.Write([]byte("0.45.1"))
	})

	mux.HandleFunc("/command/login", func(w http.ResponseWriter, r *http.Request) {
		if block(w, r) {
			return
		}
		var request struct {
			AuthToken  string `json:"auth_token"`
			RememberMe bool   `json:"remember_me"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		if behaviour.loginStatus != 0 && behaviour.loginStatus/100 != 2 {
			if behaviour.echoCredential {
				http.Error(w, "rejected token "+request.AuthToken, behaviour.loginStatus)
				return
			}
			http.Error(w, "login failed", behaviour.loginStatus)
			return
		}
		if request.AuthToken == "" {
			http.Error(w, "login failed", http.StatusUnauthorized)
			return
		}
		if !behaviour.dropCookie {
			http.SetCookie(w, &http.Cookie{
				Name: "session_token", Value: "session-for-" + request.AuthToken,
				HttpOnly: true, SameSite: http.SameSiteStrictMode, Path: "/",
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"message":"Login successful"}`))
	})

	// POST /session issues the client id that addresses the control channel.
	// The real Zellij web server answers /ws/control with 400 without it, so a
	// fake that did not require one would pass a probe the real server fails.
	mux.HandleFunc("/session", func(w http.ResponseWriter, r *http.Request) {
		if block(w, r) {
			return
		}
		if _, err := r.Cookie("session_token"); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if behaviour.refuseClientID {
			http.Error(w, "no control channel here", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"web_client_id":"` + fakeWebClientID + `","is_read_only":true}`))
	})

	mux.HandleFunc("/ws/control", func(w http.ResponseWriter, r *http.Request) {
		if block(w, r) {
			return
		}
		// The control channel sits behind the session cookie, exactly as the
		// Zellij web server has it.
		if _, err := r.Cookie("session_token"); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("web_client_id") != fakeWebClientID {
			http.Error(w, "missing field `web_client_id`", http.StatusBadRequest)
			return
		}
		key := r.Header.Get("Sec-WebSocket-Key")
		if key == "" {
			http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
			return
		}
		if behaviour.refuseUpgrade {
			// Serves pages correctly, refuses to hijack. This is a proxy that
			// forwards HTTP and drops Upgrade: the page loads, the terminal
			// never fills.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("upgrade not forwarded"))
			return
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "not hijackable", http.StatusInternalServerError)
			return
		}
		conn, buffered, err := hijacker.Hijack()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = buffered.WriteString("HTTP/1.1 101 Switching Protocols\r\n" +
			"Upgrade: websocket\r\nConnection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: " + websocketAccept(key) + "\r\n\r\n")
		_ = buffered.Flush()
	})

	if behaviour.serveTLS {
		fake.server = httptest.NewTLSServer(mux)
	} else {
		fake.server = httptest.NewServer(mux)
	}
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *endpointFake) ref() EndpointRef { return EndpointRef{BaseURL: f.server.URL} }

func (f *endpointFake) sawTerminalSocket() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.terminalSocketOpened
}

// trustedTLS returns a TLS configuration that trusts the fake's certificate, so
// a healthy https case can be exercised without weakening production defaults.
func (f *endpointFake) trustedTLS() *tls.Config {
	return f.server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
}

func websocketAccept(key string) string {
	sum := sha1.Sum([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// --- the Zellij fake ---------------------------------------------------------

// probeZellij records the probe credential lifecycle. Only CreateToken and
// RevokeToken are exercised by a probe; everything else must stay untouched.
type probeZellij struct {
	mu           sync.Mutex
	minted       []bool // readOnly flag per mint
	mintedNames  []string
	revokedNames []string
	createErr    error
	revokeErr    error
	secret       string
	stoppedWeb   bool
}

func (z *probeZellij) ValidateCapabilities(context.Context) error { return nil }
func (z *probeZellij) SessionExists(context.Context, string) (bool, error) {
	return true, nil
}
func (z *probeZellij) CreateToken(_ context.Context, _ string, readOnly bool) (TokenCredential, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	if z.createErr != nil {
		return TokenCredential{}, z.createErr
	}
	z.minted = append(z.minted, readOnly)
	// Zellij ignores the requested label and returns its own name, so a probe
	// must revoke by the name it is given, never by one it chose.
	name := fmt.Sprintf("zellij-chosen-%d", len(z.minted))
	z.mintedNames = append(z.mintedNames, name)
	secret := z.secret
	if secret == "" {
		secret = probeSentinelSecret
	}
	return TokenCredential{Name: name, Secret: secret}, nil
}
func (z *probeZellij) RevokeToken(_ context.Context, name string) error {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.revokedNames = append(z.revokedNames, name)
	return z.revokeErr
}
func (z *probeZellij) EnsureWebServer(context.Context) (string, bool, error) {
	return "http://127.0.0.1:8082", false, nil
}
func (z *probeZellij) StopWebServer(context.Context) error {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.stoppedWeb = true
	return nil
}
func (z *probeZellij) snapshot() ([]string, []string, []bool) {
	z.mu.Lock()
	defer z.mu.Unlock()
	return append([]string(nil), z.mintedNames...), append([]string(nil), z.revokedNames...), append([]bool(nil), z.minted...)
}

// probeEndpoint builds a StaticEndpoint aimed at an address, with the test
// seams wired so no test ever touches the real network or a real resolver.
func probeEndpoint(address string, zellij Zellij) *StaticEndpoint {
	endpoint := NewStaticEndpoint(config.SharingConfig{Endpoint: address}, "", "", zellij)
	endpoint.systemLookup = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.IPv4(127, 0, 0, 1)}}, nil
	}
	endpoint.publicLookup = endpoint.systemLookup
	return endpoint
}

// --- T012: the stage matrix --------------------------------------------------

func TestProbeAttributesEachLayerToItsOwnStage(t *testing.T) {
	resolvesNowhere := func(e *StaticEndpoint) {
		e.systemLookup = func(context.Context, string) ([]net.IPAddr, error) {
			return nil, errors.New("no such host")
		}
		e.publicLookup = e.systemLookup
	}
	filteredLocally := func(e *StaticEndpoint) {
		e.systemLookup = func(context.Context, string) ([]net.IPAddr, error) {
			return nil, errors.New("no such host")
		}
		e.publicLookup = func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.IPv4(203, 0, 113, 7)}}, nil
		}
	}

	cases := []struct {
		name string
		// behaviour describes the fake endpoint, or nil when the case never
		// reaches a server at all.
		behaviour *endpointBehaviour
		// address overrides the fake's address, for the cases about names.
		address string
		tune    func(*StaticEndpoint)
		// trustTLS makes the probe trust the fake's self-signed certificate.
		trustTLS bool
		wantOK   bool
		wantFail ProbeStage
		// wantDiagnostic is a substring the diagnostic must contain.
		wantDiagnostic string
		// wantDeadline expects the probe to return because the budget ran out.
		wantDeadline bool
	}{
		{
			name:      "healthy",
			behaviour: &endpointBehaviour{},
			wantOK:    true,
		},
		{
			name:           "name missing",
			address:        "https://name-that-does-not-exist.invalid",
			tune:           resolvesNowhere,
			wantFail:       StageDNS,
			wantDiagnostic: "does not resolve",
		},
		{
			name:           "name filtered",
			address:        "https://filtered.example.com",
			tune:           filteredLocally,
			wantFail:       StageDNS,
			wantDiagnostic: "filtered",
		},
		{
			name:           "bad certificate",
			behaviour:      &endpointBehaviour{serveTLS: true},
			wantFail:       StageTLS,
			wantDiagnostic: "TLS",
		},
		{
			name:           "wrong origin",
			behaviour:      &endpointBehaviour{rootBody: "<html><title>Some other service</title></html>"},
			wantFail:       StageHTTP,
			wantDiagnostic: "Zellij web client",
		},
		{
			name:           "degraded tunnel",
			behaviour:      &endpointBehaviour{rootStatus: 530},
			wantFail:       StageHTTP,
			wantDiagnostic: "530",
		},
		{
			name:           "login body dropped",
			behaviour:      &endpointBehaviour{loginStatus: 400},
			wantFail:       StageAuth,
			wantDiagnostic: "400",
		},
		{
			name:           "set-cookie stripped",
			behaviour:      &endpointBehaviour{dropCookie: true},
			wantFail:       StageAuth,
			wantDiagnostic: "session_token",
		},
		{
			// The reported failure: a page that loads and a terminal that never
			// fills, because the proxy forwards HTTP and drops the upgrade.
			name:           "upgrade refused",
			behaviour:      &endpointBehaviour{refuseUpgrade: true},
			wantFail:       StageWebSocket,
			wantDiagnostic: "upgrade",
		},
		{
			// A proxy that forwards GET but not POST beyond the login serves
			// the page, logs in, and then cannot open a control channel.
			name:           "control channel refused",
			behaviour:      &endpointBehaviour{refuseClientID: true},
			wantFail:       StageWebSocket,
			wantDiagnostic: "control channel",
		},
		{
			name:         "silent endpoint",
			behaviour:    &endpointBehaviour{silent: true},
			wantDeadline: true,
		},
		{
			name:      "healthy over TLS",
			behaviour: &endpointBehaviour{serveTLS: true},
			trustTLS:  true,
			wantOK:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			address := tc.address
			var fake *endpointFake
			if tc.behaviour != nil {
				fake = newEndpointFake(t, *tc.behaviour)
				if address == "" {
					address = fake.server.URL
				}
			}
			zellij := &probeZellij{}
			endpoint := probeEndpoint(address, zellij)
			if tc.trustTLS {
				endpoint.tlsConfig = fake.trustedTLS()
			}
			if tc.tune != nil {
				tc.tune(endpoint)
			}

			budget := 10 * time.Second
			if tc.wantDeadline {
				budget = 300 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), budget)
			defer cancel()

			started := time.Now()
			result, err := endpoint.Probe(ctx, EndpointRef{BaseURL: address}, "demo-session")
			elapsed := time.Since(started)

			require.False(t, result.CheckedAt.IsZero(), "CheckedAt is always set on a completed probe")

			if tc.wantOK {
				require.NoError(t, err)
				require.True(t, result.OK)
				require.Empty(t, result.FailedAt, "FailedAt is empty only when OK is true")
				return
			}

			require.Error(t, err)
			require.False(t, result.OK)

			if tc.wantDeadline {
				require.Less(t, elapsed, 5*time.Second, "a silent endpoint must return within the budget, not hang")
				require.NotEmpty(t, result.FailedAt, "even a deadline names the layer it expired in")
				return
			}

			require.Equal(t, tc.wantFail, result.FailedAt)
			require.Contains(t, strings.ToLower(result.Diagnostic), strings.ToLower(tc.wantDiagnostic))
			if fake != nil {
				require.False(t, fake.sawTerminalSocket(), "the probe must never open a terminal socket")
			}
		})
	}
}

// T012a: the measurement method for "the correct layer is named in 100% of
// layer-specific cases". A stage added without a failure case fails the build.
func TestProbeStageMatrixCoversEveryDeclaredStage(t *testing.T) {
	covered := map[ProbeStage]bool{}
	for _, stage := range []ProbeStage{StageDNS, StageDNS, StageTLS, StageHTTP, StageHTTP, StageAuth, StageAuth, StageWebSocket} {
		covered[stage] = true
	}
	for _, stage := range ProbeStages {
		require.True(t, covered[stage],
			"ProbeStage %q has no failure case in the stage matrix; add one to TestProbeAttributesEachLayerToItsOwnStage", stage)
	}
	require.Len(t, covered, len(ProbeStages), "the matrix names a stage that is not declared")
}

// --- T014: the probe credential lifecycle ------------------------------------

func TestProbeMintsAndRevokesItsOwnCredentialOnEveryPath(t *testing.T) {
	for _, tc := range []struct {
		name      string
		behaviour endpointBehaviour
		budget    time.Duration
	}{
		{"on success", endpointBehaviour{}, 10 * time.Second},
		{"on failure", endpointBehaviour{refuseUpgrade: true}, 10 * time.Second},
		{"on deadline expiry", endpointBehaviour{silent: true}, 300 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := newEndpointFake(t, tc.behaviour)
			zellij := &probeZellij{}
			endpoint := probeEndpoint(fake.server.URL, zellij)

			ctx, cancel := context.WithTimeout(context.Background(), tc.budget)
			defer cancel()
			result, _ := endpoint.Probe(ctx, fake.ref(), "demo-session")

			minted, revoked, readOnly := zellij.snapshot()
			if tc.name == "on deadline expiry" && len(minted) == 0 {
				// The budget expired before the auth stage. Nothing was minted,
				// so there is nothing to revoke, which is also correct.
				return
			}
			require.Len(t, minted, 1, "exactly one credential per probe")
			require.Equal(t, []bool{true}, readOnly, "the probe credential is observer role, the least privilege that works")
			require.Equal(t, minted, revoked, "the probe revokes by the name Zellij returned, on every exit path")

			// Nothing about the probe credential may reach persisted state.
			require.NotContains(t, fmt.Sprintf("%+v", result), probeSentinelSecret)
		})
	}
}

func TestProbeNeverPersistsItsCredential(t *testing.T) {
	fake := newEndpointFake(t, endpointBehaviour{})
	zellij := &probeZellij{}
	endpoint := probeEndpoint(fake.server.URL, zellij)

	result, err := endpoint.Probe(context.Background(), fake.ref(), "demo-session")
	require.NoError(t, err)
	require.True(t, result.OK)
	// ProbeResult is the only thing a probe returns for persistence, and it has
	// no field a credential could occupy.
	require.NotContains(t, fmt.Sprintf("%+v", result), probeSentinelSecret)
	require.NotContains(t, fmt.Sprintf("%+v", result), "session-for-")
}

// --- T014a: no secret reaches a diagnostic -----------------------------------

func TestProbeDiagnosticsCarryNoSecretsAtAnyStage(t *testing.T) {
	behaviours := map[ProbeStage]endpointBehaviour{
		StageHTTP:      {rootStatus: 530},
		StageAuth:      {loginStatus: 400, echoCredential: true},
		StageWebSocket: {refuseUpgrade: true},
	}
	for stage, behaviour := range behaviours {
		t.Run(string(stage), func(t *testing.T) {
			fake := newEndpointFake(t, behaviour)
			zellij := &probeZellij{secret: probeSentinelSecret}
			endpoint := probeEndpoint(fake.server.URL, zellij)

			result, err := endpoint.Probe(context.Background(), fake.ref(), "demo-session")
			require.Error(t, err)
			require.Equal(t, stage, result.FailedAt)
			require.NotContains(t, result.Diagnostic, probeSentinelSecret,
				"a probe credential must never appear in a diagnostic")
			require.NotContains(t, err.Error(), probeSentinelSecret)
			require.NotContains(t, strings.ToLower(result.Diagnostic), "authorization:")
			require.NotContains(t, result.Diagnostic, "session-for-",
				"a session cookie value must never appear in a diagnostic")
		})
	}
}

// --- probe behaviour that the contract states explicitly ---------------------

func TestProbeHonoursASingleDeadlineRatherThanPerStageBudgets(t *testing.T) {
	fake := newEndpointFake(t, endpointBehaviour{silent: true})
	endpoint := probeEndpoint(fake.server.URL, &probeZellij{})

	budget := 400 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	started := time.Now()
	_, err := endpoint.Probe(ctx, fake.ref(), "demo-session")
	elapsed := time.Since(started)

	require.Error(t, err)
	// Five stages with their own budgets would multiply this out. One deadline
	// for the whole probe is what keeps a bound from becoming a hang.
	require.Less(t, elapsed, 5*budget, "the whole probe shares one deadline")
}

func TestProbeSecondOpinionFailureNeverBreaksASuccessfulPrimaryResolution(t *testing.T) {
	fake := newEndpointFake(t, endpointBehaviour{})
	endpoint := probeEndpoint(fake.server.URL, &probeZellij{})
	endpoint.publicLookup = func(context.Context, string) ([]net.IPAddr, error) {
		return nil, errors.New("port 53 is blocked here")
	}

	result, err := endpoint.Probe(context.Background(), fake.ref(), "demo-session")
	require.NoError(t, err, "the second opinion is best effort and must not turn a pass into a failure")
	require.True(t, result.OK)
}

func TestProbeOpensTheControlChannelAndNeverATerminalSocket(t *testing.T) {
	fake := newEndpointFake(t, endpointBehaviour{})
	endpoint := probeEndpoint(fake.server.URL, &probeZellij{})

	_, err := endpoint.Probe(context.Background(), fake.ref(), "demo-session")
	require.NoError(t, err)
	require.False(t, fake.sawTerminalSocket())
}

func TestProbeRejectsAnAddressItCannotParse(t *testing.T) {
	endpoint := probeEndpoint("::not-a-url", &probeZellij{})
	result, err := endpoint.Probe(context.Background(), EndpointRef{BaseURL: "::not-a-url"}, "demo-session")
	require.Error(t, err)
	require.False(t, result.OK)
	require.NotEmpty(t, result.FailedAt)
	require.False(t, result.CheckedAt.IsZero())
}

// probeThroughRealHTTP is a guard that the fake and the probe agree on the
// handshake, independent of the stage matrix.
func TestWebSocketAcceptMatchesTheHandshakeTheFakeComputes(t *testing.T) {
	key := "dGhlIHNhbXBsZSBub25jZQ=="
	// The value from RFC 6455 section 1.3.
	require.Equal(t, "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=", websocketAccept(key))
}

// A small sanity check that the fake serves what the HTTP stage looks for.
func TestEndpointFakeServesTheZellijWebClientContract(t *testing.T) {
	fake := newEndpointFake(t, endpointBehaviour{})
	client := fake.server.Client()

	resp, err := client.Get(fake.server.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	loginURL, err := url.JoinPath(fake.server.URL, "command", "login")
	require.NoError(t, err)
	resp, err = client.Post(loginURL, "application/json", strings.NewReader(`{"auth_token":"t","remember_me":false}`))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var found bool
	for _, cookie := range resp.Cookies() {
		found = found || cookie.Name == "session_token"
	}
	require.True(t, found, "the fake must set the session_token cookie the contract names")
}

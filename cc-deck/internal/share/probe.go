package share

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// websocketGUID is the constant from RFC 6455 that turns a client key into the
// value the server must echo back.
const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// publicResolverAddress is the second opinion consulted when the system
// resolver reports that a name does not exist. A local filter answering
// NXDOMAIN for a name that resolves fine elsewhere is indistinguishable from a
// typo without this, and that ambiguity once cost a full day of diagnosis.
const publicResolverAddress = "1.1.1.1:53"

// probeRevokeTimeout bounds the credential revocation that runs on every exit
// path. It uses its own context because the probe's own deadline may already
// have expired, and a probe that leaves a credential behind is worse than a
// probe that takes slightly longer to fail.
const probeRevokeTimeout = 5 * time.Second

// maxDiagnosticBody caps how much of a response body a diagnostic may quote.
const maxDiagnosticBody = 2048

// Probe verifies that the endpoint actually reaches a live terminal. It runs
// five layers in order, stops at the first failure, and names the layer that
// failed:
//
//	DNS, TLS, HTTP, Auth, WebSocket
//
// It is an observation and nothing more. It revokes no person's credential,
// stops no process, deletes no state, and triggers no teardown, whatever it
// finds. The entire probe honours the caller's deadline; there are no per
// stage budgets, because five of those multiply into a bound that no longer
// bounds anything.
func (e *StaticEndpoint) Probe(ctx context.Context, ref EndpointRef, _ string) (ProbeResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// redact accumulates the values that must never appear in a diagnostic. The
	// probe's own credential is added to it the moment it exists.
	var secrets []string
	fail := func(stage ProbeStage, format string, args ...any) (ProbeResult, error) {
		diagnostic := redactSecrets(fmt.Sprintf(format, args...), secrets)
		result := ProbeResult{OK: false, FailedAt: stage, Diagnostic: diagnostic, CheckedAt: time.Now().UTC()}
		return result, &ProbeFailedError{Result: result, Endpoint: ref.BaseURL}
	}

	base, err := url.Parse(ref.BaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return fail(StageDNS, "endpoint address %q is not an absolute http or https URL", ref.BaseURL)
	}

	host := base.Hostname()
	port := base.Port()
	if port == "" {
		port = "80"
		if base.Scheme == "https" {
			port = "443"
		}
	}
	address := net.JoinHostPort(host, port)

	// Stage 1: DNS.
	if err := e.probeDNS(ctx, host); err != nil {
		return fail(StageDNS, "%s", err.Error())
	}

	// Stage 2: TLS. Only an https endpoint has a handshake to fail.
	if base.Scheme == "https" {
		dialer := &tls.Dialer{Config: e.clientTLSConfig(host)}
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return fail(StageTLS, "the TLS handshake failed: %v", err)
		}
		_ = conn.Close()
	}

	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: e.clientTLSConfig(host)},
		// Redirects are not followed. A proxy that redirects the web client
		// somewhere else is not serving the client at this address, and
		// following the redirect would report on a different origin.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	// Stage 3: HTTP. Receiving a 200 is not enough; the endpoint must be
	// serving the Zellij web client and not some other origin that happens to
	// answer on this address.
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return fail(StageHTTP, "could not build a request for %s: %v", base.String(), err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fail(StageHTTP, "the endpoint did not answer: %v", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxDiagnosticBody))
	_ = response.Body.Close()
	if response.StatusCode/100 != 2 {
		return fail(StageHTTP, "the endpoint answered %d %s rather than serving the web client",
			response.StatusCode, http.StatusText(response.StatusCode))
	}
	if readErr != nil {
		return fail(StageHTTP, "the endpoint answered but its response could not be read: %v", readErr)
	}
	if !looksLikeZellijWebClient(string(body)) {
		return fail(StageHTTP, "the endpoint answered 200 but is not serving the Zellij web client; check that the proxy forwards to the Zellij web server and does not rewrite paths")
	}

	// Stage 4: Auth. The probe authenticates with a credential it mints for
	// this probe alone, in the least privileged role that works, and revokes it
	// on every exit path below including panic and deadline expiry.
	credential, err := e.zellij.CreateToken(ctx, "cc-deck-probe", true)
	if err != nil {
		return fail(StageAuth, "could not mint a probe credential: %v", err)
	}
	secrets = append(secrets, credential.Secret)
	defer func() {
		revokeCtx, cancel := context.WithTimeout(context.Background(), probeRevokeTimeout)
		defer cancel()
		// Revoke by the name Zellij returned. It ignores the label it is given,
		// so a name the probe chose would revoke nothing.
		_ = e.zellij.RevokeToken(revokeCtx, credential.Name)
	}()

	loginBody, err := json.Marshal(map[string]any{"auth_token": credential.Secret, "remember_me": false})
	if err != nil {
		return fail(StageAuth, "could not build the login request: %v", err)
	}
	loginURL := base.JoinPath("command", "login")
	loginRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL.String(), bytes.NewReader(loginBody))
	if err != nil {
		return fail(StageAuth, "could not build the login request: %v", err)
	}
	loginRequest.Header.Set("Content-Type", "application/json")
	loginResponse, err := client.Do(loginRequest)
	if err != nil {
		return fail(StageAuth, "the login request failed: %v", err)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(loginResponse.Body, maxDiagnosticBody))
	_ = loginResponse.Body.Close()
	if loginResponse.StatusCode/100 != 2 {
		return fail(StageAuth, "login returned %d %s; a proxy that drops POST bodies produces exactly this",
			loginResponse.StatusCode, http.StatusText(loginResponse.StatusCode))
	}
	sessionCookie := ""
	for _, cookie := range loginResponse.Cookies() {
		if cookie.Name == "session_token" {
			sessionCookie = cookie.Value
		}
	}
	if sessionCookie == "" {
		return fail(StageAuth, "login succeeded but no session_token cookie came back; a proxy that strips Set-Cookie produces exactly this")
	}
	secrets = append(secrets, sessionCookie)

	// Stage 5: WebSocket. The control channel only. A terminal socket is never
	// opened, so the probe cannot disturb the session it is verifying.
	//
	// The control channel is addressed by a client id the server issues, so the
	// probe asks for one exactly as the web client does. Skipping this and
	// opening /ws/control bare answers 400, which would report a healthy
	// endpoint as broken.
	clientID, err := e.requestClientID(ctx, client, base, sessionCookie)
	if err != nil {
		return fail(StageWebSocket, "%s", err.Error())
	}
	if err := e.probeWebSocket(ctx, base, address, sessionCookie, clientID); err != nil {
		return fail(StageWebSocket, "%s", err.Error())
	}

	return ProbeResult{OK: true, CheckedAt: time.Now().UTC()}, nil
}

// probeDNS resolves the endpoint name and, when the system resolver says the
// name does not exist, asks an independent public resolver for a second
// opinion. A name that resolves publicly but not locally is being filtered,
// which is a different problem from a name that does not exist, and the user
// is sent somewhere useful only if the two are distinguished.
//
// The second opinion is best effort. Its own failure never turns a successful
// primary resolution into an error.
func (e *StaticEndpoint) probeDNS(ctx context.Context, host string) error {
	if host == "" {
		return fmt.Errorf("the endpoint address has no host to resolve")
	}
	if net.ParseIP(host) != nil {
		return nil
	}
	if _, err := e.lookupSystem(ctx, host); err == nil {
		return nil
	} else {
		if _, publicErr := e.lookupPublic(ctx, host); publicErr == nil {
			return fmt.Errorf("%s resolves through an independent public resolver but not through this host's resolver, so the name is being filtered locally rather than missing", host)
		}
		return fmt.Errorf("%s does not resolve: %v", host, err)
	}
}

func (e *StaticEndpoint) lookupSystem(ctx context.Context, host string) ([]net.IPAddr, error) {
	if e.systemLookup != nil {
		return e.systemLookup(ctx, host)
	}
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

func (e *StaticEndpoint) lookupPublic(ctx context.Context, host string) ([]net.IPAddr, error) {
	if e.publicLookup != nil {
		return e.publicLookup(ctx, host)
	}
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(dialCtx context.Context, network, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(dialCtx, network, publicResolverAddress)
		},
	}
	return resolver.LookupIPAddr(ctx, host)
}

// clientTLSConfig returns the verification settings for this endpoint. Tests
// inject a pool that trusts a local fake; production leaves it nil, which means
// full verification against the system roots.
func (e *StaticEndpoint) clientTLSConfig(host string) *tls.Config {
	if e.tlsConfig == nil {
		return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	}
	config := e.tlsConfig.Clone()
	if config.ServerName == "" {
		config.ServerName = host
	}
	return config
}

// requestClientID asks the server for the client id that addresses the control
// channel, which is the same call the web client makes before it connects. The
// request is authenticated by the session cookie, so a proxy that mishandles
// cookies on anything other than the login response is caught here too.
func (e *StaticEndpoint) requestClientID(ctx context.Context, client *http.Client, base *url.URL, sessionCookie string) (string, error) {
	sessionURL := base.JoinPath("session")
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, sessionURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("could not build the control channel request: %v", err)
	}
	request.AddCookie(&http.Cookie{Name: "session_token", Value: sessionCookie})
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("the control channel could not be established: %v", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxDiagnosticBody))
	_ = response.Body.Close()
	if response.StatusCode/100 != 2 {
		return "", fmt.Errorf("the endpoint serves the web client but refused to open a control channel, answering %d %s",
			response.StatusCode, http.StatusText(response.StatusCode))
	}
	if readErr != nil {
		return "", fmt.Errorf("the control channel response could not be read: %v", readErr)
	}
	var payload struct {
		WebClientID string `json:"web_client_id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.WebClientID == "" {
		return "", fmt.Errorf("the endpoint answered the control channel request without a client id, so it is not serving the Zellij web server")
	}
	return payload.WebClientID, nil
}

// probeWebSocket performs the upgrade handshake by hand. It asserts exactly two
// facts: that the endpoint answers 101, and that it echoes the key correctly.
// No frame is ever sent or read, which is why this needs no WebSocket library.
func (e *StaticEndpoint) probeWebSocket(ctx context.Context, base *url.URL, address, sessionCookie, clientID string) error {
	key, err := websocketKey()
	if err != nil {
		return fmt.Errorf("could not build the upgrade request: %v", err)
	}

	conn, err := e.dialContext(ctx, base.Scheme, address, base.Hostname())
	if err != nil {
		return fmt.Errorf("the control channel could not be reached: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	// Build the request target by hand rather than through JoinPath, which
	// yields a path with no leading slash when the endpoint address carries no
	// path of its own, and that is a malformed request line.
	request := "GET " + controlChannelPath(base) + "?web_client_id=" + url.QueryEscape(clientID) + " HTTP/1.1\r\n" +
		"Host: " + base.Host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"Cookie: session_token=" + sessionCookie + "\r\n" +
		"\r\n"
	if _, err := io.WriteString(conn, request); err != nil {
		return fmt.Errorf("the upgrade request could not be sent: %v", err)
	}

	// A HEAD-shaped request object keeps ReadResponse from trying to read a
	// body that a 101 does not have.
	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodGet})
	if err != nil {
		return fmt.Errorf("the endpoint gave no answer to the upgrade request: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusSwitchingProtocols {
		return fmt.Errorf("the endpoint serves the web client but refused the WebSocket upgrade on /ws/control, answering %d %s instead of 101",
			response.StatusCode, http.StatusText(response.StatusCode))
	}
	if got := response.Header.Get("Sec-WebSocket-Accept"); got != websocketAcceptFor(key) {
		return fmt.Errorf("the endpoint answered 101 to the upgrade but did not echo the key correctly, so the connection is not a WebSocket")
	}
	return nil
}

// dialContext opens a raw connection, with TLS when the endpoint uses it, so
// the handshake above can be written and read directly.
func (e *StaticEndpoint) dialContext(ctx context.Context, scheme, address, host string) (net.Conn, error) {
	if scheme == "https" {
		dialer := &tls.Dialer{Config: e.clientTLSConfig(host)}
		return dialer.DialContext(ctx, "tcp", address)
	}
	var dialer net.Dialer
	return dialer.DialContext(ctx, "tcp", address)
}

// controlChannelPath renders the absolute request target for /ws/control,
// preserving any base path the endpoint is mounted under.
func controlChannelPath(base *url.URL) string {
	path := strings.TrimSuffix(base.EscapedPath(), "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimSuffix(path, "/") + "/ws/control"
}

func websocketKey() (string, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(nonce[:]), nil
}

func websocketAcceptFor(key string) string {
	sum := sha1.Sum([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// looksLikeZellijWebClient distinguishes the Zellij web client from any other
// origin that answers 200 on the same address. The markers come from the page
// the Zellij web server serves.
func looksLikeZellijWebClient(body string) bool {
	lowered := strings.ToLower(body)
	if strings.Contains(lowered, "zellij web client") {
		return true
	}
	return strings.Contains(lowered, "assets/xterm.js") && strings.Contains(lowered, `id="terminal"`)
}

// redactSecrets removes every credential the probe has handled from a
// diagnostic, so no token, cookie value, or authorization header can travel out
// of the probe in a message that is printed, logged, or persisted.
func redactSecrets(diagnostic string, secrets []string) string {
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		diagnostic = strings.ReplaceAll(diagnostic, secret, "[redacted]")
	}
	return diagnostic
}

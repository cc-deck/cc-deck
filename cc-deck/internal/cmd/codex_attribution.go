package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/cc-deck/cc-deck/internal/procinfo"
)

// Codex shared daemon attribution.
//
// Since Codex 0.157 an interactive `codex` client hands its work to a shared
// app-server daemon (one per Codex home), and hooks run inside that daemon.
// The daemon inherited the environment of whichever client started it, so
// the hook's own ZELLIJ_PANE_ID and ZELLIJ_SESSION_NAME name that first
// client's pane, not the pane of the client that owns the event. The code
// here finds the owning client instead: through a cached mapping that names
// the client by pid and start time, else by finding the single Codex process
// of the same user and Codex home whose working directory equals the
// payload's cwd. Whenever that is not certain, the event is dropped: a
// missing row is recoverable, a wrong row misleads.
//
// See specs/089-codex-daemon-pane-attribution/contracts/codex-attribution.md.

// Resolution reasons for Codex events (hook debug log values).
const (
	reasonFromArg            = "from-arg"
	reasonClientEnv          = "client-env"
	reasonFromClientCache    = "from-client-cache"
	reasonDiscovered         = "discovered"
	reasonAmbiguous          = "ambiguous"
	reasonNoClient           = "no-client"
	reasonClientNotInZellij  = "client-not-in-zellij"
	daemonAncestorDepthLimit = 8
)

// profileEnv is a cc-deck profile identity as exported by a profile wrapper.
type profileEnv struct {
	Name  string
	Color string
	Icon  string
}

// attribution is the outcome of resolving a Codex event's target pane.
type attribution struct {
	PaneID        uint32
	ZellijSession string
	Profile       profileEnv
	Reason        string
	// Detail refines Reason in the debug log, for example why no client
	// was found (inspection error, stale mapping); empty when Reason says
	// everything.
	Detail string
	// ClientPID and ClientStart identify the client for the session-to-pane
	// mapping; zero for reasons that do not name a client.
	ClientPID   int
	ClientStart int64
	// Forget asks the caller to remove the session's mapping: on
	// SessionStart the session left its remembered client.
	Forget bool

	// clientDir and clientHome are the directory and Codex home of the
	// client behind a cached mapping.
	clientDir, clientHome string
}

// delivers reports whether the event is sent to the sidebar.
func (a attribution) delivers() bool {
	switch a.Reason {
	case reasonClientEnv, reasonFromClientCache, reasonDiscovered:
		return true
	}
	return false
}

// pane returns the resolved pane for the debug log, empty when the event is
// dropped.
func (a attribution) pane() string {
	if !a.delivers() {
		return ""
	}
	return strconv.FormatUint(uint64(a.PaneID), 10)
}

// codexInput is what resolveCodex needs from the hook invocation.
type codexInput struct {
	// Event is the hook event name; SessionStart (startup, resume, clear,
	// compact, fork) may rebind a session to another client.
	Event     string
	SessionID string
	// Cwd is the payload cwd as sent by Codex.
	Cwd string
	// CodexHome is the Codex home of the daemon running the hook: the hook's
	// CODEX_HOME, else $HOME/.codex, symlink-resolved.
	CodexHome string
}

// codexClient is a Codex process as attribution sees it.
type codexClient struct {
	pid           int
	start         int64
	dir           string // symlink-resolved working directory
	codexHome     string // symlink-resolved Codex home
	paneID        string // ZELLIJ_PANE_ID, empty outside Zellij
	zellijSession string // ZELLIJ_SESSION_NAME
	profile       profileEnv
	// bindable is true for interactive terminal clients, the only ones that
	// may receive a sidebar row.
	bindable bool
}

// zellijPane returns the client's pane and Zellij session, and false when
// the client does not run in a Zellij pane.
func (c codexClient) zellijPane() (uint32, string, bool) {
	if c.paneID == "" || !validZellijSessionName(c.zellijSession) {
		return 0, "", false
	}
	id, err := strconv.ParseUint(c.paneID, 10, 32)
	if err != nil {
		return 0, "", false
	}
	return uint32(id), c.zellijSession, true
}

// validZellijSessionName reports whether s can safely name the target of
// `zellij --session <s> pipe`. The name comes from another process's
// environment or from the hook payload, so it must not be read as an option
// (leading "-"), must not reach outside the socket directory ("/"), and must
// not carry control characters.
func validZellijSessionName(s string) bool {
	if s == "" || len(s) > 256 || strings.HasPrefix(s, "-") || strings.ContainsRune(s, '/') {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// isSharedDaemon reports whether p belongs to the Codex shared daemon: the
// managed app-server (`codex app-server --listen unix://... --managed-daemon`)
// or one of its helpers (`codex app-server daemon ...`).
//
// Other app-servers, such as the stdio app-server an IDE extension runs for
// itself, are not the shared daemon: their hooks keep today's attribution,
// and they count as (non-bindable) matching clients.
func isSharedDaemon(p procinfo.Process) bool {
	if p.Basename() != procinfo.CodexComm {
		return false
	}
	idx := -1
	for i, a := range p.Args {
		if a == "app-server" {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	rest := p.Args[idx+1:]
	if len(rest) > 0 && rest[0] == "daemon" {
		return true
	}
	for i, a := range rest {
		switch {
		case a == "--managed-daemon":
			return true
		case a == "--listen" && i+1 < len(rest) && strings.HasPrefix(rest[i+1], "unix:"):
			return true
		case strings.HasPrefix(a, "--listen=unix:"):
			return true
		}
	}
	return false
}

// hostKind is the outcome of the ancestor walk.
type hostKind int

const (
	// hostOwn: no shared daemon among the ancestors; the hook's own pane
	// identity is the client's (FR-003).
	hostOwn hostKind = iota
	// hostDaemon: the hook runs inside the shared daemon.
	hostDaemon
	// hostUnknown: an ancestor could not be inspected, so the hook cannot
	// tell whether its pane identity is its own or inherited.
	hostUnknown
)

// codexHost walks at most eight ancestors of this hook process, stopping at
// pid 1, and reports whether one of them is the Codex shared daemon. A
// lookup failure yields hostUnknown: trusting the possibly inherited pane
// could put the event in a wrong row, so such events are dropped. The
// second result describes the failure for the debug log.
//
// An ancestor owned by another user (sshd, sudo, login) ends the walk with
// hostOwn: the shared daemon belongs to this user and starts its hooks
// itself, so it cannot sit above a process of another user.
func codexHost(src procinfo.Source) (hostKind, string) {
	p, err := src.Ancestor(src.Self())
	if err != nil {
		return hostUnknown, "self: " + err.Error()
	}
	for i := 0; i < daemonAncestorDepthLimit; i++ {
		if p.PPID <= 1 {
			return hostOwn, ""
		}
		p, err = src.Ancestor(p.PPID)
		if errors.Is(err, procinfo.ErrForeign) {
			return hostOwn, ""
		}
		if err != nil {
			return hostUnknown, "ancestor: " + err.Error()
		}
		if isSharedDaemon(p) {
			return hostDaemon, ""
		}
	}
	return hostOwn, ""
}

// codexValueFlags are Codex CLI options that consume the next argument when
// written without "=". Knowing them is what lets toClient find the first
// non-flag argument (the subcommand, if any).
var codexValueFlags = map[string]bool{
	"-C": true, "--cd": true,
	"-c": true, "--config": true,
	"-m": true, "--model": true,
	"-p": true, "--profile": true,
	"-s": true, "--sandbox": true,
	"-a": true, "--ask-for-approval": true,
	"-i": true, "--image": true,
	"--enable": true, "--disable": true,
	"--add-dir": true, "--local-provider": true,
	"--remote": true, "--remote-auth-token-env": true,
}

// bindableSubcommands are the first non-flag arguments of interactive
// terminal clients (besides none at all).
var bindableSubcommands = map[string]bool{"resume": true, "fork": true}

// parseCodexArgs returns the first non-flag argument and the explicit
// working directory argument (-C, --cd, --cd=, -C=, -C<dir>), if any.
// promptOnly is true when the first non-flag argument follows "--", where
// Codex reads it as the initial prompt even if it spells a subcommand.
func parseCodexArgs(args []string) (first string, cd string, hasCd bool, promptOnly bool) {
	firstSet := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			if !firstSet && i+1 < len(args) {
				first, firstSet, promptOnly = args[i+1], true, true
			}
			break
		}
		switch {
		case a == "-C" || a == "--cd":
			if i+1 < len(args) {
				cd, hasCd = args[i+1], true
				i++
			}
			continue
		case strings.HasPrefix(a, "--cd="):
			cd, hasCd = strings.TrimPrefix(a, "--cd="), true
			continue
		case strings.HasPrefix(a, "-C"):
			cd, hasCd = strings.TrimPrefix(strings.TrimPrefix(a, "-C"), "="), true
			continue
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			if codexValueFlags[a] && i+1 < len(args) {
				i++
			}
			continue
		}
		if !firstSet {
			first, firstSet = a, true
		}
	}
	return first, cd, hasCd, promptOnly
}

// looksLikeSubcommand reports whether s has the shape of a Codex subcommand
// name. Anything else in the first positional slot is the initial prompt of
// an interactive session (`codex "fix the tests"`).
func looksLikeSubcommand(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

// resolvePath makes p absolute against base, cleans it and resolves symbolic
// links. A path that does not exist (any longer) is only cleaned, so it still
// compares equal to the same spelling on the other side.
func resolvePath(p, base string) string {
	if p == "" {
		return ""
	}
	if !filepath.IsAbs(p) {
		if base == "" {
			return ""
		}
		p = filepath.Join(base, p)
	}
	p = filepath.Clean(p)
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return filepath.Clean(r)
	}
	return p
}

// codexHomeFor returns the symlink-resolved Codex home named by env:
// CODEX_HOME, else $HOME/.codex, with fallbackHome standing in for a missing
// HOME.
func codexHomeFor(env map[string]string, fallbackHome string) string {
	if h := env["CODEX_HOME"]; h != "" {
		return resolvePath(h, env["PWD"])
	}
	home := env["HOME"]
	if home == "" {
		home = fallbackHome
	}
	if home == "" {
		return ""
	}
	return resolvePath(filepath.Join(home, ".codex"), "")
}

// toClient derives the attribution view of a Codex process. The boolean is
// false when p cannot be a matching client at all: not a codex executable,
// a process of the shared daemon (the daemon and its helpers never count), or no
// determinable working directory. Whether a matching client may be bound
// to a pane is reported separately in codexClient.bindable.
//
// home stands in for $HOME when the process environment has none.
func toClient(p procinfo.Process, home string) (codexClient, bool) {
	if p.Basename() != procinfo.CodexComm || isSharedDaemon(p) {
		return codexClient{}, false
	}
	first, cd, hasCd, promptOnly := parseCodexArgs(p.Args)

	// The real working directory (Linux only) is authoritative for a client
	// without -C: PWD is stale when a launcher other than a shell changed
	// the directory (a Zellij command pane with cwd, `zellij run --cwd`).
	// A relative -C is resolved against PWD, the directory the user typed
	// it in, because Codex may already have changed into the -C directory.
	base := p.Env["PWD"]
	if base == "" {
		base = p.Cwd
	}
	var dir string
	switch {
	case hasCd:
		dir = resolvePath(cd, base)
	case p.Cwd != "":
		dir = resolvePath(p.Cwd, "")
	default:
		dir = resolvePath(base, "")
	}
	if dir == "" {
		return codexClient{}, false
	}

	c := codexClient{
		pid:           p.PID,
		start:         p.StartTime,
		dir:           dir,
		codexHome:     codexHomeFor(p.Env, home),
		paneID:        p.Env["ZELLIJ_PANE_ID"],
		zellijSession: p.Env["ZELLIJ_SESSION_NAME"],
		profile: profileEnv{
			Name:  p.Env["CC_DECK_PROFILE"],
			Color: p.Env["CC_DECK_PROFILE_COLOR"],
			Icon:  p.Env["CC_DECK_PROFILE_ICON"],
		},
	}
	c.bindable = first == "" || promptOnly || bindableSubcommands[first] || !looksLikeSubcommand(first)
	return c, true
}

// hookCodexInput builds the resolver input from the payload and the hook's
// own environment (the daemon's Codex home).
func hookCodexInput(event, sessionID, cwd string) codexInput {
	env := map[string]string{
		"CODEX_HOME": os.Getenv("CODEX_HOME"),
		"HOME":       os.Getenv("HOME"),
		"PWD":        os.Getenv("PWD"),
	}
	home, _ := os.UserHomeDir()
	return codexInput{Event: event, SessionID: sessionID, Cwd: cwd, CodexHome: codexHomeFor(env, home)}
}

// resolveCodex attributes a daemon-hosted Codex event (contract section 2,
// steps 3 to 5). It never looks at the hook's inherited pane or session.
//
// A session whose remembered client is gone (exited, pid reused) is not
// rediscovered on ordinary events: its trailing events would otherwise land
// on whichever client now happens to be the only one in that directory.
// Only SessionStart (Codex fires it for startup, resume, clear, compact and
// fork) runs discovery for such a session.
//
// On SessionStart a unique discovered client also wins over a live
// mapping, so a session resumed in another client follows it. When
// discovery is not unique, the live mapping stays only if its client is
// among the matching ones (same directory and Codex home, for example a
// compaction while another client shares the directory). Otherwise the
// session has demonstrably moved: the event is dropped and the mapping is
// forgotten (Forget), so later events do not reach the old client's row.
//
// A session that was never bound has no mapping and runs discovery on every
// event (FR-007), so its trailing events after its client exited can still
// reach the one remaining client in that directory; the hook cannot tell
// them apart from a first attributable event.
func resolveCodex(in codexInput, src procinfo.Source, paneMap map[string]paneMapEntry) attribution {
	home, _ := os.UserHomeDir()
	start := in.Event == "SessionStart"

	// Step 3: a live cached mapping that names its client.
	var cached *attribution
	if e, ok := paneMap[in.SessionID]; ok && in.SessionID != "" && e.ClientPID > 0 {
		att, live := cachedClient(e, src, home)
		switch {
		case live && !start:
			return att
		case live:
			cached = &att
		case !start:
			return attribution{Reason: reasonNoClient, Detail: "stale-mapping"}
		}
	}

	att := discoverClient(in, src, home)
	if att.Reason == reasonDiscovered || cached == nil {
		return att
	}
	if cached.clientDir == resolvePath(in.Cwd, "") && cached.clientHome == in.CodexHome {
		return *cached
	}
	att.Forget = true
	if att.Detail == "" {
		att.Detail = "session-moved"
	}
	return att
}

// discoverClient is contract step 4: the single Codex process of this user
// and Codex home whose directory equals the event's cwd.
func discoverClient(in codexInput, src procinfo.Source, home string) attribution {
	cwd := resolvePath(in.Cwd, "")
	if cwd == "" || in.CodexHome == "" {
		return attribution{Reason: reasonNoClient, Detail: "no-cwd-or-codex-home"}
	}
	procs, err := src.ListUser()
	if err != nil {
		return attribution{Reason: reasonNoClient, Detail: "list: " + err.Error()}
	}
	var matches []codexClient
	for _, p := range procs {
		c, ok := toClient(p, home)
		if !ok || c.codexHome != in.CodexHome || c.dir != cwd {
			continue
		}
		matches = append(matches, c)
	}
	switch {
	case len(matches) == 0:
		return attribution{Reason: reasonNoClient}
	case len(matches) > 1:
		// Several clients share this directory and Codex home. Whether some
		// of them already own sessions does not help: a bound client
		// starting another session looks exactly like a second client
		// starting its first.
		return attribution{Reason: reasonAmbiguous}
	}
	c := matches[0]
	if !c.bindable {
		return attribution{Reason: reasonNoClient, Detail: "single-non-interactive"}
	}
	pane, session, inZellij := c.zellijPane()
	if !inZellij {
		return attribution{Reason: reasonClientNotInZellij}
	}
	return attribution{
		PaneID:        pane,
		ZellijSession: session,
		Profile:       c.profile,
		Reason:        reasonDiscovered,
		ClientPID:     c.pid,
		ClientStart:   c.start,
	}
}

// cachedClient validates a cached mapping: its client still runs (same pid
// and start time), is still an interactive Codex client, and still sits in
// the recorded pane of the recorded Zellij session.
func cachedClient(e paneMapEntry, src procinfo.Source, home string) (attribution, bool) {
	p, err := src.Get(e.ClientPID)
	if err != nil || p.StartTime != e.ClientStart {
		return attribution{}, false
	}
	c, ok := toClient(p, home)
	if !ok || !c.bindable {
		return attribution{}, false
	}
	pane, session, inZellij := c.zellijPane()
	if !inZellij || pane != e.PaneID || session != e.ZellijSession {
		return attribution{}, false
	}
	prof := profileEnv{Name: e.Profile}
	if c.profile.Name == e.Profile {
		prof = c.profile
	}
	return attribution{
		PaneID:        pane,
		ZellijSession: session,
		Profile:       prof,
		Reason:        reasonFromClientCache,
		ClientPID:     c.pid,
		ClientStart:   c.start,
		clientDir:     c.dir,
		clientHome:    c.codexHome,
	}, true
}

// clientEnvTarget implements contract step 1 (FR-011): a Codex payload that
// carries the originating client's environment (client_env) names the pane
// explicitly, and that identity wins over the cache and discovery.
//
// The target session is client_env's ZELLIJ_SESSION_NAME. Without it, a hook
// that is not daemon-hosted may use its own session (it is the client's),
// while a daemon-hosted hook cannot (its session is inherited) and reports
// false so that the cache and discovery steps run. The profile comes from
// client_env, else, for hooks that are not daemon-hosted, from the hook's own
// environment.
func clientEnvTarget(clientEnv map[string]string, sessionInherited bool, hookSession string) (attribution, bool) {
	raw := clientEnv["ZELLIJ_PANE_ID"]
	if raw == "" {
		return attribution{}, false
	}
	pane, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return attribution{}, false
	}
	session := clientEnv["ZELLIJ_SESSION_NAME"]
	if session == "" && !sessionInherited {
		session = hookSession
	}
	if !validZellijSessionName(session) {
		return attribution{}, false
	}
	prof := profileEnv{
		Name:  clientEnv["CC_DECK_PROFILE"],
		Color: clientEnv["CC_DECK_PROFILE_COLOR"],
		Icon:  clientEnv["CC_DECK_PROFILE_ICON"],
	}
	if prof.Name == "" && !sessionInherited {
		prof = profileEnv{
			Name:  os.Getenv("CC_DECK_PROFILE"),
			Color: os.Getenv("CC_DECK_PROFILE_COLOR"),
			Icon:  os.Getenv("CC_DECK_PROFILE_ICON"),
		}
	}
	return attribution{
		PaneID:        uint32(pane),
		ZellijSession: session,
		Profile:       prof,
		Reason:        reasonClientEnv,
	}, true
}

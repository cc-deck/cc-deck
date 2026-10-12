package share

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// zellijCommandTimeout bounds every zellij CLI call. A wedged Zellij session
// server still accepts the connection but never answers, so an unbounded call
// blocks forever. Sharing status is consulted by ordinary commands such as
// "cc-deck ws", so one unresponsive session must not hang the whole CLI.
// Declared as a variable so tests can shorten it.
var zellijCommandTimeout = 10 * time.Second

// ErrZellijUnresponsive marks a call that timed out. Like every other error
// from a zellij call, it is inconclusive: only a session positively reported
// absent is evidence that sharing ended, so cleanup never runs on any error.
// The sentinel lets callers name the timeout case in their diagnostics.
var ErrZellijUnresponsive = errors.New("zellij is not responding")

type ZellijCLI struct{ runner CommandRunner }

func NewZellij(r CommandRunner) *ZellijCLI { return &ZellijCLI{runner: r} }
func (z *ZellijCLI) run(ctx context.Context, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// Only impose the default bound when the caller has not set its own, so a
	// caller with a shorter deadline keeps it.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, zellijCommandTimeout)
		defer cancel()
	}
	b, e := z.runner.Run(ctx, "zellij", args...)
	// The output is returned alongside a failure as well: Zellij reports some
	// ordinary answers, such as an empty session list, through a non-zero exit,
	// and a caller can only tell those apart from real failures by reading it.
	out := strings.TrimSpace(string(b))
	if e != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return out, fmt.Errorf("zellij %s: %w (timed out after %s): %w",
				strings.Join(args, " "), ErrZellijUnresponsive, zellijCommandTimeout, ctx.Err())
		}
		return out, fmt.Errorf("zellij %s: %w", strings.Join(args, " "), e)
	}
	return out, nil
}
func (z *ZellijCLI) ValidateCapabilities(ctx context.Context) error {
	v, err := z.run(ctx, "--version")
	if err != nil {
		return fmt.Errorf("Zellij 0.44.3 or newer with web sharing is required: %w", err)
	}
	re := regexp.MustCompile(`(?:zellij\s+)?(\d+)\.(\d+)\.(\d+)`)
	m := re.FindStringSubmatch(v)
	if len(m) == 0 {
		return fmt.Errorf("cannot determine Zellij version from %q", v)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	if major == 0 && (minor < 44 || (minor == 44 && patch < 3)) {
		return fmt.Errorf("Zellij 0.44.3 or newer is required (found %s)", m[0])
	}
	webHelp, err := z.run(ctx, "web", "--help")
	if err != nil {
		return fmt.Errorf("Zellij web sharing capability unavailable: %w", err)
	}
	for _, flag := range []string{"--start", "--status", "--daemonize", "--create-token", "--token-name", "--create-read-only-token", "--revoke-token", "--stop"} {
		if !strings.Contains(webHelp, flag) {
			return fmt.Errorf("Zellij web sharing capability %s unavailable", flag)
		}
	}
	attachHelp, err := z.run(ctx, "attach", "--help")
	if err != nil || !strings.Contains(attachHelp, "--token") {
		return fmt.Errorf("Zellij remote attach capability unavailable")
	}
	optionsHelp, err := z.run(ctx, "options", "--help")
	if err != nil || !strings.Contains(optionsHelp, "--web-sharing") {
		return fmt.Errorf("Zellij session-specific web sharing capability unavailable")
	}
	return nil
}
func (z *ZellijCLI) SessionExists(ctx context.Context, requested string) (bool, error) {
	out, err := z.run(ctx, "list-sessions", "--no-formatting")
	if err != nil {
		// Zellij answers an empty session list with a message and a non-zero
		// exit. That is a positive answer, not an inability to ask, and must
		// not be confused with the exec and timeout failures that are.
		if strings.Contains(out, "No active zellij sessions") {
			return false, nil
		}
		return false, err
	}
	var matches []string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "(EXITED") {
			continue
		}
		name := parseSessionName(line)
		if name == "" {
			continue
		}
		if requested == "" || name == requested {
			matches = append(matches, name)
		}
	}
	if requested != "" && len(matches) == 1 {
		return true, nil
	}
	if requested == "" && len(matches) == 1 {
		return true, nil
	}
	if len(matches) == 0 {
		return false, nil
	}
	return false, nil
}
func parseSessionName(line string) string {
	line = strings.TrimSpace(line)
	for _, marker := range []string{" [Created ", " [EXITED ", " [Current session]"} {
		if i := strings.Index(line, marker); i >= 0 {
			return strings.TrimSpace(line[:i])
		}
	}
	return line
}
func (z *ZellijCLI) CreateToken(ctx context.Context, _ string, readOnly bool) (TokenCredential, error) {
	flag := "--create-token"
	if readOnly {
		flag = "--create-read-only-token"
	}
	out, err := z.run(ctx, "web", flag)
	if err != nil {
		return TokenCredential{}, err
	}
	return parseTokenCredential(out, readOnly)
}

func parseTokenCredential(out string, readOnly bool) (TokenCredential, error) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		return TokenCredential{}, fmt.Errorf("parse Zellij token output: empty output")
	}
	last := strings.TrimSpace(lines[len(lines)-1])
	if readOnly {
		last = strings.TrimSuffix(last, " (read-only)")
	}
	name, secret, found := strings.Cut(last, ": ")
	if !found || strings.TrimSpace(name) == "" || strings.TrimSpace(secret) == "" {
		return TokenCredential{}, fmt.Errorf("parse Zellij token output: unexpected response")
	}
	return TokenCredential{Name: strings.TrimSpace(name), Secret: strings.TrimSpace(secret)}, nil
}
func (z *ZellijCLI) RevokeToken(ctx context.Context, label string) error {
	out, err := z.run(ctx, "web", "--revoke-token", label)
	if err != nil && strings.Contains(out, "Token by that name does not exist") {
		// Revocation is idempotent. A token Zellij no longer has is the
		// outcome wanted, whether an earlier cleanup already removed it or
		// the user revoked it by hand; reporting failure here would leave the
		// operation degraded for good with nothing left to clean up.
		return nil
	}
	return err
}
func (z *ZellijCLI) EnsureWebServer(ctx context.Context) (string, bool, error) {
	out, err := z.run(ctx, "web", "--status")
	if err == nil && strings.Contains(strings.ToLower(out), "online") {
		return extractLocalURL(out), false, nil
	}
	if _, err = z.run(ctx, "web", "--daemonize"); err != nil {
		return "", false, err
	}
	out, err = z.run(ctx, "web", "--status")
	if err != nil {
		return "", true, err
	}
	if !strings.Contains(strings.ToLower(out), "online") {
		return "", true, fmt.Errorf("Zellij web server did not become ready: %s", out)
	}
	return extractLocalURL(out), true, nil
}
func extractLocalURL(out string) string {
	re := regexp.MustCompile(`https?://[^\s]+`)
	if u := re.FindString(out); u != "" {
		return strings.TrimRight(u, ".,")
	}
	return "http://127.0.0.1:8082"
}
func (z *ZellijCLI) StopWebServer(ctx context.Context) error {
	_, e := z.run(ctx, "web", "--stop")
	return e
}

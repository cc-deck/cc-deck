package share

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type ZellijCLI struct{ runner CommandRunner }

func NewZellij(r CommandRunner) *ZellijCLI { return &ZellijCLI{runner: r} }
func (z *ZellijCLI) run(ctx context.Context, args ...string) (string, error) {
	b, e := z.runner.Run(ctx, "zellij", args...)
	if e != nil {
		return "", fmt.Errorf("zellij %s: %w", strings.Join(args, " "), e)
	}
	return strings.TrimSpace(string(b)), nil
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
func (z *ZellijCLI) ResolveSession(ctx context.Context, requested string) (string, error) {
	out, err := z.run(ctx, "list-sessions", "--no-formatting")
	if err != nil {
		return "", err
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
		return matches[0], nil
	}
	if requested == "" && len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("running Zellij session %q not found", requested)
	}
	return "", fmt.Errorf("select one Zellij session explicitly")
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
func (z *ZellijCLI) ShareSession(ctx context.Context, s string) (bool, error) {
	out, err := z.run(ctx, "--session", s, "options", "--web-sharing", "on")
	if err != nil {
		return false, err
	}
	if strings.Contains(strings.ToLower(out), "already shared") {
		return false, nil
	}
	return true, nil
}
func (z *ZellijCLI) UnshareSession(ctx context.Context, s string) error {
	_, e := z.run(ctx, "--session", s, "options", "--web-sharing", "off")
	return e
}
func (z *ZellijCLI) CreateToken(ctx context.Context, label string, readOnly bool) (string, error) {
	flag := "--create-token"
	if readOnly {
		flag = "--create-read-only-token"
	}
	if _, err := z.run(ctx, "web", "--token-name", label); err != nil {
		return "", fmt.Errorf("set Zellij token name: %w", err)
	}
	return z.run(ctx, "web", flag)
}
func (z *ZellijCLI) RevokeToken(ctx context.Context, label string) error {
	_, e := z.run(ctx, "web", "--revoke-token", label)
	return e
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

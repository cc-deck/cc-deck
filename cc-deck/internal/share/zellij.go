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
	if _, err = z.run(ctx, "web", "--help"); err != nil {
		return fmt.Errorf("Zellij web sharing capability unavailable: %w", err)
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
		name := strings.Fields(strings.TrimSpace(line))
		if len(name) == 0 {
			continue
		}
		if requested == "" || name[0] == requested {
			matches = append(matches, name[0])
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
func (z *ZellijCLI) ShareSession(ctx context.Context, s string) error {
	_, e := z.run(ctx, "--session", s, "options", "--web-sharing", "on")
	return e
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
	return z.run(ctx, "web", flag, "--token-name", label)
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

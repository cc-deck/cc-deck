package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/profile"
	"github.com/cc-deck/cc-deck/internal/xdg"
)

// withProfileConfig points config.Load at a temp config.yaml holding the
// given profiles section, so runHook's profile enrichment has something to
// look up without touching the developer's real configuration.
func withProfileConfig(t *testing.T, profilesYAML string) {
	t.Helper()
	dir := t.TempDir()
	orig := xdg.ConfigHome
	xdg.ConfigHome = dir
	t.Cleanup(func() { xdg.ConfigHome = orig })

	cfgDir := filepath.Join(dir, "cc-deck")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(profilesYAML), 0o600); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
}

const hookProfileFixture = `profiles:
  work:
    harness: claude
    auth:
      login: true
    color: "#4FC1E9"
    icon: "W"
  plain:
    harness: claude
    auth:
      login: true
`

func runProfiledHook(t *testing.T, env *hookTestEnv, profileName string) agent.NormalizedPayload {
	t.Helper()
	t.Setenv("CC_DECK_PROFILE", profileName)
	env.inZellij("cc-deck-local")

	runHook(strings.NewReader(claudeEvent("SessionStart", "sess-"+profileName)), "9", "claude")

	if len(env.sent) != 1 {
		t.Fatalf("expected exactly one payload, got %d", len(env.sent))
	}
	var got agent.NormalizedPayload
	if err := json.Unmarshal(env.sent[0], &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	return got
}

func TestRunHook_NoProfileEnvLeavesProfileFieldsEmpty(t *testing.T) {
	env := newHookTestEnv(t)
	withProfileConfig(t, hookProfileFixture)
	got := runProfiledHook(t, env, "")

	if got.Profile != "" || got.ProfileColor != "" {
		t.Errorf("expected no profile fields, got profile=%q color=%q", got.Profile, got.ProfileColor)
	}
	if got.AgentIndicator != agent.Get("claude").Indicator() {
		t.Errorf("indicator: got %q, want the claude indicator", got.AgentIndicator)
	}
}

func TestRunHook_KnownProfileWithColorAndIcon(t *testing.T) {
	env := newHookTestEnv(t)
	withProfileConfig(t, hookProfileFixture)
	got := runProfiledHook(t, env, "work")

	if got.Profile != "work" {
		t.Errorf("profile: got %q, want %q", got.Profile, "work")
	}
	if got.ProfileColor != "#4FC1E9" {
		t.Errorf("color: got %q, want declared %q", got.ProfileColor, "#4FC1E9")
	}
	if got.AgentIndicator != "W" {
		t.Errorf("indicator: got %q, want the profile icon %q", got.AgentIndicator, "W")
	}
}

func TestRunHook_KnownProfileWithoutColorDerivesColorAndKeepsIndicator(t *testing.T) {
	env := newHookTestEnv(t)
	withProfileConfig(t, hookProfileFixture)
	got := runProfiledHook(t, env, "plain")

	if got.Profile != "plain" {
		t.Errorf("profile: got %q, want %q", got.Profile, "plain")
	}
	if want := profile.Derive("plain"); got.ProfileColor != want {
		t.Errorf("color: got %q, want derived %q", got.ProfileColor, want)
	}
	if got.AgentIndicator != agent.Get("claude").Indicator() {
		t.Errorf("indicator: got %q, want the claude indicator (no icon declared)", got.AgentIndicator)
	}
}

func TestRunHook_UnknownProfileStillSendsNameWithDerivedColor(t *testing.T) {
	env := newHookTestEnv(t)
	withProfileConfig(t, hookProfileFixture)
	got := runProfiledHook(t, env, "ghost")

	if got.Profile != "ghost" {
		t.Errorf("profile: got %q, want %q", got.Profile, "ghost")
	}
	if want := profile.Derive("ghost"); got.ProfileColor != want {
		t.Errorf("color: got %q, want derived %q", got.ProfileColor, want)
	}
	if got.AgentIndicator != agent.Get("claude").Indicator() {
		t.Errorf("indicator: got %q, want the claude indicator", got.AgentIndicator)
	}
}

func TestRunHook_ProfileWithoutConfigFileDerivesColor(t *testing.T) {
	env := newHookTestEnv(t)
	orig := xdg.ConfigHome
	xdg.ConfigHome = t.TempDir() // no config.yaml at all
	t.Cleanup(func() { xdg.ConfigHome = orig })
	got := runProfiledHook(t, env, "solo")

	if got.Profile != "solo" {
		t.Errorf("profile: got %q, want %q", got.Profile, "solo")
	}
	if want := profile.Derive("solo"); got.ProfileColor != want {
		t.Errorf("color: got %q, want derived %q", got.ProfileColor, want)
	}
}

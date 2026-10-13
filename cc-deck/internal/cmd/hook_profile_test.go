package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/profile"
	"github.com/cc-deck/cc-deck/internal/xdg"
)

// withProfileConfig points config.Load at a temp config.yaml holding the
// given profiles section, so runHook's profile enrichment has something to
// look up without touching the developer's real configuration.
func withProfileConfig(t *testing.T, profilesYAML string) {
	t.Helper()
	clearProfileEnv(t)
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

// clearProfileEnv removes wrapper exports inherited from the developer's
// shell so the config fallback path is what the test exercises.
func clearProfileEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CC_DECK_PROFILE_COLOR", "")
	t.Setenv("CC_DECK_PROFILE_ICON", "")
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
	clearProfileEnv(t)
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

// countProfileConfigLoads wraps the profile config loader and returns a
// pointer to the number of loads enrichProfile performs.
func countProfileConfigLoads(t *testing.T) *int {
	t.Helper()
	calls := 0
	orig := loadProfileConfig
	loadProfileConfig = func(path string) (*config.Config, error) {
		calls++
		return orig(path)
	}
	t.Cleanup(func() { loadProfileConfig = orig })
	return &calls
}

func TestRunHook_EnvColorAndIconWithoutConfigRead(t *testing.T) {
	// SC-010: a wrapper-launched session carries color and icon in its
	// environment, so the hook needs no config.yaml at all.
	env := newHookTestEnv(t)
	withProfileConfig(t, hookProfileFixture)
	loads := countProfileConfigLoads(t)
	t.Setenv("CC_DECK_PROFILE_COLOR", "#8686EA")
	t.Setenv("CC_DECK_PROFILE_ICON", "P")
	got := runProfiledHook(t, env, "work")

	if *loads != 0 {
		t.Errorf("profile config loads: got %d, want 0", *loads)
	}
	if got.ProfileIcon != "P" {
		t.Errorf("profile_icon: got %q, want %q", got.ProfileIcon, "P")
	}

	if got.Profile != "work" {
		t.Errorf("profile: got %q, want %q", got.Profile, "work")
	}
	if got.ProfileColor != "#8686EA" {
		t.Errorf("color: got %q, want the exported %q", got.ProfileColor, "#8686EA")
	}
	if got.AgentIndicator != "P" {
		t.Errorf("indicator: got %q, want the exported icon %q", got.AgentIndicator, "P")
	}
}

func TestRunHook_EnvColorWithoutIconKeepsIndicator(t *testing.T) {
	env := newHookTestEnv(t)
	withProfileConfig(t, hookProfileFixture)
	loads := countProfileConfigLoads(t)
	t.Setenv("CC_DECK_PROFILE_COLOR", "#5AE29E")
	t.Setenv("CC_DECK_PROFILE_ICON", "")
	got := runProfiledHook(t, env, "work")

	if *loads != 0 {
		t.Errorf("profile config loads: got %d, want 0", *loads)
	}
	if got.ProfileIcon != "" {
		t.Errorf("profile_icon: got %q, want empty", got.ProfileIcon)
	}

	if got.ProfileColor != "#5AE29E" {
		t.Errorf("color: got %q, want %q", got.ProfileColor, "#5AE29E")
	}
	if got.AgentIndicator != agent.Get("claude").Indicator() {
		t.Errorf("indicator: got %q, want the claude indicator", got.AgentIndicator)
	}
}

func TestRunHook_InvalidEnvColorFallsBackToConfig(t *testing.T) {
	for _, bad := range []string{"red", "#12345"} {
		t.Run(bad, func(t *testing.T) {
			env := newHookTestEnv(t)
			withProfileConfig(t, hookProfileFixture)
			loads := countProfileConfigLoads(t)
			t.Setenv("CC_DECK_PROFILE_COLOR", bad)
			t.Setenv("CC_DECK_PROFILE_ICON", "X")
			got := runProfiledHook(t, env, "work")

			if *loads != 1 {
				t.Errorf("profile config loads: got %d, want 1 (fallback)", *loads)
			}

			if got.ProfileColor != "#4FC1E9" {
				t.Errorf("color: got %q, want the declared %q", got.ProfileColor, "#4FC1E9")
			}
			if got.AgentIndicator != "W" {
				t.Errorf("indicator: got %q, want the declared icon %q", got.AgentIndicator, "W")
			}
		})
	}
}

func TestRunHook_UnknownProfileWithoutEnvColorDerives(t *testing.T) {
	env := newHookTestEnv(t)
	withProfileConfig(t, hookProfileFixture)
	got := runProfiledHook(t, env, "stranger")

	if want := profile.Derive("stranger"); got.ProfileColor != want {
		t.Errorf("color: got %q, want derived %q", got.ProfileColor, want)
	}
}

func TestRunHook_UnsafeEnvIconIgnored(t *testing.T) {
	for _, icon := range []string{"\x1b[2J", "AB", "\u200b"} {
		t.Run(icon, func(t *testing.T) {
			env := newHookTestEnv(t)
			withProfileConfig(t, hookProfileFixture)
			t.Setenv("CC_DECK_PROFILE_COLOR", "#5AE29E")
			t.Setenv("CC_DECK_PROFILE_ICON", icon)
			got := runProfiledHook(t, env, "work")

			if got.AgentIndicator != agent.Get("claude").Indicator() {
				t.Errorf("indicator: got %q, want the claude indicator", got.AgentIndicator)
			}
			if got.ProfileIcon != "" {
				t.Errorf("profile_icon: got %q, want empty", got.ProfileIcon)
			}
		})
	}
}

func TestRunHook_InvalidConfigColorDerives(t *testing.T) {
	// FR-014a fallback: a hand-edited, invalid color must not reach the
	// plugin; the session still counts as profiled with the derived color.
	env := newHookTestEnv(t)
	withProfileConfig(t, `profiles:
  work:
    harness: claude
    auth:
      login: true
    color: "red"
`)
	got := runProfiledHook(t, env, "work")

	if want := profile.Derive("work"); got.ProfileColor != want {
		t.Errorf("color: got %q, want derived %q", got.ProfileColor, want)
	}
}

func TestRunHook_ConfigIconSetsProfileIcon(t *testing.T) {
	env := newHookTestEnv(t)
	withProfileConfig(t, hookProfileFixture)
	got := runProfiledHook(t, env, "work")

	if got.ProfileIcon != "W" {
		t.Errorf("profile_icon: got %q, want %q", got.ProfileIcon, "W")
	}
}

func TestRunHook_UnsafeConfigIconIgnored(t *testing.T) {
	env := newHookTestEnv(t)
	withProfileConfig(t, "profiles:\n  work:\n    harness: claude\n    auth:\n      login: true\n    icon: \"\\e[2J\"\n")
	got := runProfiledHook(t, env, "work")

	if got.ProfileIcon != "" {
		t.Errorf("profile_icon: got %q, want empty", got.ProfileIcon)
	}
	if got.AgentIndicator != agent.Get("claude").Indicator() {
		t.Errorf("indicator: got %q, want the claude indicator", got.AgentIndicator)
	}
}

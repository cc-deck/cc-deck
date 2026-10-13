package profile_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTranslatorContract verifies the six behaviors from
// contracts/harness-translator.md section 2.5 against every registered
// translator.
func TestTranslatorContract(t *testing.T) {
	all := profile.All()
	if len(all) == 0 {
		t.Skip("no translators registered yet")
	}

	for harness, tr := range all {
		t.Run(harness, func(t *testing.T) {
			a := agent.Get(harness)
			require.NotNil(t, a, "agent.Get(%q) returned nil", harness)

			p := testProfile(tr)
			rp, err := profile.Resolve("contract-test", p, a)
			require.NoError(t, err)
			// Override paths to temp dirs for testing
			tmpDir := t.TempDir()
			rp.ConfigDir = filepath.Join(tmpDir, "config", harness)
			rp.CredDir = filepath.Join(tmpDir, "cred")
			rp.BinDir = filepath.Join(tmpDir, "bin")

			// 2. Render output contains no credential value.
			// Set the env var BEFORE rendering so the value is available in
			// the process environment when the wrapper is generated.
			testEnvVal := "TEST_SECRET_VALUE_12345"
			t.Setenv("CONTRACT_TEST_KEY", testEnvVal)

			// 1. Render output passes sh -n
			ws, err := tr.Render(rp)
			require.NoError(t, err)
			tmpScript := filepath.Join(tmpDir, ws.Name)
			require.NoError(t, os.WriteFile(tmpScript, ws.Content, 0755))
			shPath, err := exec.LookPath("sh")
			if err == nil {
				out, err := exec.Command(shPath, "-n", tmpScript).CombinedOutput()
				assert.NoError(t, err, "sh -n failed: %s", string(out))
			}

			assert.NotContains(t, string(ws.Content), testEnvVal,
				"wrapper must not contain credential values")

			// 3. Render is deterministic
			ws2, err := tr.Render(rp)
			require.NoError(t, err)
			assert.Equal(t, ws.Content, ws2.Content, "Render must be deterministic")

			// 4. PrepareConfigDir links non-isolated, leaves isolated absent (or seeded)
			defaultDir := filepath.Join(tmpDir, "default-"+harness)
			require.NoError(t, os.MkdirAll(defaultDir, 0700))
			// Create sample entries in default dir
			require.NoError(t, os.WriteFile(filepath.Join(defaultDir, "shared.json"), []byte("{}"), 0644))
			require.NoError(t, os.MkdirAll(filepath.Join(defaultDir, "shared-dir"), 0755))
			for _, iso := range tr.IsolatedEntries() {
				require.NoError(t, os.WriteFile(filepath.Join(defaultDir, iso), []byte("isolated"), 0644))
			}

			_, err = tr.PrepareConfigDir(rp, defaultDir)
			require.NoError(t, err)

			// Non-isolated entries should be symlinked to the default dir.
			// Symlinks may be relative and macOS resolves /var -> /private/var,
			// so EvalSymlinks both sides for a stable comparison.
			sharedLink := filepath.Join(rp.ConfigDir, "shared.json")
			resolved, err := filepath.EvalSymlinks(sharedLink)
			if assert.NoError(t, err, "shared.json should be a resolvable symlink") {
				expectedRaw := filepath.Join(defaultDir, "shared.json")
				expected, _ := filepath.EvalSymlinks(expectedRaw)
				assert.Equal(t, expected, resolved,
					"shared.json symlink must resolve to the default dir entry")
			}

			// Isolated entries should NOT be symlinked from default
			for _, iso := range tr.IsolatedEntries() {
				target, err := os.Readlink(filepath.Join(rp.ConfigDir, iso))
				if err == nil {
					assert.NotContains(t, target, defaultDir,
						"isolated entry %q must not link to default dir", iso)
				}
			}

			// 5. PrepareConfigDir twice yields the same tree
			_, err = tr.PrepareConfigDir(rp, defaultDir)
			require.NoError(t, err, "second PrepareConfigDir must succeed (idempotent)")

			// 6. Lookup(a.Name()) succeeds for every agent
			_, ok := profile.Lookup(a.Name())
			assert.True(t, ok, "Lookup(%q) must succeed", a.Name())
		})
	}
}

// TestAgentTranslatorPairing verifies every registered agent has a translator.
func TestAgentTranslatorPairing(t *testing.T) {
	all := profile.All()
	if len(all) == 0 {
		t.Skip("no translators registered yet")
	}
	for _, a := range agent.All() {
		_, ok := profile.Lookup(a.Name())
		assert.True(t, ok, "agent %q has no translator", a.Name())
	}
}

// TestLoginRejection verifies that translators which do not support login
// return an error from Render when rp.Login is true (contract 2.2).
func TestLoginRejection(t *testing.T) {
	all := profile.All()
	for harness, tr := range all {
		if tr.SupportsLogin() {
			continue // skip translators that support login
		}
		t.Run(harness, func(t *testing.T) {
			a := agent.Get(harness)
			require.NotNil(t, a, "agent.Get(%q) returned nil", harness)

			p := testProfile(tr)
			// Force login mode in the auth config.
			p.Auth = &config.AuthConfig{Login: true}

			rp, err := profile.Resolve("login-test", p, a)
			require.NoError(t, err)

			_, err = tr.Render(rp)
			require.Error(t, err, "Render must fail for login on %s", harness)
			assert.Contains(t, err.Error(), "login is not supported",
				"error message must explain login is not supported")
		})
	}
}

// testProfile builds a minimal valid profile for a translator's harness.
func testProfile(tr profile.Translator) config.Profile {
	backends := tr.Backends()
	p := config.Profile{
		Harness: tr.Harness(),
		Backend: backends[0],
		Model:   "test-model",
		Auth: &config.AuthConfig{
			APIKey: &config.CredentialSource{Env: "CONTRACT_TEST_KEY"},
		},
	}
	if tr.SupportsLogin() {
		// Use API key for the contract test, login is tested separately
	}
	if backends[0] == config.BackendVertex {
		p.Project = "test-project"
		p.Region = "us-east5"
	}
	return p
}

// kindProfile builds a valid profile of the given auth kind for a translator,
// or reports false when the translator cannot express that kind.
func kindProfile(tr profile.Translator, kind profile.AuthKind) (config.Profile, bool) {
	direct := config.BackendType("")
	hasVertex := false
	for _, b := range tr.Backends() {
		if b == config.BackendVertex {
			hasVertex = true
		} else if direct == "" {
			direct = b
		}
	}
	p := config.Profile{Harness: tr.Harness(), Model: "test-model"}
	switch kind {
	case profile.AuthAPIKey:
		p.Backend = direct
		p.Auth = &config.AuthConfig{APIKey: &config.CredentialSource{Env: "CONTRACT_TEST_KEY"}}
	case profile.AuthOAuthToken:
		if !tr.SupportsOAuthToken() {
			return p, false
		}
		p.Backend = config.BackendAnthropic
		p.Auth = &config.AuthConfig{OAuthToken: &config.CredentialSource{Env: "CONTRACT_TEST_TOKEN"}}
	case profile.AuthLogin:
		if !tr.SupportsLogin() {
			return p, false
		}
		p.Backend = direct
		p.Auth = &config.AuthConfig{Login: true}
	case profile.AuthVertex:
		if !hasVertex {
			return p, false
		}
		p.Backend = config.BackendVertex
		p.Project = "test-project"
		p.Region = "us-east5"
		p.Auth = &config.AuthConfig{Credentials: &config.CredentialSource{File: "/tmp/adc.json"}}
	default:
		return p, false
	}
	return p, true
}

// expectedUnset returns the sorted variables of every kind other than own.
func expectedUnset(vars map[profile.AuthKind][]string, own profile.AuthKind) []string {
	var out []string
	for k, names := range vars {
		if k != own {
			out = append(out, names...)
		}
	}
	sort.Strings(out)
	return out
}

// TestTranslatorContractAuthClearing verifies contract behavior 7: every
// rendered wrapper exports CC_DECK_PROFILE_COLOR and, directly after the
// CC_DECK_PROFILE* exports, carries exactly the expected unset line.
func TestTranslatorContractAuthClearing(t *testing.T) {
	kinds := []profile.AuthKind{profile.AuthAPIKey, profile.AuthOAuthToken, profile.AuthLogin, profile.AuthVertex, profile.AuthBedrock}
	for harness, tr := range profile.All() {
		a := agent.Get(harness)
		require.NotNil(t, a, "agent.Get(%q) returned nil", harness)
		for _, kind := range kinds {
			p, ok := kindProfile(tr, kind)
			if !ok {
				continue
			}
			t.Run(harness+"/"+string(kind), func(t *testing.T) {
				rp, err := profile.Resolve("kind-test", p, a)
				require.NoError(t, err)
				require.Equal(t, kind, rp.Kind())

				ws, err := tr.Render(rp)
				require.NoError(t, err)
				lines := strings.Split(string(ws.Content), "\n")

				assert.Contains(t, lines, fmt.Sprintf("export CC_DECK_PROFILE_COLOR='%s'", rp.Color))

				// Skip the shebang, marker and CC_DECK_PROFILE* exports.
				i := 2
				for i < len(lines) && strings.HasPrefix(lines[i], "export CC_DECK_PROFILE") {
					i++
				}
				want := expectedUnset(tr.AuthVars(), kind)
				var unsetLines []string
				for _, l := range lines {
					if strings.HasPrefix(l, "unset ") {
						unsetLines = append(unsetLines, l)
					}
				}
				if len(want) == 0 {
					assert.Empty(t, unsetLines, "no unset line expected")
					return
				}
				require.Len(t, unsetLines, 1, "exactly one unset line expected")
				assert.Equal(t, "unset "+strings.Join(want, " "), lines[i],
					"unset line must follow the CC_DECK_PROFILE exports")
			})
		}
	}
}

// TestTranslatorContractOAuthToken verifies contract behavior 8: an
// OAuthToken profile renders for claude and is rejected by every translator
// that does not support it.
func TestTranslatorContractOAuthToken(t *testing.T) {
	t.Setenv("CONTRACT_TEST_TOKEN", "TOKEN_SECRET_VALUE_67890")
	for harness, tr := range profile.All() {
		t.Run(harness, func(t *testing.T) {
			a := agent.Get(harness)
			require.NotNil(t, a)

			if tr.SupportsOAuthToken() {
				p, _ := kindProfile(tr, profile.AuthOAuthToken)
				rp, err := profile.Resolve("token-test", p, a)
				require.NoError(t, err)
				ws, err := tr.Render(rp)
				require.NoError(t, err)
				assert.NotContains(t, string(ws.Content), "TOKEN_SECRET_VALUE_67890")
				return
			}

			// Resolve rejects the token up front.
			p, _ := kindProfile(tr, profile.AuthAPIKey)
			tokenProfile := p
			tokenProfile.Auth = &config.AuthConfig{OAuthToken: &config.CredentialSource{Env: "CONTRACT_TEST_TOKEN"}}
			_, err := profile.Resolve("token-test", tokenProfile, a)
			require.Error(t, err)

			// Render rejects it too when handed a token profile directly.
			rp, err := profile.Resolve("token-test", p, a)
			require.NoError(t, err)
			rp.APIKey = nil
			rp.OAuthToken = &config.CredentialSource{Env: "CONTRACT_TEST_TOKEN"}
			_, err = tr.Render(rp)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "oauth_token")
		})
	}
}

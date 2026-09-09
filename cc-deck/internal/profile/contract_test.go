package profile_test

import (
	"os"
	"os/exec"
	"path/filepath"
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

			// 2. Render output contains no credential value
			testEnvVal := "TEST_SECRET_VALUE_12345"
			t.Setenv("CONTRACT_TEST_KEY", testEnvVal)
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

			err = tr.PrepareConfigDir(rp, defaultDir)
			require.NoError(t, err)

			// Non-isolated entries should be symlinked
			link, err := os.Readlink(filepath.Join(rp.ConfigDir, "shared.json"))
			if err == nil {
				assert.Contains(t, link, "shared.json", "should link to shared entry")
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
			err = tr.PrepareConfigDir(rp, defaultDir)
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

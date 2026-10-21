package profile_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/profile"
	"github.com/stretchr/testify/require"
)

func TestCodexDaemonIsolation(t *testing.T) {
	for _, remote := range []bool{false, true} {
		mode := "local"
		if remote {
			mode = "remote"
		}
		for _, initial := range []string{"absent", "old-link", "directory"} {
			t.Run(mode+"/"+initial, func(t *testing.T) {
				home := setupTestEnv(t)
				t.Setenv("DAEMON_TEST_KEY", "test-key")
				defaultDir := filepath.Join(home, ".codex")
				configDir := filepath.Join(home, ".local/share/cc-deck/profiles/private/codex")
				defaultDaemon := filepath.Join(defaultDir, "app-server-daemon")
				daemonDir := filepath.Join(configDir, "app-server-daemon")
				require.NoError(t, os.MkdirAll(defaultDaemon, 0700))
				require.NoError(t, os.WriteFile(filepath.Join(defaultDaemon, "state"), []byte("default state"), 0600))
				require.NoError(t, os.WriteFile(filepath.Join(defaultDir, "shared.json"), []byte("{}"), 0600))
				require.NoError(t, os.MkdirAll(configDir, 0700))
				switch initial {
				case "old-link":
					target := defaultDaemon
					if !remote {
						var err error
						target, err = filepath.Rel(configDir, defaultDaemon)
						require.NoError(t, err)
					}
					require.NoError(t, os.Symlink(target, daemonDir))
				case "directory":
					require.NoError(t, os.MkdirAll(daemonDir, 0700))
					require.NoError(t, os.WriteFile(filepath.Join(daemonDir, "state"), []byte("private state"), 0600))
				}

				prepare := func() {
					if !remote {
						tr, ok := profile.Lookup("codex")
						require.True(t, ok)
						_, err := tr.PrepareConfigDir(profile.ResolvedProfile{Name: "private", ConfigDir: configDir}, defaultDir)
						require.NoError(t, err)
						return
					}
					cfg := &config.Config{Profiles: config.ProfileGroupsFromFlat(map[string]config.Profile{
						"private": {Harness: "codex", Auth: &config.AuthConfig{APIKey: &config.CredentialSource{Env: "DAEMON_TEST_KEY"}}},
					})}
					tgt := &memTarget{home: home, agents: []string{"codex"}}
					_, err := profile.Provision(cfg, tgt)
					require.NoError(t, err)
					prep := tgt.findUpload("prepare.sh")
					require.NotNil(t, prep)
					script := filepath.Join(t.TempDir(), "prepare.sh")
					require.NoError(t, os.WriteFile(script, prep.content, 0700))
					out, err := exec.Command("sh", script).CombinedOutput()
					require.NoError(t, err, "prepare.sh failed: %s", out)
				}

				prepare()
				if initial != "directory" {
					_, err := os.Lstat(daemonDir)
					require.True(t, os.IsNotExist(err), "daemon path should be absent, got: %v", err)
					// Simulate Codex creating its own daemon state after migration.
					require.NoError(t, os.Mkdir(daemonDir, 0700))
					require.NoError(t, os.WriteFile(filepath.Join(daemonDir, "state"), []byte("private state"), 0600))
				}
				prepare()
				info, err := os.Lstat(daemonDir)
				require.NoError(t, err)
				require.True(t, info.IsDir(), "daemon state must be a real directory")
				data, err := os.ReadFile(filepath.Join(daemonDir, "state"))
				require.NoError(t, err)
				require.Equal(t, "private state", string(data))
				data, err = os.ReadFile(filepath.Join(defaultDaemon, "state"))
				require.NoError(t, err)
				require.Equal(t, "default state", string(data))
				_, err = os.Readlink(filepath.Join(configDir, "shared.json"))
				require.NoError(t, err, "non-isolated configuration stays shared")
			})
		}
	}
}

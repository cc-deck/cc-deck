package profile_test

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/profile"
)

// memTarget implements profile.Target with in-memory recording.
type memTarget struct {
	home    string
	agents  []string
	uploads []memUpload
	cmds    []string
	// runErrors maps a substring to an error; if a command contains the
	// substring, Run returns that error.
	runErrors map[string]error
}

type memUpload struct {
	path    string
	content []byte
	mode    os.FileMode
}

func (m *memTarget) Upload(path string, content []byte, mode os.FileMode) error {
	m.uploads = append(m.uploads, memUpload{path, content, mode})
	return nil
}

func (m *memTarget) Run(cmd string) (string, error) {
	m.cmds = append(m.cmds, cmd)
	for sub, err := range m.runErrors {
		if strings.Contains(cmd, sub) {
			return "", err
		}
	}
	return "", nil
}

func (m *memTarget) Home() string      { return m.home }
func (m *memTarget) Agents() []string  { return m.agents }

func (m *memTarget) findUpload(suffix string) *memUpload {
	for i := range m.uploads {
		if strings.HasSuffix(m.uploads[i].path, suffix) {
			return &m.uploads[i]
		}
	}
	return nil
}

// ---------- Tests ----------

func TestProvision_CorrectWrapperSet(t *testing.T) {
	home := setupTestEnv(t)

	t.Setenv("WORK_KEY", "secret")
	t.Setenv("TEAM_KEY", "secret2")

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"work": {
				Harness: "claude",
				Auth:    &config.AuthConfig{APIKey: &config.CredentialSource{Env: "WORK_KEY"}},
			},
			"team": {
				Harness: "codex",
				Auth:    &config.AuthConfig{APIKey: &config.CredentialSource{Env: "TEAM_KEY"}},
			},
		},
	}

	tgt := &memTarget{
		home:   home,
		agents: []string{"claude", "codex"},
	}

	result, err := profile.Provision(cfg, tgt)
	require.NoError(t, err)

	assert.Len(t, result.Written, 2, "both wrappers must be written")
	assert.Contains(t, result.Written, "claude-work")
	assert.Contains(t, result.Written, "codex-team")

	// Wrappers must be uploaded with executable mode.
	workUp := tgt.findUpload("claude-work")
	require.NotNil(t, workUp, "claude-work wrapper must be uploaded")
	assert.Equal(t, os.FileMode(0o755), workUp.mode)

	teamUp := tgt.findUpload("codex-team")
	require.NotNil(t, teamUp, "codex-team wrapper must be uploaded")
	assert.Equal(t, os.FileMode(0o755), teamUp.mode)

	// prepare.sh must be uploaded.
	prepUp := tgt.findUpload("prepare.sh")
	require.NotNil(t, prepUp, "prepare.sh must be uploaded")
}

func TestProvision_PrepareShContent(t *testing.T) {
	home := setupTestEnv(t)

	t.Setenv("WORK_KEY", "secret")

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"work": {
				Harness: "claude",
				Auth:    &config.AuthConfig{APIKey: &config.CredentialSource{Env: "WORK_KEY"}},
			},
		},
	}

	tgt := &memTarget{
		home:   home,
		agents: []string{"claude"},
	}

	_, err := profile.Provision(cfg, tgt)
	require.NoError(t, err)

	prepUp := tgt.findUpload("prepare.sh")
	require.NotNil(t, prepUp, "prepare.sh must be uploaded")

	content := string(prepUp.content)
	assert.True(t, strings.HasPrefix(content, "#!/bin/sh\n"), "must start with shebang")
	assert.Contains(t, content, "mkdir -p", "must create directories")
	assert.Contains(t, content, "ln -sfn", "must create symlinks")
	assert.Contains(t, content, ".local/share/cc-deck/bin", "must reference bin dir")
	assert.Contains(t, content, "profiles/work/claude", "must reference profile config dir")
}

func TestProvision_RCBlockAppendedOnce(t *testing.T) {
	home := setupTestEnv(t)

	t.Setenv("WORK_KEY", "secret")

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"work": {
				Harness: "claude",
				Auth:    &config.AuthConfig{APIKey: &config.CredentialSource{Env: "WORK_KEY"}},
			},
		},
	}

	tgt := &memTarget{
		home:   home,
		agents: []string{"claude"},
	}

	result, err := profile.Provision(cfg, tgt)
	require.NoError(t, err)
	assert.True(t, result.RCChanged, "RCChanged must be true")

	// Check that both .bashrc and .zshrc are addressed in the commands.
	bashrcFound := false
	zshrcFound := false
	for _, cmd := range tgt.cmds {
		if strings.Contains(cmd, ".bashrc") && strings.Contains(cmd, "cc-deck") {
			bashrcFound = true
		}
		if strings.Contains(cmd, ".zshrc") && strings.Contains(cmd, "cc-deck") {
			zshrcFound = true
		}
	}
	assert.True(t, bashrcFound, "must update .bashrc")
	assert.True(t, zshrcFound, "must update .zshrc")
}

func TestProvision_MissingHarnessSkipped(t *testing.T) {
	setupTestEnv(t)

	t.Setenv("TEAM_KEY", "secret")

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"team": {
				Harness: "codex",
				Auth:    &config.AuthConfig{APIKey: &config.CredentialSource{Env: "TEAM_KEY"}},
			},
		},
	}

	// Target only has "claude", not "codex".
	tgt := &memTarget{
		home:   "/home/testuser",
		agents: []string{"claude"},
	}

	result, err := profile.Provision(cfg, tgt)
	require.NoError(t, err)

	assert.Empty(t, result.Written, "no wrappers should be written")
	require.Len(t, result.Skipped, 1)
	assert.Equal(t, "team", result.Skipped[0].Profile)
	assert.Contains(t, result.Skipped[0].Reason, "not in target agent list")
}

func TestProvision_StaleWrapperRemoval(t *testing.T) {
	home := setupTestEnv(t)

	t.Setenv("WORK_KEY", "secret")

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"work": {
				Harness: "claude",
				Auth:    &config.AuthConfig{APIKey: &config.CredentialSource{Env: "WORK_KEY"}},
			},
		},
	}

	tgt := &memTarget{
		home:   home,
		agents: []string{"claude"},
	}

	result, err := profile.Provision(cfg, tgt)
	require.NoError(t, err)
	assert.Contains(t, result.Written, "claude-work")

	// Verify prepare.sh contains stale wrapper removal logic.
	prepUp := tgt.findUpload("prepare.sh")
	require.NotNil(t, prepUp, "prepare.sh must be uploaded")

	content := string(prepUp.content)
	assert.Contains(t, content, "Remove stale cc-deck wrappers",
		"prepare.sh must contain stale wrapper removal comment")
	assert.Contains(t, content, "generated by cc-deck profile",
		"prepare.sh must check for cc-deck marker")
	// The known wrapper name must appear in the skip case so it is not
	// removed by the stale-cleanup loop.
	assert.Contains(t, content, "claude-work",
		"prepare.sh must skip the current wrapper name")
}

func TestProvision_RemoteBinaryNotFound(t *testing.T) {
	home := setupTestEnv(t)

	t.Setenv("WORK_KEY", "secret")
	t.Setenv("TEAM_KEY", "secret2")

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"work": {
				Harness: "claude",
				Auth:    &config.AuthConfig{APIKey: &config.CredentialSource{Env: "WORK_KEY"}},
			},
			"team": {
				Harness: "codex",
				Auth:    &config.AuthConfig{APIKey: &config.CredentialSource{Env: "TEAM_KEY"}},
			},
		},
	}

	tgt := &memTarget{
		home:   home,
		agents: []string{"claude", "codex"},
		runErrors: map[string]error{
			"command -v codex": fmt.Errorf("codex: not found"),
		},
	}

	result, err := profile.Provision(cfg, tgt)
	require.NoError(t, err)

	// Claude profile should succeed, codex should be skipped.
	assert.Contains(t, result.Written, "claude-work")

	skippedNames := make([]string, len(result.Skipped))
	for i, s := range result.Skipped {
		skippedNames[i] = s.Profile
	}
	assert.Contains(t, skippedNames, "team")

	for _, s := range result.Skipped {
		if s.Profile == "team" {
			assert.Contains(t, s.Reason, "not found on remote")
		}
	}
}

func TestProvision_OAuthTokenFileUploaded(t *testing.T) {
	home := setupTestEnv(t)
	tokenValue := "sk-ant-oat01-PROVISION-TOKEN"
	tokenFile := home + "/token"
	require.NoError(t, os.WriteFile(tokenFile, []byte(tokenValue), 0o600))

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"private": {
				Harness: "claude",
				Auth:    &config.AuthConfig{OAuthToken: &config.CredentialSource{File: tokenFile}},
			},
		},
	}
	target := &memTarget{home: "/home/remote", agents: []string{"claude"}}

	result, err := profile.Provision(cfg, target)
	require.NoError(t, err)
	assert.Contains(t, result.Written, "claude-private")

	up := target.findUpload("/home/remote/.config/cc-deck/profiles/private/oauth_token")
	require.NotNil(t, up, "token file must be uploaded to the profile credential dir")
	assert.Equal(t, os.FileMode(0o600), up.mode)
	assert.Equal(t, tokenValue, string(up.content))

	wrapper := target.findUpload("/home/remote/.local/share/cc-deck/bin/claude-private")
	require.NotNil(t, wrapper)
	assert.NotContains(t, string(wrapper.content), tokenValue, "wrapper must not contain the token value")
	assert.Contains(t, string(wrapper.content), `_f_oauthtoken="$HOME/.config/cc-deck/profiles/private/oauth_token"`)
}

func TestEnvCredentials_IncludesTokenEnvForPresentHarness(t *testing.T) {
	t.Setenv("CLAUDE_TOKEN_PRIVATE", "sk-ant-oat01-ENV")
	t.Setenv("OPENAI_API_KEY_TEAM", "sk-team")

	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"private": {
				Harness: "claude",
				Auth:    &config.AuthConfig{OAuthToken: &config.CredentialSource{Env: "CLAUDE_TOKEN_PRIVATE"}},
			},
			"team": {
				Harness: "codex",
				Auth:    &config.AuthConfig{APIKey: &config.CredentialSource{Env: "OPENAI_API_KEY_TEAM"}},
			},
		},
	}

	env := profile.EnvCredentials(cfg, []string{"claude"})
	assert.Equal(t, map[string]string{"CLAUDE_TOKEN_PRIVATE": "sk-ant-oat01-ENV"}, env,
		"only profiles whose harness is present on the target contribute")

	env = profile.EnvCredentials(cfg, []string{"claude", "codex"})
	assert.Len(t, env, 2)
}

func TestProvision_PrepareShUnlinksIsolatedEntries(t *testing.T) {
	home := setupTestEnv(t)
	t.Setenv("WORK_KEY", "secret")
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not available")
	}

	defaultDir := home + "/.claude"
	require.NoError(t, os.MkdirAll(defaultDir+"/backups", 0o755))
	require.NoError(t, os.WriteFile(defaultDir+"/settings.json", []byte("{}"), 0o644))
	configDir := home + "/.local/share/cc-deck/profiles/work/claude"
	require.NoError(t, os.MkdirAll(configDir, 0o700))
	// Link left behind by an earlier provisioning that still shared backups.
	require.NoError(t, os.Symlink(defaultDir+"/backups", configDir+"/backups"))

	cfg := &config.Config{Profiles: map[string]config.Profile{
		"work": {Harness: "claude", Auth: &config.AuthConfig{APIKey: &config.CredentialSource{Env: "WORK_KEY"}}},
	}}
	tgt := &memTarget{home: home, agents: []string{"claude"}}
	_, err = profile.Provision(cfg, tgt)
	require.NoError(t, err)
	prep := tgt.findUpload("prepare.sh")
	require.NotNil(t, prep)

	script := t.TempDir() + "/prepare.sh"
	require.NoError(t, os.WriteFile(script, prep.content, 0o755))
	out, err := exec.Command(sh, script).CombinedOutput()
	require.NoError(t, err, "prepare.sh failed: %s", out)

	_, err = os.Lstat(configDir + "/backups")
	assert.True(t, os.IsNotExist(err), "stale backups link removed")
	_, err = os.Stat(defaultDir + "/backups")
	assert.NoError(t, err, "link target untouched")
	_, err = os.Readlink(configDir + "/settings.json")
	assert.NoError(t, err, "shared entries still linked")
}

package ws

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/credential"
)

func tokenProfileConfig() *config.Config {
	return &config.Config{Profiles: config.ProfileGroupsFromFlat(map[string]config.Profile{
		"private": {
			Harness: "claude",
			Auth:    &config.AuthConfig{OAuthToken: &config.CredentialSource{Env: "CLAUDE_TOKEN_PRIVATE"}},
		},
	})}
}

func TestSSHCredentialSet_ProfileTokenWithoutDetectedModes(t *testing.T) {
	// SC-009: a user with only a subscription token profile still gets the
	// token into credentials.env.
	t.Setenv("CLAUDE_TOKEN_PRIVATE", "sk-ant-oat01-ssh")

	got, err := sshCredentialSet(nil, tokenProfileConfig(), []string{"claude"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"CLAUDE_TOKEN_PRIVATE": "sk-ant-oat01-ssh"}, got.EnvVars)
}

func TestSSHCredentialSet_MergesDetectedAndProfileCredentials(t *testing.T) {
	t.Setenv("CLAUDE_TOKEN_PRIVATE", "sk-ant-oat01-ssh")
	modes := []credential.DetectedMode{{
		AgentName: "claude",
		Resolved:  credential.ResolvedCredentials{EnvVars: map[string]string{"ANTHROPIC_API_KEY": "sk-detected"}},
	}}

	got, err := sshCredentialSet(modes, tokenProfileConfig(), []string{"claude"})
	require.NoError(t, err)
	assert.Equal(t, "sk-detected", got.EnvVars["ANTHROPIC_API_KEY"])
	assert.Equal(t, "sk-ant-oat01-ssh", got.EnvVars["CLAUDE_TOKEN_PRIVATE"])
}

func TestSSHCredentialSet_SkipsProfilesForAbsentHarness(t *testing.T) {
	t.Setenv("CLAUDE_TOKEN_PRIVATE", "sk-ant-oat01-ssh")

	got, err := sshCredentialSet(nil, tokenProfileConfig(), []string{"codex"})
	require.NoError(t, err)
	assert.Empty(t, got.EnvVars)
}

func TestSSHCredentialSet_NilConfigAndNoModesIsEmpty(t *testing.T) {
	got, err := sshCredentialSet(nil, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, got.EnvVars)
	assert.Nil(t, got.FileCredential)
}

func TestSSHCredentialSet_MergeConflictReturnsError(t *testing.T) {
	modes := []credential.DetectedMode{
		{AgentName: "a", Resolved: credential.ResolvedCredentials{EnvVars: map[string]string{"K": "1"}}},
		{AgentName: "b", Resolved: credential.ResolvedCredentials{EnvVars: map[string]string{"K": "2"}}},
	}
	_, err := sshCredentialSet(modes, tokenProfileConfig(), []string{"claude"})
	require.Error(t, err, "a merge conflict must stop injection instead of writing a partial set")
}

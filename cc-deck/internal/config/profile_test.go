package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// workedExample is the YAML from contracts/profile-schema.md.
const workedExample = `
profiles:
  claude:
    work:
      backend: vertex
      project: acme-ml
      region: us-east5
      model: claude-sonnet-5
      auth:
        credentials: {file: ~/.config/gcloud/acme-adc.json}
      color: "#5AC0E2"
    private:
      model: claude-opus-5
      auth:
        oauth_token: {env: CLAUDE_TOKEN_PRIVATE}
      color: "#E27C5A"
  codex:
    team:
      model: gpt-5
      auth:
        api_key: {env: OPENAI_API_KEY_TEAM}
`

func TestProfileYAMLRoundTrip(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(workedExample), &cfg))

	assert.Len(t, cfg.ListProfileKeys(), 3)

	work := cfg.Profiles["claude"]["work"]
	assert.Equal(t, "claude", work.HarnessName())
	assert.Equal(t, BackendVertex, work.EffectiveBackend())
	assert.Equal(t, "acme-ml", work.Project)
	assert.Equal(t, "us-east5", work.Region)
	assert.Equal(t, "claude-sonnet-5", work.Model)
	assert.Equal(t, "#5AC0E2", work.Color)
	require.NotNil(t, work.Auth)
	require.NotNil(t, work.Auth.Credentials)
	assert.Equal(t, "~/.config/gcloud/acme-adc.json", work.Auth.Credentials.File)

	priv := cfg.Profiles["claude"]["private"]
	assert.Equal(t, "claude", priv.HarnessName())
	assert.Equal(t, BackendAnthropic, priv.EffectiveBackend())
	require.NotNil(t, priv.Auth)
	require.NotNil(t, priv.Auth.OAuthToken)
	assert.Equal(t, "CLAUDE_TOKEN_PRIVATE", priv.Auth.OAuthToken.Env)
	assert.False(t, priv.Auth.Login)
	assert.Equal(t, "claude-opus-5", priv.Model)
	require.NotNil(t, priv.EffectiveAuth().OAuthToken)
	assert.NoError(t, priv.Validate())

	team := cfg.Profiles["codex"]["team"]
	assert.Equal(t, "codex", team.HarnessName())
	assert.Equal(t, BackendOpenAI, team.EffectiveBackend())
	require.NotNil(t, team.Auth)
	require.NotNil(t, team.Auth.APIKey)
	assert.Equal(t, "OPENAI_API_KEY_TEAM", team.Auth.APIKey.Env)

	// Round-trip: marshal and unmarshal again
	out, err := yaml.Marshal(&cfg)
	require.NoError(t, err)
	var cfg2 Config
	require.NoError(t, yaml.Unmarshal(out, &cfg2))
	assert.Equal(t, len(cfg.ListProfileKeys()), len(cfg2.ListProfileKeys()))
	assert.Equal(t, cfg.Profiles["claude"]["private"], cfg2.Profiles["claude"]["private"])
	assert.Empty(t, validateProfiles(cfg2.Profiles, ""))
}

func TestHarnessScopedSelectors(t *testing.T) {
	cfg := &Config{}
	claude := Profile{Auth: &AuthConfig{Login: true}}
	// Codex login is currently unsupported, so use an API key source.
	codex := Profile{Auth: &AuthConfig{APIKey: &CredentialSource{Env: "CODEX_KEY"}}}
	require.NoError(t, cfg.AddProfile(ProfileKey{"claude", "private"}, claude))
	require.NoError(t, cfg.SetDefaultProfile("private"))
	assert.Equal(t, "claude/private", cfg.DefaultProfile)
	require.NoError(t, cfg.AddProfile(ProfileKey{"codex", "private"}, codex))
	_, err := cfg.ResolveProfileRef("private")
	require.ErrorContains(t, err, "ambiguous")
	assert.ErrorContains(t, err, "claude/private")
	assert.ErrorContains(t, err, "codex/private")
	assert.Equal(t, "claude/private", cfg.DefaultProfile)
	key, err := cfg.ResolveProfileRef("codex/private")
	require.NoError(t, err)
	assert.Equal(t, ProfileKey{"codex", "private"}, key)
	require.Error(t, cfg.AddProfile(key, codex))
	require.NoError(t, cfg.DeleteProfile(ProfileKey{"claude", "private"}))
	_, err = cfg.GetProfileFor("codex", "private")
	require.NoError(t, err)
	assert.Empty(t, cfg.DefaultProfile)
}

func TestHarnessScopedYAMLStrictness(t *testing.T) {
	for name, input := range map[string]string{
		"legacy":                  "profiles:\n  private:\n    auth: {login: true}\n",
		"duplicate harness":       "profiles:\n  claude: {}\n  claude: {}\n",
		"duplicate name":          "profiles:\n  claude:\n    private: {auth: {login: true}}\n    private: {auth: {login: true}}\n",
		"inner harness":           "profiles:\n  claude:\n    private: {harness: claude, auth: {login: true}}\n",
		"unknown harness":         "profiles:\n  unknown:\n    private: {}\n",
		"unknown profile field":   "profiles:\n  claude:\n    private: {auth: {login: true}, modle: typo}\n",
		"duplicate profile field": "profiles:\n  claude:\n    private: {model: one, model: two}\n",
	} {
		t.Run(name, func(t *testing.T) {
			var cfg Config
			require.Error(t, yaml.Unmarshal([]byte(input), &cfg))
		})
	}
}

func TestUnknownNestedProfileFieldsAreRejected(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
	}{
		{"profile", "modle: typo", `unknown field "modle"`},
		{"auth", "auth: {api_keey: {env: KEY}}", `unknown field "auth.api_keey"`},
		{"credential source", "auth: {api_key: {enV: KEY}}", `unknown field "auth.api_key.enV"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config
			err := yaml.Unmarshal([]byte("profiles:\n  claude:\n    private: {"+tc.input+"}\n"), &cfg)
			require.Error(t, err)
			assert.ErrorContains(t, err, `profile "claude/private": `+tc.want)
		})
	}
}

func TestProfileEnvAllowsArbitraryKeys(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte("profiles:\n  claude:\n    private:\n      auth: {login: true}\n      env: {modle: preserved}\n"), &cfg)
	require.NoError(t, err)
	assert.Equal(t, "preserved", cfg.Profiles["claude"]["private"].Env["modle"])
}

func TestMigrationFixturePreservesProfilesAndDefault(t *testing.T) {
	legacy := map[string]Profile{
		"private": {Harness: "claude", Model: "claude-opus-5", Color: "#E27C5A", Auth: &AuthConfig{OAuthToken: &CredentialSource{File: "~/token"}}},
		"team":    {Harness: "codex", Model: "gpt-5", Auth: &AuthConfig{APIKey: &CredentialSource{Env: "TEAM_KEY"}}},
	}
	cfg := &Config{Profiles: ProfileGroupsFromFlat(legacy)}
	require.NoError(t, cfg.SetDefaultProfile("private"))
	assert.Equal(t, "claude/private", cfg.DefaultProfile)
	encoded, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "harness:")
	var reloaded Config
	require.NoError(t, yaml.Unmarshal(encoded, &reloaded))
	assert.Equal(t, legacy["private"].Model, reloaded.Profiles["claude"]["private"].Model)
	assert.Equal(t, legacy["private"].Auth.OAuthToken.File, reloaded.Profiles["claude"]["private"].Auth.OAuthToken.File)
	assert.Equal(t, legacy["team"].Model, reloaded.Profiles["codex"]["team"].Model)
	assert.Equal(t, "claude/private", reloaded.DefaultProfile)
}

func TestEffectiveAuth_Legacy(t *testing.T) {
	// Legacy anthropic profile with api_key_secret
	p := Profile{
		Backend:      BackendAnthropic,
		APIKeySecret: "my-secret",
	}
	auth := p.EffectiveAuth()
	require.NotNil(t, auth.APIKey)
	assert.Equal(t, SourceSecret, auth.APIKey.Kind())
	assert.Equal(t, "my-secret", auth.APIKey.Secret)

	// Legacy vertex profile with credentials_secret
	p2 := Profile{
		Backend:           BackendVertex,
		CredentialsSecret: "cred-secret",
		Project:           "p",
		Region:            "r",
	}
	auth2 := p2.EffectiveAuth()
	require.NotNil(t, auth2.Credentials)
	assert.Equal(t, SourceSecret, auth2.Credentials.Kind())
	assert.Equal(t, "cred-secret", auth2.Credentials.Secret)
}

func TestEffectiveAuth_NewStyle(t *testing.T) {
	p := Profile{
		Harness: "claude",
		Auth: &AuthConfig{
			APIKey: &CredentialSource{Env: "MY_KEY"},
		},
	}
	auth := p.EffectiveAuth()
	require.NotNil(t, auth.APIKey)
	assert.Equal(t, SourceEnv, auth.APIKey.Kind())
	assert.Equal(t, "MY_KEY", auth.APIKey.Env)
}

func TestEffectiveAuth_NewDoesNotOverrideLegacy(t *testing.T) {
	// When both Auth and legacy are set, Auth takes precedence
	p := Profile{
		APIKeySecret: "legacy-secret",
		Auth: &AuthConfig{
			APIKey: &CredentialSource{Env: "NEW_KEY"},
		},
	}
	auth := p.EffectiveAuth()
	require.NotNil(t, auth.APIKey)
	assert.Equal(t, SourceEnv, auth.APIKey.Kind())
	assert.Equal(t, "NEW_KEY", auth.APIKey.Env)
}

func TestCredentialSourceKind(t *testing.T) {
	tests := []struct {
		name string
		cs   *CredentialSource
		want SourceKind
	}{
		{"nil", nil, SourceNone},
		{"empty", &CredentialSource{}, SourceNone},
		{"env", &CredentialSource{Env: "MY_VAR"}, SourceEnv},
		{"file", &CredentialSource{File: "/path"}, SourceFile},
		{"secret", &CredentialSource{Secret: "s"}, SourceSecret},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cs.Kind())
		})
	}
}

func TestHarnessName_Default(t *testing.T) {
	p := Profile{}
	assert.Equal(t, "claude", p.HarnessName())
}

func TestEffectiveBackend_Defaults(t *testing.T) {
	tests := []struct {
		harness string
		want    BackendType
	}{
		{"claude", BackendAnthropic},
		{"codex", BackendOpenAI},
		{"opencode", BackendOpenAI},
		{"", BackendAnthropic},
	}
	for _, tt := range tests {
		t.Run(tt.harness, func(t *testing.T) {
			p := Profile{Harness: tt.harness}
			assert.Equal(t, tt.want, p.EffectiveBackend())
		})
	}
}

func TestWrapperName(t *testing.T) {
	assert.Equal(t, "claude-work", WrapperName("claude", "work"))
	assert.Equal(t, "codex-team", WrapperName("codex", "team"))
}

func TestSaveOmitsUnsetNewFields(t *testing.T) {
	// A legacy profile should marshal without new fields
	p := Profile{
		Backend:      BackendAnthropic,
		APIKeySecret: "s",
	}
	out, err := yaml.Marshal(p)
	require.NoError(t, err)
	s := string(out)
	assert.NotContains(t, s, "harness:")
	assert.NotContains(t, s, "auth:")
	assert.NotContains(t, s, "env:")
	assert.NotContains(t, s, "color:")
	assert.NotContains(t, s, "icon:")

	// A new-style profile without a token omits oauth_token.
	p2 := Profile{Harness: "claude", Auth: &AuthConfig{APIKey: &CredentialSource{Env: "K"}}}
	out, err = yaml.Marshal(p2)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "oauth_token")
}

func TestValidate_OAuthTokenSatisfiesAnthropic(t *testing.T) {
	p := Profile{Harness: "claude", Auth: &AuthConfig{OAuthToken: &CredentialSource{File: "~/.secrets/tok"}}}
	assert.NoError(t, p.Validate())

	empty := Profile{Harness: "claude", Auth: &AuthConfig{}}
	err := empty.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth.api_key, auth.oauth_token or auth.login")
}

func TestPreFeatureFixtureRequiresMigration(t *testing.T) {
	cfgYAML := `
default_profile: prod
profiles:
  prod:
    backend: anthropic
    api_key_secret: my-secret
  staging:
    backend: vertex
    project: my-project
    region: us-central1
`
	var cfg Config
	err := yaml.Unmarshal([]byte(cfgYAML), &cfg)
	require.ErrorContains(t, err, "expected profiles.<harness>.<name>")
}

func TestValidate_LegacyProfile(t *testing.T) {
	p := Profile{
		Backend:      BackendAnthropic,
		APIKeySecret: "s",
	}
	assert.NoError(t, p.Validate())

	p2 := Profile{
		Backend: BackendVertex,
		Project: "p",
		Region:  "r",
	}
	assert.NoError(t, p2.Validate())
}

func TestValidate_NewStyleProfile(t *testing.T) {
	p := Profile{
		Harness: "claude",
		Auth: &AuthConfig{
			APIKey: &CredentialSource{Env: "KEY"},
		},
	}
	assert.NoError(t, p.Validate())

	// Login profile
	p2 := Profile{
		Harness: "claude",
		Auth:    &AuthConfig{Login: true},
	}
	assert.NoError(t, p2.Validate())

	// Codex profile
	p3 := Profile{
		Harness: "codex",
		Auth: &AuthConfig{
			APIKey: &CredentialSource{Env: "KEY"},
		},
	}
	assert.NoError(t, p3.Validate())
}

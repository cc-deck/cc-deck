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
  work:
    harness: claude
    backend: vertex
    project: acme-ml
    region: us-east5
    model: claude-sonnet-5
    auth:
      credentials: {file: ~/.config/gcloud/acme-adc.json}
    color: "#4FC1E9"
  private:
    harness: claude
    model: claude-opus-5
    auth:
      login: true
    color: "#EC87C0"
  team:
    harness: codex
    model: gpt-5
    auth:
      api_key: {env: OPENAI_API_KEY_TEAM}
`

func TestProfileYAMLRoundTrip(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(workedExample), &cfg))

	assert.Len(t, cfg.Profiles, 3)

	work := cfg.Profiles["work"]
	assert.Equal(t, "claude", work.HarnessName())
	assert.Equal(t, BackendVertex, work.EffectiveBackend())
	assert.Equal(t, "acme-ml", work.Project)
	assert.Equal(t, "us-east5", work.Region)
	assert.Equal(t, "claude-sonnet-5", work.Model)
	assert.Equal(t, "#4FC1E9", work.Color)
	require.NotNil(t, work.Auth)
	require.NotNil(t, work.Auth.Credentials)
	assert.Equal(t, "~/.config/gcloud/acme-adc.json", work.Auth.Credentials.File)

	priv := cfg.Profiles["private"]
	assert.Equal(t, "claude", priv.HarnessName())
	assert.Equal(t, BackendAnthropic, priv.EffectiveBackend())
	require.NotNil(t, priv.Auth)
	assert.True(t, priv.Auth.Login)
	assert.Equal(t, "claude-opus-5", priv.Model)

	team := cfg.Profiles["team"]
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
	assert.Equal(t, len(cfg.Profiles), len(cfg2.Profiles))
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
	p := Profile{Harness: "claude"}
	assert.Equal(t, "claude-work", p.WrapperName("claude", "work"))
	assert.Equal(t, "codex-team", p.WrapperName("codex", "team"))
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
}

func TestPreFeatureFixtureLoadsWithoutFindings(t *testing.T) {
	// SC-005: pre-feature config loads and validates without findings
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
	require.NoError(t, yaml.Unmarshal([]byte(cfgYAML), &cfg))
	findings := cfg.Validate()
	for _, f := range findings {
		if f.Category == CategoryProfiles && f.Severity == SeverityError {
			t.Errorf("unexpected error finding: %s", f.Message)
		}
	}
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

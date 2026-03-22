package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestGenerate_WithProxy(t *testing.T) {
	out, err := Generate(GenerateOptions{
		SessionName: "test-session",
		ImageRef:    "quay.io/cc-deck/demo:latest",
		Domains:     []string{"pypi.org", ".github.com", "api.anthropic.com"},
	})
	require.NoError(t, err)

	// Verify compose YAML structure
	var cf composeFile
	require.NoError(t, yaml.Unmarshal([]byte(out.ComposeYAML), &cf))

	// Session container
	session, ok := cf.Services["session"]
	require.True(t, ok)
	assert.Equal(t, "quay.io/cc-deck/demo:latest", session.Image)
	assert.Equal(t, []string{"internal"}, session.Networks)
	assert.Contains(t, session.Environment, "HTTP_PROXY")
	assert.Contains(t, session.Environment, "HTTPS_PROXY")
	assert.Equal(t, []string{"proxy"}, session.DependsOn)

	// Proxy container
	proxy, ok := cf.Services["proxy"]
	require.True(t, ok)
	assert.Contains(t, proxy.Networks, "internal")
	assert.Contains(t, proxy.Networks, "default")
	assert.NotEmpty(t, proxy.Volumes)

	// Internal network
	internal, ok := cf.Networks["internal"]
	require.True(t, ok)
	assert.True(t, internal.Internal)

	// Proxy config files
	assert.NotEmpty(t, out.TinyproxyConf)
	assert.Contains(t, out.TinyproxyConf, "FilterDefaultDeny Yes")
	assert.NotEmpty(t, out.Whitelist)
	assert.Contains(t, out.Whitelist, `pypi\.org$`)
	assert.Contains(t, out.Whitelist, `github\.com$`)

	// Env example
	assert.NotEmpty(t, out.EnvExample)
	assert.Contains(t, out.EnvExample, "ANTHROPIC_API_KEY")
}

func TestGenerate_WithoutProxy(t *testing.T) {
	out, err := Generate(GenerateOptions{
		SessionName: "test-session",
		ImageRef:    "quay.io/cc-deck/demo:latest",
		Domains:     nil,
	})
	require.NoError(t, err)

	var cf composeFile
	require.NoError(t, yaml.Unmarshal([]byte(out.ComposeYAML), &cf))

	// No proxy container
	_, hasProxy := cf.Services["proxy"]
	assert.False(t, hasProxy)

	// No internal network
	assert.Empty(t, cf.Networks)

	// Session should not have proxy env vars
	session := cf.Services["session"]
	assert.NotContains(t, session.Environment, "HTTP_PROXY")

	// No proxy config files
	assert.Empty(t, out.TinyproxyConf)
	assert.Empty(t, out.Whitelist)
}

func TestGenerate_MissingSessionName(t *testing.T) {
	_, err := Generate(GenerateOptions{ImageRef: "img:latest"})
	assert.Error(t, err)
}

func TestGenerate_MissingImageRef(t *testing.T) {
	_, err := Generate(GenerateOptions{SessionName: "test"})
	assert.Error(t, err)
}

func TestGenerate_WithVolumes(t *testing.T) {
	out, err := Generate(GenerateOptions{
		SessionName: "test-session",
		ImageRef:    "quay.io/cc-deck/demo:latest",
		Volumes:     []string{"./..:/workspace"},
	})
	require.NoError(t, err)

	var cf composeFile
	require.NoError(t, yaml.Unmarshal([]byte(out.ComposeYAML), &cf))

	session := cf.Services["session"]
	assert.Contains(t, session.Volumes, "./..:/workspace")
	assert.True(t, session.Stdin, "stdin_open should be true")
	assert.True(t, session.TTY, "tty should be true")
}

func TestGenerate_WithCredentialVolumeMounts(t *testing.T) {
	out, err := Generate(GenerateOptions{
		SessionName: "test-session",
		ImageRef:    "quay.io/cc-deck/demo:latest",
		Volumes: []string{
			"./..:/workspace",
			"/home/user/.config/gcloud/adc.json:/run/secrets/google-application-credentials:ro",
		},
	})
	require.NoError(t, err)

	var cf composeFile
	require.NoError(t, yaml.Unmarshal([]byte(out.ComposeYAML), &cf))

	session := cf.Services["session"]
	assert.Contains(t, session.Volumes,
		"/home/user/.config/gcloud/adc.json:/run/secrets/google-application-credentials:ro")
	assert.Equal(t, "keep-id", session.UserNSMode)
}

func TestGenerate_WithPorts(t *testing.T) {
	out, err := Generate(GenerateOptions{
		SessionName: "test-session",
		ImageRef:    "quay.io/cc-deck/demo:latest",
		Ports:       []string{"8080:8080", "3000:3000"},
	})
	require.NoError(t, err)

	var cf composeFile
	require.NoError(t, yaml.Unmarshal([]byte(out.ComposeYAML), &cf))

	session := cf.Services["session"]
	assert.Equal(t, []string{"8080:8080", "3000:3000"}, session.Ports)
}

func TestGenerate_StdinAndTTY(t *testing.T) {
	out, err := Generate(GenerateOptions{
		SessionName: "s1",
		ImageRef:    "img:latest",
	})
	require.NoError(t, err)

	var cf composeFile
	require.NoError(t, yaml.Unmarshal([]byte(out.ComposeYAML), &cf))

	session := cf.Services["session"]
	assert.True(t, session.Stdin, "stdin_open should always be true")
	assert.True(t, session.TTY, "tty should always be true")
}

func TestGenerate_UserNSModeAlwaysSet(t *testing.T) {
	tests := []struct {
		name string
		opts GenerateOptions
	}{
		{
			name: "minimal",
			opts: GenerateOptions{SessionName: "s1", ImageRef: "img:latest"},
		},
		{
			name: "with volumes",
			opts: GenerateOptions{
				SessionName: "s1",
				ImageRef:    "img:latest",
				Volumes:     []string{"./..:/workspace"},
			},
		},
		{
			name: "with proxy",
			opts: GenerateOptions{
				SessionName: "s1",
				ImageRef:    "img:latest",
				Domains:     []string{"example.com"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Generate(tt.opts)
			require.NoError(t, err)

			var cf composeFile
			require.NoError(t, yaml.Unmarshal([]byte(out.ComposeYAML), &cf))

			session := cf.Services["session"]
			assert.Equal(t, "keep-id", session.UserNSMode,
				"userns_mode: keep-id must always be set on session service")
		})
	}
}

func TestGenerate_NoUFlagInVolumes(t *testing.T) {
	out, err := Generate(GenerateOptions{
		SessionName: "s1",
		ImageRef:    "img:latest",
		Volumes: []string{
			"./..:/workspace",
			"/tmp/creds.json:/run/secrets/creds:ro",
		},
	})
	require.NoError(t, err)

	var cf composeFile
	require.NoError(t, yaml.Unmarshal([]byte(out.ComposeYAML), &cf))

	for _, vol := range cf.Services["session"].Volumes {
		assert.NotContains(t, vol, ":U",
			"volume %q must not use :U flag (causes lchown failures on read-only files)", vol)
	}
}

func TestGenerate_ProxyEnvVars(t *testing.T) {
	out, err := Generate(GenerateOptions{
		SessionName: "s1",
		ImageRef:    "img:latest",
		Domains:     []string{"example.com"},
	})
	require.NoError(t, err)

	var cf composeFile
	require.NoError(t, yaml.Unmarshal([]byte(out.ComposeYAML), &cf))

	env := cf.Services["session"].Environment
	assert.Equal(t, "http://proxy:8888", env["HTTP_PROXY"])
	assert.Equal(t, "http://proxy:8888", env["HTTPS_PROXY"])
	assert.Equal(t, "http://proxy:8888", env["http_proxy"])
	assert.Equal(t, "http://proxy:8888", env["https_proxy"])
}

package openshell

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_ValidConfig(t *testing.T) {
	cfg := GatewayConfig{Address: "localhost:17670"}
	client, err := NewClient(cfg)
	require.NoError(t, err)
	assert.Equal(t, "localhost:17670", client.Address())
}

func TestNewClient_EmptyAddress(t *testing.T) {
	cfg := GatewayConfig{}
	_, err := NewClient(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gateway address is required")
}

func TestResolveGatewayConfig_FromDefinition(t *testing.T) {
	cfg := &GatewayConfig{Address: "gateway.example.com:9090", TLS: true}
	result := ResolveGatewayConfig(cfg)
	assert.Equal(t, "gateway.example.com:9090", result.Address)
	assert.True(t, result.TLS)
}

func TestResolveGatewayConfig_FromEnvVar(t *testing.T) {
	t.Setenv("OPENSHELL_GATEWAY_URL", "env-gateway:4444")
	result := ResolveGatewayConfig(nil)
	assert.Equal(t, "env-gateway:4444", result.Address)
}

func TestResolveGatewayConfig_Default(t *testing.T) {
	t.Setenv("OPENSHELL_GATEWAY_URL", "")
	result := ResolveGatewayConfig(nil)
	assert.Equal(t, "localhost:17670", result.Address)
}

func TestResolveGatewayConfig_EmptyDefinition(t *testing.T) {
	t.Setenv("OPENSHELL_GATEWAY_URL", "")
	cfg := &GatewayConfig{}
	result := ResolveGatewayConfig(cfg)
	assert.Equal(t, "localhost:17670", result.Address)
}

func TestIsLocalhost(t *testing.T) {
	tests := []struct {
		addr     string
		expected bool
	}{
		{"localhost:17670", true},
		{"127.0.0.1:8080", true},
		{"[::1]:8080", true},
		{"::1", true},
		{"localhost", true},
		{"gateway.example.com:8080", false},
		{"10.0.0.1:8080", false},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			assert.Equal(t, tt.expected, isLocalhost(tt.addr))
		})
	}
}

func TestParseSandboxState(t *testing.T) {
	tests := []struct {
		input    string
		expected SandboxState
	}{
		{"Phase: Running", SandboxStateRunning},
		{"Phase: Ready", SandboxStateRunning},
		{"Phase: Provisioning", SandboxStateCreating},
		{"Phase: Error", SandboxStateError},
		{"Phase: Deleting", SandboxStateDeleted},
		{"running", SandboxStateRunning},
		{"ready", SandboxStateRunning},
		{"provisioning", SandboxStateCreating},
		{"not found", SandboxStateDeleted},
		{"unknown-state", SandboxStateError},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseSandboxState(tt.input))
		})
	}
}

func TestParseSandboxState_FullOutput(t *testing.T) {
	output := "Sandbox:\n\n  Id: abc-123\n  Name: test-sb\n  Phase: Ready\n  Policy source: sandbox\n"
	assert.Equal(t, SandboxStateRunning, parseSandboxState(output))
}

func TestParseSandboxName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"simple",
			"Created sandbox: my-sandbox\n",
			"my-sandbox",
		},
		{
			"with progress",
			"Created sandbox: polite-euglena\n\n  [0.0s] Requesting compute...\n[4.5s] Sandbox allocated\n",
			"polite-euglena",
		},
		{
			"with ansi codes",
			"\x1b[1m\x1b[36mCreated sandbox:\x1b[39m\x1b[0m \x1b[1mpolite-euglena\x1b[0m\n",
			"polite-euglena",
		},
		{
			"empty",
			"",
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseSandboxName(tt.input))
		})
	}
}

func TestStripANSI(t *testing.T) {
	assert.Equal(t, "hello", stripANSI("\x1b[1mhello\x1b[0m"))
	assert.Equal(t, "Phase: Ready", stripANSI("\x1b[2mPhase:\x1b[0m Ready"))
	assert.Equal(t, "plain text", stripANSI("plain text"))
}

func TestNewClient_ReturnsInterface(t *testing.T) {
	cfg := GatewayConfig{Address: "localhost:17670"}
	client, err := NewClient(cfg)
	require.NoError(t, err)
	var _ Client = client
}

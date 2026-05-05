package openshell

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_ValidConfig(t *testing.T) {
	cfg := GatewayConfig{Address: "localhost:8080"}
	client, err := NewClient(cfg)
	require.NoError(t, err)
	assert.Equal(t, "localhost:8080", client.Address())
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
	assert.Equal(t, "localhost:8080", result.Address)
}

func TestResolveGatewayConfig_EmptyDefinition(t *testing.T) {
	t.Setenv("OPENSHELL_GATEWAY_URL", "")
	cfg := &GatewayConfig{}
	result := ResolveGatewayConfig(cfg)
	assert.Equal(t, "localhost:8080", result.Address)
}

func TestIsLocalhost(t *testing.T) {
	tests := []struct {
		addr     string
		expected bool
	}{
		{"localhost:8080", true},
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
		{"running", SandboxStateRunning},
		{"Running", SandboxStateRunning},
		{"creating", SandboxStateCreating},
		{"suspended", SandboxStateSuspended},
		{"error", SandboxStateError},
		{"deleted", SandboxStateDeleted},
		{"not found", SandboxStateDeleted},
		{"unknown-state", SandboxStateError},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseSandboxState(tt.input))
		})
	}
}

func TestParseSSHSession_Defaults(t *testing.T) {
	session := parseSSHSession("")
	assert.Equal(t, "localhost", session.Host)
	assert.Equal(t, 22, session.Port)
	assert.Equal(t, "sandbox", session.User)
}

func TestParseSSHSession_Parsed(t *testing.T) {
	output := "host: gateway.local\nport: 2222\nuser: dev\nsession_id: abc-123"
	session := parseSSHSession(output)
	assert.Equal(t, "gateway.local", session.Host)
	assert.Equal(t, 2222, session.Port)
	assert.Equal(t, "dev", session.User)
	assert.Equal(t, "abc-123", session.SessionID)
}

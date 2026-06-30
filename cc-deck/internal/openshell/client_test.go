package openshell

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/rhuss/openshell-sdk-go/openshell/v1"
)

func TestNewSDKClient_ValidConfig(t *testing.T) {
	cfg := GatewayConfig{Address: "localhost:17670"}
	client, err := NewSDKClient(cfg)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewSDKClient_EmptyAddress(t *testing.T) {
	cfg := GatewayConfig{}
	_, err := NewSDKClient(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "address")
}

func TestNewSDKClient_ReturnsInterface(t *testing.T) {
	cfg := GatewayConfig{Address: "localhost:17670"}
	client, err := NewSDKClient(cfg)
	require.NoError(t, err)
	var _ v1.ClientInterface = client
}

func TestToSDKConfig_Basic(t *testing.T) {
	cfg := GatewayConfig{Address: "localhost:17670"}
	sdkCfg, err := cfg.ToSDKConfig()
	require.NoError(t, err)
	assert.Equal(t, "localhost:17670", sdkCfg.Address)
	assert.NotNil(t, sdkCfg.Auth)
	require.NotNil(t, sdkCfg.TLS)
	assert.True(t, sdkCfg.TLS.Insecure)
}

func TestToSDKConfig_WithTLS(t *testing.T) {
	cfg := GatewayConfig{
		Address:     "gateway.example.com:9090",
		TLS:         true,
		TLSCertPath: "/path/cert.pem",
		TLSKeyPath:  "/path/key.pem",
		TLSCAPath:   "/path/ca.pem",
	}
	sdkCfg, err := cfg.ToSDKConfig()
	require.NoError(t, err)
	assert.Equal(t, "gateway.example.com:9090", sdkCfg.Address)
	require.NotNil(t, sdkCfg.TLS)
	assert.Equal(t, "/path/cert.pem", sdkCfg.TLS.CertFile)
	assert.Equal(t, "/path/key.pem", sdkCfg.TLS.KeyFile)
	assert.Equal(t, "/path/ca.pem", sdkCfg.TLS.CAFile)
}

func TestToSDKConfig_NoTLSUsesInsecure(t *testing.T) {
	cfg := GatewayConfig{Address: "localhost:17670", TLS: false}
	sdkCfg, err := cfg.ToSDKConfig()
	require.NoError(t, err)
	require.NotNil(t, sdkCfg.TLS)
	assert.True(t, sdkCfg.TLS.Insecure)
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

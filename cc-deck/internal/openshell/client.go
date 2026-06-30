package openshell

import (
	"log"
	"net"
	"os"
	"strings"

	v1 "github.com/rhuss/openshell-sdk-go/openshell/v1"
)

// GatewayConfig holds connection parameters for the OpenShell gateway.
type GatewayConfig struct {
	Address     string
	TLS         bool
	TLSCertPath string
	TLSKeyPath  string
	TLSCAPath   string
}

// ResolveGatewayConfig determines the gateway configuration by checking
// the workspace definition first, then the environment variable, then
// the default.
func ResolveGatewayConfig(gateway *GatewayConfig) GatewayConfig {
	if gateway != nil && gateway.Address != "" {
		return *gateway
	}
	if envAddr, ok := os.LookupEnv("OPENSHELL_GATEWAY_URL"); ok && envAddr != "" {
		return GatewayConfig{Address: envAddr}
	}
	return GatewayConfig{Address: "localhost:17670"}
}

// ToSDKConfig converts a GatewayConfig to the SDK's v1.Config.
func (g GatewayConfig) ToSDKConfig() (v1.Config, error) {
	cfg := v1.Config{
		Address: g.Address,
		Auth:    v1.NoAuth(),
	}
	if g.TLS {
		cfg.TLS = &v1.TLSConfig{
			CertFile: g.TLSCertPath,
			KeyFile:  g.TLSKeyPath,
			CAFile:   g.TLSCAPath,
		}
	} else {
		cfg.TLS = &v1.TLSConfig{Insecure: true}
		if !isLocalhost(g.Address) {
			log.Printf("WARNING: connecting to non-localhost gateway %s without TLS", g.Address)
		}
	}
	return cfg, nil
}

// isLocalhost returns true if the address targets a loopback interface.
func isLocalhost(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

package share

import "github.com/cc-deck/cc-deck/internal/config"

// sharingConfigWithEndpoint builds the single-endpoint configuration shape.
func sharingConfigWithEndpoint(address string) config.SharingConfig {
	return config.SharingConfig{Endpoint: address}
}

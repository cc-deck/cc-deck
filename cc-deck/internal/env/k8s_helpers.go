package env

import (
	"github.com/cc-deck/cc-deck/internal/setup"
	"github.com/cc-deck/cc-deck/internal/network"
)

// newDomainResolver creates a domain resolver with user config.
func newDomainResolver() (*network.Resolver, error) {
	userGroups, err := network.LoadUserConfig()
	if err != nil {
		// Non-fatal: use empty user config.
		userGroups = nil
	}
	return network.NewResolver(userGroups), nil
}

// loadBuildManifest reads a cc-deck-image.yaml manifest file.
func loadBuildManifest(path string) (*setup.Manifest, error) {
	return setup.LoadManifest(path)
}

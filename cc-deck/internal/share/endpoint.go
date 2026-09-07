package share

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"

	"github.com/cc-deck/cc-deck/internal/config"
)

// StaticEndpoint is a reverse proxy the user already runs in front of the
// Zellij web server. It owns no process: there is no Start, no Stop, and no
// PID anywhere in this type, because cc-deck neither creates nor supervises
// the endpoint it verifies.
type StaticEndpoint struct {
	cfg config.SharingConfig
	// override is the --endpoint address, used for one command only.
	override string
	// selected is the --endpoint-name selection.
	selected string
	zellij   Zellij

	// Test seams. Production leaves these nil, which selects the system
	// resolver, a well known public resolver, and strict TLS verification.
	systemResolver *net.Resolver
	publicResolver *net.Resolver
	tlsConfig      *tls.Config
}

func NewStaticEndpoint(cfg config.SharingConfig, override, selected string, zellij Zellij) *StaticEndpoint {
	return &StaticEndpoint{cfg: cfg, override: override, selected: selected, zellij: zellij}
}

// Name identifies the implementation, not the configured endpoint. The name of
// the endpoint a particular share used is EndpointRef.Name, recorded on the
// operation, because that is what must survive a configuration edit.
func (e *StaticEndpoint) Name() string { return "static" }

// Resolve applies the endpoint resolution precedence and performs no network
// access whatsoever, so a caller that only needs an address never pays for a
// probe.
//
// Precedence: the --endpoint address, then the --endpoint-name selection, then
// the configured default, then the single configured endpoint.
func (e *StaticEndpoint) Resolve(_ context.Context) (EndpointRef, error) {
	if e.override != "" && e.selected != "" {
		return EndpointRef{}, fmt.Errorf("--endpoint and --endpoint-name cannot be used together")
	}
	switch {
	case e.override != "":
		if err := validateEndpointURL(e.override); err != nil {
			return EndpointRef{}, err
		}
		// An address given by flag has no configured name.
		return EndpointRef{Name: "", BaseURL: e.override}, nil

	case e.selected != "":
		address, ok := e.cfg.Endpoints[e.selected]
		if !ok {
			return EndpointRef{}, fmt.Errorf("no endpoint named %q is configured; %s", e.selected, configuredEndpointNames(e.cfg))
		}
		if err := validateEndpointURL(address); err != nil {
			return EndpointRef{}, fmt.Errorf("endpoint %q: %w", e.selected, err)
		}
		return EndpointRef{Name: e.selected, BaseURL: address}, nil

	case e.cfg.Default != "":
		address, ok := e.cfg.Endpoints[e.cfg.Default]
		if !ok {
			return EndpointRef{}, fmt.Errorf("sharing.default names %q, which is not a configured endpoint; %s", e.cfg.Default, configuredEndpointNames(e.cfg))
		}
		if err := validateEndpointURL(address); err != nil {
			return EndpointRef{}, fmt.Errorf("endpoint %q: %w", e.cfg.Default, err)
		}
		return EndpointRef{Name: e.cfg.Default, BaseURL: address}, nil

	case e.cfg.Endpoint != "":
		if err := validateEndpointURL(e.cfg.Endpoint); err != nil {
			return EndpointRef{}, err
		}
		return EndpointRef{Name: "", BaseURL: e.cfg.Endpoint}, nil
	}

	return EndpointRef{}, fmt.Errorf(
		"no sharing endpoint is configured, so there is nothing to share through; %s\n"+
			"cc-deck does not start an endpoint for you. Set sharing.endpoint in your cc-deck config,\n"+
			"or pass --endpoint URL for one command. See the sharing guide for endpoint requirements.",
		configuredEndpointNames(e.cfg))
}

// validateEndpointURL rejects anything that is not an absolute http or https
// address, which is the only shape an endpoint can usefully take.
func validateEndpointURL(address string) error {
	parsed, err := url.Parse(address)
	if err != nil {
		return fmt.Errorf("endpoint address %q is not a valid URL: %w", address, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("endpoint address %q must use the http or https scheme", address)
	}
	if parsed.Host == "" {
		return fmt.Errorf("endpoint address %q must be an absolute URL with a host", address)
	}
	return nil
}

// configuredEndpointNames renders the names a user could have meant, so a
// refusal points somewhere rather than merely saying no.
func configuredEndpointNames(cfg config.SharingConfig) string {
	names := make([]string, 0, len(cfg.Endpoints))
	for name := range cfg.Endpoints {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		if cfg.Endpoint != "" {
			return "the only configured endpoint is " + cfg.Endpoint
		}
		return "no endpoints are configured"
	}
	return "configured endpoints: " + strings.Join(names, ", ")
}

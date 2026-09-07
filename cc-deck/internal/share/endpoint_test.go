package share

import (
	"context"
	"testing"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/stretchr/testify/require"
)

// sharingConfigWithEndpoint builds the single-endpoint configuration shape.
func sharingConfigWithEndpoint(address string) config.SharingConfig {
	return config.SharingConfig{Endpoint: address}
}

// namedConfig is the multi-endpoint shape used across the resolution tests.
func namedConfig() config.SharingConfig {
	return config.SharingConfig{
		Endpoint:  "https://single.example",
		Endpoints: map[string]string{"work": "https://work.example", "home": "https://home.example"},
		Default:   "home",
	}
}

// T050: the full resolution precedence, in order.
func TestEndpointResolutionPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cfg      config.SharingConfig
		override string
		selected string
		wantURL  string
		wantName string
	}{
		{
			name:     "the explicit flag wins over everything",
			cfg:      namedConfig(),
			override: "https://flag.example",
			wantURL:  "https://flag.example",
			wantName: "", // an address given by flag has no configured name
		},
		{
			name:     "a named selection wins over the declared default",
			cfg:      namedConfig(),
			selected: "work",
			wantURL:  "https://work.example",
			wantName: "work",
		},
		{
			name:     "the declared default wins over the single endpoint",
			cfg:      namedConfig(),
			wantURL:  "https://home.example",
			wantName: "home",
		},
		{
			name:    "the single endpoint is used when nothing else applies",
			cfg:     sharingConfigWithEndpoint("https://single.example"),
			wantURL: "https://single.example",
		},
		{
			name: "a named endpoint with no declared default still needs naming",
			cfg: config.SharingConfig{
				Endpoint:  "https://single.example",
				Endpoints: map[string]string{"work": "https://work.example"},
			},
			wantURL: "https://single.example",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := NewStaticEndpoint(tc.cfg, tc.override, tc.selected, nil).Resolve(context.Background())
			require.NoError(t, err)
			require.Equal(t, tc.wantURL, ref.BaseURL)
			require.Equal(t, tc.wantName, ref.Name)
		})
	}
}

// T051: an unknown name fails with the configured names listed, so a typo
// points somewhere rather than merely being refused.
func TestUnknownEndpointNameListsTheConfiguredNames(t *testing.T) {
	_, err := NewStaticEndpoint(namedConfig(), "", "office", nil).Resolve(context.Background())
	require.ErrorContains(t, err, `no endpoint named "office" is configured`)
	require.ErrorContains(t, err, "home, work")
}

// T027: refusing to share when nothing resolves must say what to do next.
func TestResolveRefusesWithActionableGuidanceWhenNothingIsConfigured(t *testing.T) {
	_, err := NewStaticEndpoint(config.SharingConfig{}, "", "", nil).Resolve(context.Background())
	require.ErrorContains(t, err, "no sharing endpoint is configured")
	require.ErrorContains(t, err, "cc-deck does not start an endpoint for you")
	require.ErrorContains(t, err, "sharing.endpoint")
	require.ErrorContains(t, err, "--endpoint")
}

func TestResolveListsConfiguredNamesWhenTheDefaultIsWrong(t *testing.T) {
	cfg := config.SharingConfig{
		Endpoints: map[string]string{"work": "https://work.example", "home": "https://home.example"},
		Default:   "office",
	}
	_, err := NewStaticEndpoint(cfg, "", "", nil).Resolve(context.Background())
	require.ErrorContains(t, err, `sharing.default names "office"`)
	require.ErrorContains(t, err, "home, work")
}

func TestResolveRejectsAddressesThatAreNotAbsoluteHTTPURLs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cfg      config.SharingConfig
		override string
		selected string
		want     string
	}{
		{"relative override", config.SharingConfig{}, "dev.example.com", "", "must use the http or https scheme"},
		{"non-http scheme", config.SharingConfig{}, "ssh://dev.example.com", "", "must use the http or https scheme"},
		{"no host", config.SharingConfig{}, "https://", "", "must be an absolute URL with a host"},
		{"malformed named endpoint", config.SharingConfig{Endpoints: map[string]string{"work": "nope"}}, "", "work", "must use the http or https scheme"},
		{"malformed single endpoint", config.SharingConfig{Endpoint: "nope"}, "", "", "must use the http or https scheme"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewStaticEndpoint(tc.cfg, tc.override, tc.selected, nil).Resolve(context.Background())
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestResolveRefusesBothSelectorsAtOnce(t *testing.T) {
	_, err := NewStaticEndpoint(namedConfig(), "https://flag.example", "work", nil).Resolve(context.Background())
	require.ErrorContains(t, err, "cannot be used together")
}

// The endpoint name recorded on a share is what survives a configuration edit,
// so it travels on the reference rather than being looked up again later.
func TestResolveCarriesTheConfiguredNameOnTheReference(t *testing.T) {
	ref, err := NewStaticEndpoint(namedConfig(), "", "work", nil).Resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, EndpointRef{Name: "work", BaseURL: "https://work.example"}, ref)
}

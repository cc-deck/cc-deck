package share

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// The behavioural contract for Endpoint implementations lands with the stage
// matrix that exercises it. This skeleton asserts only the shape: that the
// production implementation satisfies the interface and that the interface has
// no lifecycle surface at all.

func TestStaticEndpointSatisfiesTheEndpointInterface(t *testing.T) {
	var endpoint Endpoint = NewStaticEndpoint(sharingConfigWithEndpoint("https://dev.example.com"), "", "", &startZellij{})
	require.NotEmpty(t, endpoint.Name())
	ref, err := endpoint.Resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, "https://dev.example.com", ref.BaseURL)
}

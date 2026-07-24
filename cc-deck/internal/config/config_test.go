package config

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSharingProviderDefault(t *testing.T) {
	require.Equal(t, "cloudflare", (&Config{}).SharingProvider())
	require.Equal(t, "other", (&Config{Sharing: SharingConfig{Provider: "other"}}).SharingProvider())
}

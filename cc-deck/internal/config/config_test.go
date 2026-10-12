package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestVerifyTimeoutFallsBackToDefault(t *testing.T) {
	require.Equal(t, DefaultVerifyTimeout, (&Config{}).VerifyTimeout())
	require.Equal(t, DefaultVerifyTimeout, (&Config{Sharing: SharingConfig{VerifyTimeout: 0}}).VerifyTimeout())
	require.Equal(t, 3*time.Second, (&Config{Sharing: SharingConfig{VerifyTimeout: 3 * time.Second}}).VerifyTimeout())
}

func TestSharingSchemaRoundTripsNamedEndpoints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
sharing:
  endpoint: https://single.example
  endpoints:
    work: https://work.example
    home: http://home.example:8080
  default: work
  verify_timeout: 20s
`), 0o600))

	cfg, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, "https://single.example", cfg.Sharing.Endpoint)
	require.Equal(t, map[string]string{"work": "https://work.example", "home": "http://home.example:8080"}, cfg.Sharing.Endpoints)
	require.Equal(t, "work", cfg.Sharing.Default)
	require.Equal(t, 20*time.Second, cfg.VerifyTimeout())
}

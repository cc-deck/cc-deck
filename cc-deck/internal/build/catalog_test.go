package build

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T034: CatalogIndex parsing tests

func TestFetchCatalogIndex_ValidYAML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
version: 1
components:
  - rust.yaml
  - go.yaml
  - custom.yaml
base_url: https://example.com/components
`))
	}))
	defer server.Close()

	index, err := FetchCatalogIndex(server.URL)
	require.NoError(t, err)
	assert.Equal(t, 1, index.Version)
	assert.Equal(t, []string{"rust.yaml", "go.yaml", "custom.yaml"}, index.Components)
	assert.Equal(t, "https://example.com/components", index.BaseURL)
}

func TestFetchCatalogIndex_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := FetchCatalogIndex(server.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 404")
}

func TestFetchCatalogIndex_InvalidYAML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`invalid: [yaml: broken`))
	}))
	defer server.Close()

	_, err := FetchCatalogIndex(server.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing catalog index")
}

func TestDownloadCatalogComponents_WritesFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rust.yaml":
			_, _ = w.Write([]byte("key: pkg_rust\nname: rust\nmatch:\n  tools: [rust]\nendpoints:\n  - host: crates.io\n    port: 443\n"))
		case "/custom.yaml":
			_, _ = w.Write([]byte("key: custom\nname: custom\nmatch:\n  always: true\nendpoints:\n  - host: custom.io\n    port: 443\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	index := &CatalogIndex{
		Version:    1,
		Components: []string{"rust.yaml", "custom.yaml"},
	}

	err := DownloadCatalogComponents(index, server.URL, cacheDir)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(cacheDir, "rust.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "pkg_rust")

	data, err = os.ReadFile(filepath.Join(cacheDir, "custom.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "custom")
}

// T035: Offline fallback tests

func TestFetchAndCacheCatalog_OfflineFallback(t *testing.T) {
	err := FetchAndCacheCatalog("http://127.0.0.1:1/nonexistent", "http://127.0.0.1:1", t.TempDir())
	assert.NoError(t, err, "offline fallback should warn but not error")
}

func TestFetchAndCacheCatalog_PartialDownloadContinues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/catalog.yaml":
			_, _ = w.Write([]byte("version: 1\ncomponents:\n  - good.yaml\n  - missing.yaml\n"))
		case "/good.yaml":
			_, _ = w.Write([]byte("key: good\nname: good\nmatch:\n  always: true\nendpoints:\n  - host: good.io\n    port: 443\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	index := &CatalogIndex{
		Version:    1,
		Components: []string{"good.yaml", "missing.yaml"},
	}

	err := DownloadCatalogComponents(index, server.URL, cacheDir)
	require.NoError(t, err)

	_, err = os.ReadFile(filepath.Join(cacheDir, "good.yaml"))
	assert.NoError(t, err, "good.yaml should be cached")

	_, err = os.ReadFile(filepath.Join(cacheDir, "missing.yaml"))
	assert.Error(t, err, "missing.yaml should not be cached")
}

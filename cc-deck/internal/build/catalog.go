package build

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// CatalogIndex is the catalog.yaml file in the remote catalog repo.
type CatalogIndex struct {
	Version    int      `yaml:"version"`
	Components []string `yaml:"components"`
	BaseURL    string   `yaml:"base_url,omitempty"`
}

const (
	DefaultCatalogIndexURL = "https://raw.githubusercontent.com/cc-deck/openshell-policies/main/catalog.yaml"
	DefaultCatalogBaseURL  = "https://raw.githubusercontent.com/cc-deck/openshell-policies/main"

	maxCatalogIndexSize  = 64 * 1024  // 64 KB
	maxComponentFileSize = 512 * 1024 // 512 KB
)

// FetchCatalogIndex downloads and parses the catalog index from the given URL.
func FetchCatalogIndex(indexURL string) (*CatalogIndex, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(indexURL)
	if err != nil {
		return nil, fmt.Errorf("fetching catalog index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching catalog index: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalogIndexSize))
	if err != nil {
		return nil, fmt.Errorf("reading catalog index: %w", err)
	}

	var index CatalogIndex
	if err := yaml.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("parsing catalog index: %w", err)
	}

	return &index, nil
}

// DownloadCatalogComponents fetches all component files listed in the catalog
// index and writes them to the cache directory. The baseURL provided by the
// caller is always used; the index's base_url field is ignored for security
// (prevents SSRF via attacker-controlled redirect).
func DownloadCatalogComponents(index *CatalogIndex, baseURL string, cacheDir string) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, filename := range index.Components {
		if strings.Contains(filename, "..") || strings.ContainsAny(filename, "/\\") {
			fmt.Printf("WARNING: skipping suspicious filename: %s\n", filename)
			continue
		}

		url := strings.TrimRight(baseURL, "/") + "/" + filename
		resp, err := client.Get(url)
		if err != nil {
			fmt.Printf("WARNING: failed to download %s: %v\n", filename, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			fmt.Printf("WARNING: failed to download %s: HTTP %d\n", filename, resp.StatusCode)
			continue
		}

		data, err := io.ReadAll(io.LimitReader(resp.Body, maxComponentFileSize))
		resp.Body.Close()

		if err != nil {
			fmt.Printf("WARNING: failed to read %s: %v\n", filename, err)
			continue
		}

		target := filepath.Join(cacheDir, filename)
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
	}

	return nil
}

// FetchAndCacheCatalog is the high-level catalog update operation.
// On network failure, it warns and returns nil (offline fallback).
func FetchAndCacheCatalog(indexURL string, baseURL string, cacheDir string) error {
	index, err := FetchCatalogIndex(indexURL)
	if err != nil {
		fmt.Printf("WARNING: catalog update skipped: %v\n", err)
		return nil
	}

	if err := DownloadCatalogComponents(index, baseURL, cacheDir); err != nil {
		fmt.Printf("WARNING: catalog download incomplete: %v\n", err)
		return nil
	}

	return nil
}

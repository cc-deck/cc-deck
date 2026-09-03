package record

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/build"
)

const testComponentYAMLTmpl = `key: %s
name: %s
match:
  always: true
endpoints:
  - host: %s
    port: 443
`

func writeComponentFile(t *testing.T, dir, filename, key, name, host string) {
	t.Helper()
	content := fmt.Sprintf(testComponentYAMLTmpl, key, name, host)
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644))
}

func TestBuildDomainIndex_EmbeddedComponents(t *testing.T) {
	index := BuildDomainIndex(nil, "", nil, "")

	// Embedded components should be loaded; the exact domains depend
	// on what's in the embedded policies, but the index should not be empty.
	assert.True(t, len(index) > 0, "domain index should contain entries from embedded components")
}

func TestMatchAgainstCatalog_CoveredByComponent(t *testing.T) {
	index := BuildDomainIndex(nil, "", nil, "")

	// Find a domain that's in the embedded components.
	var knownDomain string
	var knownComponent string
	for domain, comp := range index {
		knownDomain = domain
		knownComponent = comp
		break
	}

	if knownDomain == "" {
		t.Skip("no embedded components with endpoints found")
	}

	result := &RecordingResult{
		ObservedDomains: []string{knownDomain, "unknown-domain.example.com"},
	}

	manifest := &build.Manifest{Version: 3}
	matched := MatchAgainstCatalog(result, manifest, "", "")

	require.Len(t, matched.CoveredDomains, 1)
	assert.Equal(t, knownDomain, matched.CoveredDomains[0].Domain)
	assert.Equal(t, knownComponent, matched.CoveredDomains[0].CoveredBy)

	require.Len(t, matched.NewDomains, 1)
	assert.Equal(t, "unknown-domain.example.com", matched.NewDomains[0])
}

func TestMatchAgainstCatalog_CoveredByAllowedDomains(t *testing.T) {
	result := &RecordingResult{
		ObservedDomains: []string{"already-allowed.com", "brand-new.com"},
	}

	manifest := &build.Manifest{
		Version: 3,
		Network: &build.NetworkConfig{
			AllowedDomains: []string{"already-allowed.com"},
		},
	}

	matched := MatchAgainstCatalog(result, manifest, "", "")

	var allowedCovered bool
	for _, cd := range matched.CoveredDomains {
		if cd.Domain == "already-allowed.com" && cd.CoveredBy == "allowed_domains" {
			allowedCovered = true
		}
	}
	assert.True(t, allowedCovered, "already-allowed.com should be covered by allowed_domains")

	require.Len(t, matched.NewDomains, 1)
	assert.Equal(t, "brand-new.com", matched.NewDomains[0])
}

func TestMatchAgainstCatalog_AllCovered(t *testing.T) {
	result := &RecordingResult{
		ObservedDomains: []string{"existing.com"},
	}

	manifest := &build.Manifest{
		Version: 3,
		Network: &build.NetworkConfig{
			AllowedDomains: []string{"existing.com"},
		},
	}

	matched := MatchAgainstCatalog(result, manifest, "", "")
	assert.Empty(t, matched.NewDomains)
	assert.Len(t, matched.CoveredDomains, 1)
}

func TestMatchAgainstCatalog_NilNetwork(t *testing.T) {
	result := &RecordingResult{
		ObservedDomains: []string{"example.com"},
	}

	manifest := &build.Manifest{Version: 3}
	matched := MatchAgainstCatalog(result, manifest, "", "")

	assert.Len(t, matched.NewDomains, 1)
	assert.Equal(t, "example.com", matched.NewDomains[0])
}

func TestMatchAgainstCatalog_EmptyObserved(t *testing.T) {
	result := &RecordingResult{}
	manifest := &build.Manifest{Version: 3}
	matched := MatchAgainstCatalog(result, manifest, "", "")

	assert.Empty(t, matched.CoveredDomains)
	assert.Empty(t, matched.NewDomains)
}

func TestMatchAgainstCatalog_CaseInsensitive(t *testing.T) {
	result := &RecordingResult{
		ObservedDomains: []string{"EXISTING.COM"},
	}

	manifest := &build.Manifest{
		Version: 3,
		Network: &build.NetworkConfig{
			AllowedDomains: []string{"existing.com"},
		},
	}

	matched := MatchAgainstCatalog(result, manifest, "", "")
	assert.Len(t, matched.CoveredDomains, 1)
	assert.Empty(t, matched.NewDomains)
}

func TestBuildDomainIndex_CatalogFSOverlay(t *testing.T) {
	dir := t.TempDir()
	writeComponentFile(t, dir, "custom.yaml", "custom_component", "custom component", "custom.example.com")

	index := BuildDomainIndex(os.DirFS(dir), ".", nil, "")

	assert.Equal(t, "custom component", index["custom.example.com"])
}

func TestBuildDomainIndex_UserLocalFSOverlay(t *testing.T) {
	dir := t.TempDir()
	writeComponentFile(t, dir, "userlocal.yaml", "userlocal_component", "user local component", "userlocal.example.com")

	index := BuildDomainIndex(nil, "", os.DirFS(dir), ".")

	assert.Equal(t, "user local component", index["userlocal.example.com"])
}

func TestBuildDomainIndex_UserLocalOverridesCatalog(t *testing.T) {
	catalogDir := t.TempDir()
	writeComponentFile(t, catalogDir, "shared.yaml", "shared_component", "catalog version", "shared.example.com")

	userLocalDir := t.TempDir()
	writeComponentFile(t, userLocalDir, "shared.yaml", "shared_component", "user-local version", "shared.example.com")

	index := BuildDomainIndex(os.DirFS(catalogDir), ".", os.DirFS(userLocalDir), ".")

	assert.Equal(t, "user-local version", index["shared.example.com"],
		"user-local tier should be applied after (and override) catalog tier")
}

func TestBuildDomainIndex_CaseInsensitiveHost(t *testing.T) {
	dir := t.TempDir()
	writeComponentFile(t, dir, "mixed.yaml", "mixed_component", "mixed case component", "Mixed.Example.COM")

	index := BuildDomainIndex(os.DirFS(dir), ".", nil, "")

	assert.Equal(t, "mixed case component", index["mixed.example.com"])
}

func TestMatchAgainstCatalog_UsesCatalogDirAndUserLocalDir(t *testing.T) {
	catalogDir := t.TempDir()
	writeComponentFile(t, catalogDir, "catalog.yaml", "catalog_component", "catalog comp", "catalog-domain.com")

	userLocalDir := t.TempDir()
	writeComponentFile(t, userLocalDir, "userlocal.yaml", "userlocal_component", "userlocal comp", "userlocal-domain.com")

	result := &RecordingResult{
		ObservedDomains: []string{"catalog-domain.com", "userlocal-domain.com", "totally-new.com"},
	}
	manifest := &build.Manifest{Version: 3}

	matched := MatchAgainstCatalog(result, manifest, catalogDir, userLocalDir)

	require.Len(t, matched.CoveredDomains, 2)
	require.Len(t, matched.NewDomains, 1)
	assert.Equal(t, "totally-new.com", matched.NewDomains[0])
}

func TestMatchAgainstCatalog_NonexistentCatalogDirIgnored(t *testing.T) {
	result := &RecordingResult{
		ObservedDomains: []string{"example.com"},
	}
	manifest := &build.Manifest{Version: 3}

	matched := MatchAgainstCatalog(result, manifest, "/nonexistent/catalog/dir", "/nonexistent/userlocal/dir")

	require.Len(t, matched.NewDomains, 1)
	assert.Equal(t, "example.com", matched.NewDomains[0])
}

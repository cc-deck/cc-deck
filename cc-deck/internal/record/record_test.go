package record

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/build"
)

func TestGenerateCorefile(t *testing.T) {
	corefile := GenerateCorefile()

	assert.Contains(t, corefile, "forward . /etc/resolv.conf")
	assert.Contains(t, corefile, "log .")
	assert.Contains(t, corefile, ".:53")
}

func TestUpdateManifest_AppendsNewDomains(t *testing.T) {
	m := &build.Manifest{
		Version: 3,
		Network: &build.NetworkConfig{
			AllowedDomains: []string{"existing.com"},
		},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "build.yaml")
	require.NoError(t, build.SaveManifest(m, path))

	err := UpdateManifest(path, []string{"new-domain.org", "another.io"})
	require.NoError(t, err)

	loaded, err := build.LoadManifest(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"existing.com", "new-domain.org", "another.io"}, loaded.Network.AllowedDomains)
}

func TestUpdateManifest_DeduplicatesAgainstExisting(t *testing.T) {
	m := &build.Manifest{
		Version: 3,
		Network: &build.NetworkConfig{
			AllowedDomains: []string{"pypi.org", "crates.io"},
		},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "build.yaml")
	require.NoError(t, build.SaveManifest(m, path))

	err := UpdateManifest(path, []string{"pypi.org", "new.com", "crates.io"})
	require.NoError(t, err)

	loaded, err := build.LoadManifest(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"pypi.org", "crates.io", "new.com"}, loaded.Network.AllowedDomains)
}

func TestUpdateManifest_InitializesNetworkIfNil(t *testing.T) {
	m := &build.Manifest{Version: 3}
	dir := t.TempDir()
	path := filepath.Join(dir, "build.yaml")
	require.NoError(t, build.SaveManifest(m, path))

	err := UpdateManifest(path, []string{"example.com"})
	require.NoError(t, err)

	loaded, err := build.LoadManifest(path)
	require.NoError(t, err)
	require.NotNil(t, loaded.Network)
	assert.Equal(t, []string{"example.com"}, loaded.Network.AllowedDomains)
}

func TestUpdateManifest_EmptyNewDomains(t *testing.T) {
	m := &build.Manifest{
		Version: 3,
		Network: &build.NetworkConfig{
			AllowedDomains: []string{"existing.com"},
		},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "build.yaml")
	require.NoError(t, build.SaveManifest(m, path))

	err := UpdateManifest(path, nil)
	require.NoError(t, err)

	loaded, err := build.LoadManifest(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"existing.com"}, loaded.Network.AllowedDomains)
}

func TestUpdateManifest_CaseInsensitiveDedup(t *testing.T) {
	m := &build.Manifest{
		Version: 3,
		Network: &build.NetworkConfig{
			AllowedDomains: []string{"PyPI.org"},
		},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "build.yaml")
	require.NoError(t, build.SaveManifest(m, path))

	err := UpdateManifest(path, []string{"pypi.org"})
	require.NoError(t, err)

	loaded, err := build.LoadManifest(path)
	require.NoError(t, err)
	assert.Len(t, loaded.Network.AllowedDomains, 1)
}

func TestPrintSummary_NewDomains(t *testing.T) {
	result := &RecordingResult{
		ObservedDomains: []string{"pypi.org", "crates.io", "new.com"},
		NewDomains:      []string{"new.com"},
		CoveredDomains:  []CoveredDomain{{Domain: "pypi.org", CoveredBy: "python"}, {Domain: "crates.io", CoveredBy: "rust"}},
		TotalQueries:    50,
		FilteredCount:   10,
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintSummary(result, "/path/to/build.yaml")

	w.Close()
	os.Stdout = old

	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "Total DNS queries")
	assert.Contains(t, output, "New domains added")
	assert.Contains(t, output, "new.com")
	assert.Contains(t, output, "build refresh")
	assert.Contains(t, output, "Already covered")
	assert.Contains(t, output, "pypi.org")
	assert.Contains(t, output, "python")
}

func TestPrintSummary_AllCovered(t *testing.T) {
	result := &RecordingResult{
		ObservedDomains: []string{"pypi.org"},
		CoveredDomains:  []CoveredDomain{{Domain: "pypi.org", CoveredBy: "python"}},
		TotalQueries:    10,
		FilteredCount:   5,
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintSummary(result, "/path/to/build.yaml")

	w.Close()
	os.Stdout = old

	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "already covered")
}

func TestPrintSummary_NoDomains(t *testing.T) {
	result := &RecordingResult{
		TotalQueries:  5,
		FilteredCount: 5,
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintSummary(result, "/path/to/build.yaml")

	w.Close()
	os.Stdout = old

	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "No meaningful egress domains observed")
}

func TestRecordingResult_Fields(t *testing.T) {
	result := RecordingResult{
		ObservedDomains: []string{"a.com", "b.com"},
		CoveredDomains:  []CoveredDomain{{Domain: "a.com", CoveredBy: "python"}},
		NewDomains:      []string{"b.com"},
		TotalQueries:    100,
		FilteredCount:   20,
	}

	assert.Len(t, result.ObservedDomains, 2)
	assert.Len(t, result.CoveredDomains, 1)
	assert.Len(t, result.NewDomains, 1)
	assert.Equal(t, 100, result.TotalQueries)
	assert.Equal(t, 20, result.FilteredCount)
}

func TestCoreDNSImage_Constant(t *testing.T) {
	assert.True(t, strings.HasPrefix(CoreDNSImage, "docker.io/coredns/coredns:"),
		"CoreDNS image should reference the official image")
}

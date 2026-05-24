package record

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/build"
)

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

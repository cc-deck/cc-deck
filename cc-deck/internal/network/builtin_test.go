package network

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltinGroupNames_ContainsKnownGroups(t *testing.T) {
	names := BuiltinGroupNames()
	require.NotEmpty(t, names)

	for _, want := range []string{"anthropic", "python", "nodejs", "rust", "golang", "docker"} {
		assert.Contains(t, names, want)
	}
}

func TestBuiltinGroupNames_Sorted(t *testing.T) {
	names := BuiltinGroupNames()
	sorted := make([]string, len(names))
	copy(sorted, names)
	sort.Strings(sorted)
	assert.Equal(t, sorted, names)
}

func TestBuiltinGroupNames_MatchesBuiltinGroupsMap(t *testing.T) {
	names := BuiltinGroupNames()
	assert.Len(t, names, len(builtinGroups))
	for _, name := range names {
		_, ok := builtinGroups[name]
		assert.True(t, ok, "name %q should exist in builtinGroups", name)
	}
}

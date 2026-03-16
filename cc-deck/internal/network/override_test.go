package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyOverrides_Empty(t *testing.T) {
	result, err := ApplyOverrides("", []string{"python", "github"})
	require.NoError(t, err)
	assert.Equal(t, []string{"python", "github"}, result.Domains)
	assert.False(t, result.Disabled)
}

func TestApplyOverrides_All(t *testing.T) {
	result, err := ApplyOverrides("all", []string{"python"})
	require.NoError(t, err)
	assert.True(t, result.Disabled)
}

func TestApplyOverrides_Add(t *testing.T) {
	result, err := ApplyOverrides("+rust", []string{"python", "github"})
	require.NoError(t, err)
	assert.Equal(t, []string{"python", "github", "rust"}, result.Domains)
}

func TestApplyOverrides_AddDuplicate(t *testing.T) {
	result, err := ApplyOverrides("+python", []string{"python", "github"})
	require.NoError(t, err)
	assert.Equal(t, []string{"python", "github"}, result.Domains)
}

func TestApplyOverrides_Remove(t *testing.T) {
	result, err := ApplyOverrides("-nodejs", []string{"python", "github", "nodejs"})
	require.NoError(t, err)
	assert.Equal(t, []string{"python", "github"}, result.Domains)
}

func TestApplyOverrides_AddAndRemove(t *testing.T) {
	result, err := ApplyOverrides("+rust,-nodejs", []string{"python", "github", "nodejs"})
	require.NoError(t, err)
	assert.Equal(t, []string{"python", "github", "rust"}, result.Domains)
}

func TestApplyOverrides_Replace(t *testing.T) {
	result, err := ApplyOverrides("vertexai,rust", []string{"python", "github"})
	require.NoError(t, err)
	assert.Equal(t, []string{"vertexai", "rust"}, result.Domains)
}

func TestApplyOverrides_EmptyAdd(t *testing.T) {
	_, err := ApplyOverrides("+", []string{"python"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty group name")
}

func TestApplyOverrides_MixedBareAndModifiers(t *testing.T) {
	_, err := ApplyOverrides("+rust,python", []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mixed bare")
}

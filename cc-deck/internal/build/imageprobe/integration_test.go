//go:build integration

package imageprobe

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_ProbeFedora41(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tools := []ProbeToolEntry{
		{Name: "git"},
		{Name: "curl"},
		{Name: "bash"},
	}

	result, err := RunProbe(ctx, "registry.fedoraproject.org/fedora:41", tools)
	require.NoError(t, err)

	assert.Equal(t, "fedora", result.OS.ID)
	assert.Equal(t, "dnf", result.PackageManager)

	git, ok := result.Tools["git"]
	require.True(t, ok, "git should be found in Fedora image")
	assert.True(t, git.Present)
	assert.NotEmpty(t, git.Version)

	assert.Greater(t, result.DurationMS, int64(0))
}

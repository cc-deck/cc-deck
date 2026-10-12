package share

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLabelGeneratorRetriesCollisions(t *testing.T) {
	generator := NewLabelGenerator(bytes.NewReader([]byte{0, 0, 1, 1}))
	got, err := generator.Next(map[string]bool{"brave-otter": true})
	require.NoError(t, err)
	require.Equal(t, "bright-panda", got)
}

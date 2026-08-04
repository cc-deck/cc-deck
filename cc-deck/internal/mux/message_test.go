package mux

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDedupKey_Deterministic(t *testing.T) {
	m := Message{SessionName: "sess1", PipeName: "cc-deck:hook", Args: `{"event":"test"}`}
	key1 := m.DedupKey()
	key2 := m.DedupKey()
	assert.Equal(t, key1, key2)
	assert.Len(t, key1, 16)
}

func TestDedupKey_DifferentForDifferentMessages(t *testing.T) {
	m1 := Message{SessionName: "sess1", PipeName: "cc-deck:hook", Args: `{"a":1}`}
	m2 := Message{SessionName: "sess2", PipeName: "cc-deck:hook", Args: `{"a":1}`}
	m3 := Message{SessionName: "sess1", PipeName: "other:pipe", Args: `{"a":1}`}
	m4 := Message{SessionName: "sess1", PipeName: "cc-deck:hook", Args: `{"a":2}`}

	keys := map[string]bool{
		m1.DedupKey(): true,
		m2.DedupKey(): true,
		m3.DedupKey(): true,
		m4.DedupKey(): true,
	}
	assert.Len(t, keys, 4, "all messages should produce unique dedup keys")
}

func TestMarshalLine(t *testing.T) {
	m := Message{SessionName: "s", PipeName: "p", Args: "a"}
	line, err := m.MarshalLine()
	require.NoError(t, err)
	assert.Equal(t, byte('\n'), line[len(line)-1])

	var parsed Message
	err = json.Unmarshal(line[:len(line)-1], &parsed)
	require.NoError(t, err)
	assert.Equal(t, m, parsed)
}

func TestUnmarshalMessage(t *testing.T) {
	original := Message{SessionName: "sess", PipeName: "cc-deck:hook", Args: `{"x":1}`}
	data, _ := json.Marshal(original)

	parsed, err := UnmarshalMessage(data)
	require.NoError(t, err)
	assert.Equal(t, original, parsed)
}

func TestUnmarshalMessage_Invalid(t *testing.T) {
	_, err := UnmarshalMessage([]byte("not json"))
	assert.Error(t, err)
}

func TestDedupKey_EmptyFields(t *testing.T) {
	m := Message{}
	key := m.DedupKey()
	assert.Len(t, key, 16)
}

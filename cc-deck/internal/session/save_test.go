package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dumpStateFixture mirrors the plugin's dump-state envelope with a foreground
// Claude row, a daemon-hosted Codex row, and a backgrounded Claude row.
const dumpStateFixture = `{
  "attended_pane_id": 1,
  "sessions": {
    "1": {"pane_id": 1, "session_id": "claude-sess", "display_name": "cc-deck",
          "tab_index": 0, "tab_name": "cc-deck", "agent_name": "claude",
          "profile": "work", "paused": false, "background": false},
    "2": {"pane_id": 2, "session_id": "01a153d8-15e6-7821-8c43-c2866f599728",
          "display_name": "sandcast", "tab_index": 1, "tab_name": "sandcast",
          "agent_name": "codex", "paused": false, "background": false},
    "3": {"pane_id": 3, "session_id": "90691ed7-40ed-460d-9813-6cac3787103a",
          "display_name": "agentmeter", "tab_index": 2, "tab_name": "agentmeter",
          "agent_name": "claude", "profile": "private", "paused": true,
          "background": true}
  }
}`

func TestParseDumpState_DecodesAgentAndBackground(t *testing.T) {
	sessions, err := parseDumpState([]byte(dumpStateFixture))
	require.NoError(t, err)
	require.Len(t, sessions, 3)

	codex := sessions["2"]
	require.NotNil(t, codex.AgentName)
	assert.Equal(t, "codex", *codex.AgentName)
	assert.Equal(t, "01a153d8-15e6-7821-8c43-c2866f599728", codex.SessionID)
	assert.False(t, codex.Background)

	bg := sessions["3"]
	assert.True(t, bg.Background, "backgrounded Claude row must decode background=true")
}

func TestBuildEntries_CapturesCodexAndBackground(t *testing.T) {
	sessions, err := parseDumpState([]byte(dumpStateFixture))
	require.NoError(t, err)

	entries := buildEntries(sessions)
	require.Len(t, entries, 3)

	// Sorted by tab index: cc-deck (0), sandcast (1), agentmeter (2).
	assert.Equal(t, "cc-deck", entries[0].TabName)
	assert.Equal(t, "claude", entries[0].Agent)
	assert.False(t, entries[0].Background)

	assert.Equal(t, "sandcast", entries[1].TabName)
	assert.Equal(t, "codex", entries[1].Agent, "codex session must be captured as agent=codex")
	assert.Equal(t, "01a153d8-15e6-7821-8c43-c2866f599728", entries[1].SessionID)

	assert.Equal(t, "agentmeter", entries[2].TabName)
	assert.True(t, entries[2].Background, "backgrounded Claude row must carry background=true into the snapshot")
	assert.True(t, entries[2].Paused)
}

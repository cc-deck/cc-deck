package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPrintVersion_DefaultFormat(t *testing.T) {
	origVersion, origCommit, origDate := Version, Commit, Date
	defer func() { Version, Commit, Date = origVersion, origCommit, origDate }()

	Version = "1.2.3"
	Commit = "abcdef0123456789"
	Date = "2024-01-02"

	var buf bytes.Buffer
	err := printVersion(&buf, "")
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "cc-deck version 1.2.3")
	// Commit should be truncated to 12 characters.
	assert.Contains(t, out, "abcdef012345")
	assert.NotContains(t, out, "abcdef0123456789")
	assert.Contains(t, out, "2024-01-02")
}

func TestPrintVersion_ShortCommitUnchanged(t *testing.T) {
	origVersion, origCommit, origDate := Version, Commit, Date
	defer func() { Version, Commit, Date = origVersion, origCommit, origDate }()

	Version = "dev"
	Commit = "short"
	Date = "unknown"

	var buf bytes.Buffer
	err := printVersion(&buf, "text")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "commit: short")
}

func TestPrintVersion_JSONFormat(t *testing.T) {
	origVersion, origCommit, origDate := Version, Commit, Date
	defer func() { Version, Commit, Date = origVersion, origCommit, origDate }()

	Version = "1.0.0"
	Commit = "deadbeefcafefeed"
	Date = "2024-05-06"

	var buf bytes.Buffer
	err := printVersion(&buf, "json")
	require.NoError(t, err)

	var info versionInfo
	require.NoError(t, json.Unmarshal(buf.Bytes(), &info))
	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, "deadbeefcafe", info.Commit)
	assert.Equal(t, "2024-05-06", info.Date)
}

func TestPrintVersion_YAMLFormat(t *testing.T) {
	origVersion, origCommit, origDate := Version, Commit, Date
	defer func() { Version, Commit, Date = origVersion, origCommit, origDate }()

	Version = "2.0.0"
	Commit = "0123456789abcdef"
	Date = "2024-07-08"

	var buf bytes.Buffer
	err := printVersion(&buf, "yaml")
	require.NoError(t, err)

	var info versionInfo
	require.NoError(t, yaml.Unmarshal(buf.Bytes(), &info))
	assert.Equal(t, "2.0.0", info.Version)
	assert.Equal(t, "012345678901", info.Commit)
	assert.Equal(t, "2024-07-08", info.Date)
}

func TestNewVersionCmd_Run(t *testing.T) {
	gf := &GlobalFlags{Output: "json"}
	cmd := NewVersionCmd(gf)
	assert.Equal(t, "version", cmd.Use)

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// RunE writes to os.Stdout directly, so just verify the command is wired
	// and executes without error.
	err := cmd.RunE(cmd, nil)
	require.NoError(t, err)
}

func TestPrintVersion_UnknownFormatFallsBackToText(t *testing.T) {
	var buf bytes.Buffer
	err := printVersion(&buf, "xml")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(buf.String(), "cc-deck version "))
}

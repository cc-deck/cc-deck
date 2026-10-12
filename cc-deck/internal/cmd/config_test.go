package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRootCmdForCompletion() *cobra.Command {
	root := &cobra.Command{Use: "cc-deck"}
	root.AddCommand(NewCompletionCmd())
	return root
}

func TestNewCompletionCmd_Metadata(t *testing.T) {
	cmd := NewCompletionCmd()
	assert.Equal(t, "completion [bash|zsh|fish]", cmd.Use)
	assert.ElementsMatch(t, []string{"bash", "zsh", "fish"}, cmd.ValidArgs)
	assert.True(t, cmd.DisableFlagsInUseLine)
}

func TestNewCompletionCmd_InvalidArgRejected(t *testing.T) {
	root := newRootCmdForCompletion()
	root.SetArgs([]string{"completion", "powershell"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	assert.Error(t, err)
}

func TestNewCompletionCmd_NoArgsRejected(t *testing.T) {
	root := newRootCmdForCompletion()
	root.SetArgs([]string{"completion"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	assert.Error(t, err)
}

func TestNewCompletionCmd_Bash(t *testing.T) {
	root := newRootCmdForCompletion()
	root.SetArgs([]string{"completion", "bash"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})

	// GenBashCompletion writes to os.Stdout directly regardless of SetOut,
	// so just assert the command executes without error.
	err := root.Execute()
	require.NoError(t, err)
}

func TestNewCompletionCmd_Zsh(t *testing.T) {
	root := newRootCmdForCompletion()
	root.SetArgs([]string{"completion", "zsh"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	require.NoError(t, err)
}

func TestNewCompletionCmd_Fish(t *testing.T) {
	root := newRootCmdForCompletion()
	root.SetArgs([]string{"completion", "fish"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	require.NoError(t, err)
}

func TestNewConfigCmd_HasSubcommands(t *testing.T) {
	gf := &GlobalFlags{}
	cmd := NewConfigCmd(gf)
	assert.Equal(t, "config", cmd.Use)

	names := make([]string, 0)
	for _, c := range cmd.Commands() {
		names = append(names, c.Name())
	}
	assert.Contains(t, names, "check")
	assert.Contains(t, names, "plugin")
	assert.Contains(t, names, "profile")
	assert.Contains(t, names, "domains")
	assert.Contains(t, names, "completion")
}

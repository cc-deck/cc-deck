package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addTestGroup registers commands under a named group.
func addTestGroup(parent *cobra.Command, groupID string, cmds ...*cobra.Command) {
	for _, c := range cmds {
		c.GroupID = groupID
		parent.AddCommand(c)
	}
}

// buildRootCmd creates a root command with all promoted and ws commands
// registered, matching the structure in main.go.
func buildRootCmd() *cobra.Command {
	gf := &GlobalFlags{}

	root := &cobra.Command{Use: "cc-deck"}

	root.AddGroup(
		&cobra.Group{ID: "workspace", Title: "Workspace:"},
		&cobra.Group{ID: "session", Title: "Session:"},
		&cobra.Group{ID: "build", Title: "Build:"},
		&cobra.Group{ID: "config", Title: "Config:"},
	)

	addTestGroup(root, "workspace",
		NewAttachCmd(gf),
		NewListCmd(gf),
		NewExecCmd(gf),
		NewWsCmd(gf),
	)

	addTestGroup(root, "session",
		NewSnapshotCmd(gf),
	)

	addTestGroup(root, "build",
		NewBuildCmd(gf),
	)

	addTestGroup(root, "config",
		NewConfigCmd(gf),
	)

	// Utility commands (ungrouped).
	root.AddCommand(NewHookCmd())
	root.AddCommand(NewVersionCmd(gf))

	return root
}

func TestPromotedCommands_ExistOnRoot(t *testing.T) {
	root := buildRootCmd()

	tests := []struct {
		name    string
		use     string
		short   string
		aliases []string
	}{
		{"attach", "attach [name]", "Attach to a workspace", nil},
		{"list", "list", "List workspaces", []string{"ls"}},
		{"exec", "exec <name> -- <cmd...>", "Run a command inside a workspace", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, _, err := root.Find([]string{tt.name})
			require.NoError(t, err)
			assert.Equal(t, tt.use, cmd.Use)
			assert.Equal(t, tt.short, cmd.Short)
			if tt.aliases != nil {
				assert.Equal(t, tt.aliases, cmd.Aliases)
			}
		})
	}
}

func TestPromotedCommands_ShareBehaviorWithWs(t *testing.T) {
	gf := &GlobalFlags{}

	pairs := []struct {
		name    string
		promote func(*GlobalFlags) *cobra.Command
		wsSub   string
	}{
		{"attach", NewAttachCmd, "attach"},
		{"list", NewListCmd, "list"},
		{"exec", NewExecCmd, "exec"},
	}

	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			promoted := p.promote(gf)

			wsCmd := NewWsCmd(gf)
			var wsSub *cobra.Command
			for _, c := range wsCmd.Commands() {
				if c.Name() == p.wsSub {
					wsSub = c
					break
				}
			}
			require.NotNil(t, wsSub, "ws subcommand %q not found", p.wsSub)

			assert.Equal(t, promoted.Use, wsSub.Use, "Use mismatch")
			assert.Equal(t, promoted.Short, wsSub.Short, "Short mismatch")
			assert.Equal(t, promoted.Long, wsSub.Long, "Long mismatch")
			assert.Equal(t, len(promoted.Aliases), len(wsSub.Aliases), "Aliases length mismatch")

			// Verify flags match.
			promotedFlags := promoted.Flags().NFlag()
			wsSubFlags := wsSub.Flags().NFlag()
			assert.Equal(t, promotedFlags, wsSubFlags, "flag count mismatch")
		})
	}
}

func TestPromotedCommands_ListAlias(t *testing.T) {
	root := buildRootCmd()

	// "ls" should resolve to the list command.
	cmd, _, err := root.Find([]string{"ls"})
	require.NoError(t, err)
	assert.Equal(t, "list", cmd.Name())
}

func TestHelpOutput_ContainsGroups(t *testing.T) {
	root := buildRootCmd()

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"--help"})
	err := root.Execute()
	require.NoError(t, err)

	help := buf.String()

	groups := []string{"Workspace:", "Session:", "Build:", "Config:"}
	for _, g := range groups {
		assert.Contains(t, help, g, "missing group heading %q", g)
	}

	// Verify promoted commands appear under Workspace.
	wsCmds := []string{"attach", "list", "exec", "ws"}
	for _, c := range wsCmds {
		assert.Contains(t, help, c, "missing workspace command %q in help output", c)
	}

	// Verify build appears.
	assert.Contains(t, help, "build")

	// Verify config appears.
	assert.Contains(t, help, "config")

	// Verify utility commands appear under Additional Commands.
	assert.Contains(t, help, "Additional Commands:")
	assert.Contains(t, help, "version")
}

func TestHelpOutput_CommandPlacement(t *testing.T) {
	root := buildRootCmd()

	tests := []struct {
		name    string
		groupID string
	}{
		{"attach", "workspace"},
		{"list", "workspace"},
		{"exec", "workspace"},
		{"ws", "workspace"},
		{"snapshot", "session"},
		{"build", "build"},
		{"config", "config"},
		{"hook", ""},
		{"version", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, _, err := root.Find([]string{tt.name})
			require.NoError(t, err)
			assert.Equal(t, tt.groupID, cmd.GroupID, "wrong group for %q", tt.name)
		})
	}
}

func TestDualPath_IdenticalBehavior(t *testing.T) {
	root := buildRootCmd()

	promoted := []string{"attach", "list", "exec"}

	for _, name := range promoted {
		t.Run(name, func(t *testing.T) {
			topCmd, _, err := root.Find([]string{name})
			require.NoError(t, err, "top-level %q not found", name)

			wsCmd, _, err := root.Find([]string{"ws", name})
			require.NoError(t, err, "ws %q not found", name)

			assert.Equal(t, topCmd.Use, wsCmd.Use, "Use mismatch for %q", name)
			assert.Equal(t, topCmd.Short, wsCmd.Short, "Short mismatch for %q", name)
			assert.Equal(t, topCmd.Long, wsCmd.Long, "Long mismatch for %q", name)

			assert.Equal(t, topCmd.Aliases, wsCmd.Aliases, "Aliases mismatch for %q", name)
			assert.Equal(t, topCmd.Args == nil, wsCmd.Args == nil, "Args nil mismatch for %q", name)

			topFlagNames := make(map[string]bool)
			topCmd.Flags().VisitAll(func(f *pflag.Flag) {
				topFlagNames[f.Name] = true
			})
			wsFlagNames := make(map[string]bool)
			wsCmd.Flags().VisitAll(func(f *pflag.Flag) {
				wsFlagNames[f.Name] = true
			})
			assert.Equal(t, topFlagNames, wsFlagNames, "flags mismatch for %q", name)
		})
	}
}

func TestDualPath_ShellCompletionIncludesBothPaths(t *testing.T) {
	root := buildRootCmd()

	rootCmds := make(map[string]bool)
	for _, c := range root.Commands() {
		rootCmds[c.Name()] = true
	}

	promoted := []string{"attach", "list", "exec"}
	for _, name := range promoted {
		assert.True(t, rootCmds[name], "promoted command %q not at root level", name)
	}

	assert.True(t, rootCmds["ws"], "ws command not at root level")

	wsCmd, _, err := root.Find([]string{"ws"})
	require.NoError(t, err)

	wsSubCmds := make(map[string]bool)
	for _, c := range wsCmd.Commands() {
		wsSubCmds[c.Name()] = true
	}

	for _, name := range promoted {
		assert.True(t, wsSubCmds[name], "promoted command %q not under ws", name)
	}

	// Verify ws-only commands are NOT at root.
	wsOnly := []string{"new", "delete", "push", "pull", "harvest", "prune", "status", "start", "stop", "logs"}
	for _, name := range wsOnly {
		assert.False(t, rootCmds[name], "ws-only command %q should not be at root level", name)
		assert.True(t, wsSubCmds[name], "ws-only command %q should be under ws", name)
	}
}

func TestWsAlias_Workspace(t *testing.T) {
	root := buildRootCmd()

	cmd, _, err := root.Find([]string{"workspace"})
	require.NoError(t, err)
	assert.Equal(t, "ws", cmd.Name())
}

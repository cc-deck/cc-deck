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

// buildRootCmd creates a root command with all promoted and env commands
// registered, matching the structure in main.go.
func buildRootCmd() *cobra.Command {
	gf := &GlobalFlags{}

	root := &cobra.Command{Use: "cc-deck"}

	root.AddGroup(
		&cobra.Group{ID: "daily", Title: "Daily:"},
		&cobra.Group{ID: "session", Title: "Session:"},
		&cobra.Group{ID: "environment", Title: "Environment:"},
		&cobra.Group{ID: "setup", Title: "Setup:"},
	)

	addTestGroup(root, "daily",
		NewAttachCmd(gf),
		NewListCmd(gf),
		NewStatusCmd(gf),
		NewStartCmd(gf),
		NewStopCmd(gf),
		NewLogsCmd(gf),
	)

	addTestGroup(root, "session",
		NewSnapshotCmd(gf),
	)

	addTestGroup(root, "environment",
		NewEnvCmd(gf),
	)

	addTestGroup(root, "setup",
		NewPluginCmd(gf),
		NewProfileCmd(gf),
		NewDomainsCmd(gf),
		NewSetupCmd(gf),
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
		{"attach", "attach [name]", "Attach to an environment", nil},
		{"list", "list", "List environments", []string{"ls"}},
		{"status", "status [name]", "Show environment status", nil},
		{"start", "start [name]", "Start a stopped environment", nil},
		{"stop", "stop [name]", "Stop a running environment", nil},
		{"logs", "logs <name>", "View environment logs", nil},
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

func TestPromotedCommands_ShareBehaviorWithEnv(t *testing.T) {
	gf := &GlobalFlags{}

	// Verify that promoted and env commands are built from the same
	// constructor by comparing Use and Short fields.
	pairs := []struct {
		name    string
		promote func(*GlobalFlags) *cobra.Command
		envSub  string
	}{
		{"attach", NewAttachCmd, "attach"},
		{"list", NewListCmd, "list"},
		{"status", NewStatusCmd, "status"},
		{"start", NewStartCmd, "start"},
		{"stop", NewStopCmd, "stop"},
		{"logs", NewLogsCmd, "logs"},
	}

	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			promoted := p.promote(gf)

			envCmd := NewEnvCmd(gf)
			var envSub *cobra.Command
			for _, c := range envCmd.Commands() {
				if c.Name() == p.envSub {
					envSub = c
					break
				}
			}
			require.NotNil(t, envSub, "env subcommand %q not found", p.envSub)

			assert.Equal(t, promoted.Use, envSub.Use, "Use mismatch")
			assert.Equal(t, promoted.Short, envSub.Short, "Short mismatch")
			assert.Equal(t, promoted.Long, envSub.Long, "Long mismatch")
			assert.Equal(t, len(promoted.Aliases), len(envSub.Aliases), "Aliases length mismatch")

			// Verify flags match.
			promotedFlags := promoted.Flags().NFlag()
			envSubFlags := envSub.Flags().NFlag()
			assert.Equal(t, promotedFlags, envSubFlags, "flag count mismatch")
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

	// Verify all four group headings appear.
	groups := []string{"Daily:", "Session:", "Environment:", "Setup:"}
	for _, g := range groups {
		assert.Contains(t, help, g, "missing group heading %q", g)
	}

	// Verify promoted commands appear under Daily.
	dailyCmds := []string{"attach", "list", "logs", "start", "status", "stop"}
	for _, c := range dailyCmds {
		assert.Contains(t, help, c, "missing daily command %q in help output", c)
	}

	// Verify env appears under Environment.
	assert.Contains(t, help, "env")

	// Verify setup commands appear.
	setupCmds := []string{"plugin", "profile", "domains", "image"}
	for _, c := range setupCmds {
		assert.Contains(t, help, c, "missing setup command %q in help output", c)
	}

	// Verify utility commands appear under Additional Commands.
	assert.Contains(t, help, "Additional Commands:")
	assert.Contains(t, help, "version")
}

func TestHelpOutput_CommandPlacement(t *testing.T) {
	root := buildRootCmd()

	// Verify GroupID assignments.
	tests := []struct {
		name    string
		groupID string
	}{
		{"attach", "daily"},
		{"list", "daily"},
		{"status", "daily"},
		{"start", "daily"},
		{"stop", "daily"},
		{"logs", "daily"},
		{"snapshot", "session"},
		{"env", "environment"},
		{"plugin", "setup"},
		{"profile", "setup"},
		{"domains", "setup"},
		{"image", "setup"},
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

	promoted := []string{"attach", "list", "status", "start", "stop", "logs"}

	for _, name := range promoted {
		t.Run(name, func(t *testing.T) {
			// Find via top-level path.
			topCmd, _, err := root.Find([]string{name})
			require.NoError(t, err, "top-level %q not found", name)

			// Find via env subcommand path.
			envCmd, _, err := root.Find([]string{"env", name})
			require.NoError(t, err, "env %q not found", name)

			// Verify identical Use, Short, Long.
			assert.Equal(t, topCmd.Use, envCmd.Use, "Use mismatch for %q", name)
			assert.Equal(t, topCmd.Short, envCmd.Short, "Short mismatch for %q", name)
			assert.Equal(t, topCmd.Long, envCmd.Long, "Long mismatch for %q", name)

			// Verify identical aliases.
			assert.Equal(t, topCmd.Aliases, envCmd.Aliases, "Aliases mismatch for %q", name)

			// Verify identical Args validation.
			assert.Equal(t, topCmd.Args == nil, envCmd.Args == nil, "Args nil mismatch for %q", name)

			// Verify identical flag names.
			topFlagNames := make(map[string]bool)
			topCmd.Flags().VisitAll(func(f *pflag.Flag) {
				topFlagNames[f.Name] = true
			})
			envFlagNames := make(map[string]bool)
			envCmd.Flags().VisitAll(func(f *pflag.Flag) {
				envFlagNames[f.Name] = true
			})
			assert.Equal(t, topFlagNames, envFlagNames, "flags mismatch for %q", name)
		})
	}
}

func TestDualPath_ShellCompletionIncludesBothPaths(t *testing.T) {
	root := buildRootCmd()

	// Verify promoted commands are at root level.
	rootCmds := make(map[string]bool)
	for _, c := range root.Commands() {
		rootCmds[c.Name()] = true
	}

	promoted := []string{"attach", "list", "status", "start", "stop", "logs"}
	for _, name := range promoted {
		assert.True(t, rootCmds[name], "promoted command %q not at root level", name)
	}

	// Verify env subcommand exists at root.
	assert.True(t, rootCmds["env"], "env command not at root level")

	// Verify all promoted commands exist as env subcommands.
	envCmd, _, err := root.Find([]string{"env"})
	require.NoError(t, err)

	envSubCmds := make(map[string]bool)
	for _, c := range envCmd.Commands() {
		envSubCmds[c.Name()] = true
	}

	for _, name := range promoted {
		assert.True(t, envSubCmds[name], "promoted command %q not under env", name)
	}

	// Verify env-only commands are NOT at root.
	envOnly := []string{"create", "delete", "exec", "push", "pull", "harvest", "prune"}
	for _, name := range envOnly {
		assert.False(t, rootCmds[name], "env-only command %q should not be at root level", name)
		assert.True(t, envSubCmds[name], "env-only command %q should be under env", name)
	}
}

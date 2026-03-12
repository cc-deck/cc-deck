package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rhuss/cc-mux/cc-deck/internal/build"
)

// NewBuildCmd creates the build parent command with subcommands.
func NewBuildCmd(flags *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Container image build pipeline",
		Long:  "Commands for creating, building, and pushing cc-deck container images.",
	}

	cmd.AddCommand(newBuildInitCmd(flags))

	return cmd
}

func newBuildInitCmd(flags *GlobalFlags) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init <dir>",
		Short: "Initialize a build directory",
		Long: `Scaffold a new build directory with a cc-deck-build.yaml manifest,
Claude Code commands for AI-driven image configuration, and helper scripts.

After initialization, open the directory in Claude Code and use:
  /cc-deck.extract     - Analyze repos for tool dependencies
  /cc-deck.plugin      - Add Claude Code plugins
  /cc-deck.mcp         - Add MCP server sidecars
  /cc-deck.containerfile - Generate the Containerfile`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			if err := build.InitBuildDir(dir, force); err != nil {
				return err
			}
			fmt.Printf("Build directory initialized: %s\n\n", dir)
			fmt.Println("  Manifest:  cc-deck-build.yaml")
			fmt.Println("  Commands:  .claude/commands/cc-deck.*.md")
			fmt.Println("  Scripts:   .claude/scripts/")
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Printf("  cd %s\n", dir)
			fmt.Println("  claude                        # Open in Claude Code")
			fmt.Println("  /cc-deck.extract              # Analyze repositories")
			fmt.Println("  /cc-deck.containerfile        # Generate Containerfile")
			fmt.Printf("  cc-deck build %s              # Build the image\n", dir)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing build directory")

	return cmd
}

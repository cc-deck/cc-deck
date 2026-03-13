package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rhuss/cc-mux/cc-deck/internal/build"
)

// NewImageCmd creates the image parent command with subcommands.
func NewImageCmd(flags *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Container image lifecycle",
		Long:  "Commands for creating, building, pushing, and verifying cc-deck container images.",
	}

	cmd.AddCommand(newImageInitCmd(flags))
	cmd.AddCommand(newImageVerifyCmd(flags))
	cmd.AddCommand(newImageDiffCmd(flags))

	return cmd
}

func newImageInitCmd(_ *GlobalFlags) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init <dir>",
		Short: "Initialize a build directory",
		Long: `Scaffold a new build directory with a cc-deck-build.yaml manifest,
Claude Code commands for AI-driven image configuration, and helper scripts.

After initialization, open the directory in Claude Code and use:
  /cc-deck.extract       - Analyze repos for tool dependencies
  /deck-kit.settings     - Select local settings, plugins, MCP to include
  /cc-deck.build         - Generate Containerfile and build the image
  /cc-deck.push          - Push the image to a registry`,
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
			fmt.Println("  /deck-kit.settings            # Select settings, plugins, MCP")
			fmt.Println("  /cc-deck.build                # Generate Containerfile & build")
			fmt.Println("  /cc-deck.push                 # Push to registry")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing build directory")

	return cmd
}

func newImageVerifyCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "verify <dir>",
		Short: "Smoke-test a built container image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVerify(args[0])
		},
	}
}

func runVerify(dir string) error {
	manifestPath := filepath.Join(dir, "cc-deck-build.yaml")
	m, err := build.LoadManifest(manifestPath)
	if err != nil {
		return err
	}

	runtime, err := build.DetectRuntime()
	if err != nil {
		return err
	}

	imageRef := m.ImageRef()
	fmt.Printf("Verifying image: %s\n\n", imageRef)

	type check struct {
		name    string
		command string
	}

	checks := []check{
		{"cc-deck version", "cc-deck version"},
		{"Claude Code available", "claude --version"},
	}

	for _, tool := range m.Tools {
		parts := strings.Fields(tool)
		if len(parts) > 0 {
			name := strings.ToLower(parts[0])
			switch {
			case strings.Contains(name, "go"):
				checks = append(checks, check{"Go compiler", "go version"})
			case strings.Contains(name, "python"):
				checks = append(checks, check{"Python", "python3 --version"})
			case strings.Contains(name, "node"):
				checks = append(checks, check{"Node.js", "node --version"})
			case strings.Contains(name, "rust"):
				checks = append(checks, check{"Rust", "rustc --version"})
			}
		}
	}

	passed := 0
	failed := 0

	for _, c := range checks {
		verifyCmd := exec.Command(runtime, "run", "--rm", imageRef, "sh", "-c", c.command)
		output, err := verifyCmd.CombinedOutput()
		result := strings.TrimSpace(string(output))
		if err != nil {
			fmt.Printf("  FAIL  %s\n", c.name)
			if result != "" {
				fmt.Printf("        %s\n", result)
			}
			failed++
		} else {
			fmt.Printf("  PASS  %s: %s\n", c.name, result)
			passed++
		}
	}

	fmt.Printf("\n%d passed, %d failed\n", passed, failed)
	if failed > 0 {
		return fmt.Errorf("%d verification checks failed", failed)
	}
	return nil
}

func newImageDiffCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "diff <dir>",
		Short: "Show manifest changes since last Containerfile generation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiff(args[0])
		},
	}
}

func runDiff(dir string) error {
	manifestPath := filepath.Join(dir, "cc-deck-build.yaml")
	m, err := build.LoadManifest(manifestPath)
	if err != nil {
		return err
	}

	containerfilePath := filepath.Join(dir, "Containerfile")
	cfData, err := os.ReadFile(containerfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no Containerfile found, run /cc-deck.containerfile first")
		}
		return err
	}
	cfContent := string(cfData)

	hasChanges := false

	fmt.Println("Tools:")
	for _, tool := range m.Tools {
		toolLower := strings.ToLower(tool)
		words := strings.Fields(toolLower)
		found := false
		for _, w := range words {
			if len(w) > 2 && strings.Contains(strings.ToLower(cfContent), w) {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("  + %s (in manifest, not in Containerfile)\n", tool)
			hasChanges = true
		}
	}

	fmt.Println("\nPlugins:")
	for _, p := range m.Plugins {
		if !strings.Contains(cfContent, p.Name) {
			fmt.Printf("  + %s (%s) (in manifest, not in Containerfile)\n", p.Name, p.Source)
			hasChanges = true
		}
	}

	fmt.Println("\nMCP Servers:")
	for _, mcp := range m.MCP {
		if !strings.Contains(cfContent, mcp.Image) {
			fmt.Printf("  + %s (%s) (in manifest, not referenced)\n", mcp.Name, mcp.Image)
			hasChanges = true
		}
	}

	fmt.Println("\nGitHub Tools:")
	for _, gt := range m.GithubTools {
		if !strings.Contains(cfContent, gt.Repo) {
			fmt.Printf("  + %s (%s) (in manifest, not in Containerfile)\n", gt.Binary, gt.Repo)
			hasChanges = true
		}
	}

	if !hasChanges {
		fmt.Println("\nNo differences detected. Manifest and Containerfile appear in sync.")
	} else {
		fmt.Println("\nRegenerate with: claude /cc-deck.containerfile")
	}

	return nil
}

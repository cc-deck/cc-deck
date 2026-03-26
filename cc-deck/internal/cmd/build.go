package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/build"
	"github.com/cc-deck/cc-deck/internal/project"
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
		Use:   "init [dir]",
		Short: "Initialize a build directory",
		Long: `Scaffold a new build directory with a cc-deck-image.yaml manifest,
Claude Code commands for AI-driven image configuration, and helper scripts.

When no directory is specified and the current project has .cc-deck/,
defaults to .cc-deck/image/ (FR-017).

After initialization, start Claude Code from the project directory and use:
  /cc-deck.capture       - Discover tools and settings for the image
  /cc-deck.build         - Generate Containerfile and build the image
  /cc-deck.push          - Push the image to a registry`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, projectRoot := resolveImageDirAndRoot(args)
			if err := build.InitBuildDir(dir, projectRoot, force); err != nil {
				return err
			}
			fmt.Printf("Build directory initialized: %s\n\n", dir)
			fmt.Printf("  Manifest:  %s/cc-deck-image.yaml\n", dir)
			fmt.Printf("  Commands:  %s/.claude/commands/cc-deck.*.md\n", projectRoot)
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Printf("  cd %s\n", projectRoot)
			fmt.Println("  claude                        # Open in Claude Code")
			fmt.Println("  /cc-deck.capture              # Discover tools and settings")
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
		Use:   "verify [dir]",
		Short: "Smoke-test a built container image",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVerify(resolveImageDir(args))
		},
	}
}

func runVerify(dir string) error {
	manifestPath := filepath.Join(dir, "cc-deck-image.yaml")
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
				// Truncate long error output (e.g. crash stack traces)
				lines := strings.SplitN(result, "\n", 4)
				if len(lines) > 3 {
					for _, line := range lines[:3] {
						fmt.Printf("        %s\n", line)
					}
					fmt.Printf("        ... (output truncated)\n")
				} else {
					fmt.Printf("        %s\n", result)
				}
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
		Use:   "diff [dir]",
		Short: "Show manifest changes since last Containerfile generation",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiff(resolveImageDir(args))
		},
	}
}

// resolveImageDir returns the image directory from args or defaults to
// .cc-deck/image/ if inside a project with .cc-deck/ (FR-017).
// resolveImageDir returns just the image directory (for verify, diff).
func resolveImageDir(args []string) string {
	dir, _ := resolveImageDirAndRoot(args)
	return dir
}

// resolveImageDirAndRoot returns the image build directory and the project
// root. Commands go into projectRoot/.claude/commands/, build artifacts
// into imageDir (.cc-deck/image/).
//
// When no args are given, defaults to .cc-deck/image/ relative to the
// project root (git root or cwd). Image build artifacts always live
// inside .cc-deck/ alongside other project-local cc-deck state.
func resolveImageDirAndRoot(args []string) (imageDir string, projectRoot string) {
	if len(args) > 0 {
		// Explicit dir: use its parent as project root.
		return args[0], filepath.Dir(args[0])
	}
	cwd, err := os.Getwd()
	if err != nil {
		return filepath.Join(".", ".cc-deck", "image"), "."
	}
	// Prefer project config root, then git root, then cwd.
	if root, findErr := project.FindProjectConfig(cwd); findErr == nil {
		return filepath.Join(root, ".cc-deck", "image"), root
	}
	if root, gitErr := project.FindGitRoot(cwd); gitErr == nil {
		return filepath.Join(root, ".cc-deck", "image"), root
	}
	return filepath.Join(cwd, ".cc-deck", "image"), cwd
}

func runDiff(dir string) error {
	manifestPath := filepath.Join(dir, "cc-deck-image.yaml")
	m, err := build.LoadManifest(manifestPath)
	if err != nil {
		return err
	}

	containerfilePath := filepath.Join(dir, "Containerfile")
	cfData, err := os.ReadFile(containerfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no Containerfile found, run /cc-deck.build first")
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
		fmt.Println("\nRegenerate with: claude /cc-deck.build")
	}

	return nil
}

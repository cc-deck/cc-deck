package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/project"
	"github.com/cc-deck/cc-deck/internal/setup"
)

// NewSetupCmd creates the setup parent command with subcommands.
func NewSetupCmd(flags *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup profile lifecycle",
		Long:  "Commands for initializing, verifying, and diffing cc-deck setup profiles.",
	}

	cmd.AddCommand(newSetupInitCmd(flags))
	cmd.AddCommand(newSetupVerifyCmd(flags))
	cmd.AddCommand(newSetupDiffCmd(flags))

	return cmd
}

func newSetupInitCmd(_ *GlobalFlags) *cobra.Command {
	var force bool
	var target string

	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Initialize a setup directory",
		Long: `Scaffold a new setup directory with a cc-deck-setup.yaml manifest
and Claude Code commands for AI-driven environment configuration.

When no directory is specified, defaults to .cc-deck/setup/.

Use --target to pre-configure for specific targets:
  --target container      Uncomment container target section
  --target ssh            Uncomment SSH target section and create role skeletons
  --target container,ssh  Configure both targets

After initialization, start Claude Code from the project directory and use:
  /cc-deck.capture       - Discover tools and settings
  /cc-deck.build         - Build container image or provision SSH target`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, projectRoot := resolveSetupDirAndRoot(args)

			var targets []string
			if target != "" {
				for _, t := range strings.Split(target, ",") {
					t = strings.TrimSpace(t)
					if t != "container" && t != "ssh" {
						return fmt.Errorf("invalid target %q: must be container, ssh, or container,ssh", t)
					}
					targets = append(targets, t)
				}
			}

			if err := setup.InitSetupDir(dir, projectRoot, force, targets); err != nil {
				return err
			}
			fmt.Printf("Setup directory initialized: %s\n\n", dir)
			fmt.Printf("  Manifest:  %s/cc-deck-setup.yaml\n", dir)
			fmt.Printf("  Commands:  %s/.claude/commands/cc-deck.*.md\n", projectRoot)
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Printf("  cd %s\n", projectRoot)
			fmt.Println("  claude                        # Open in Claude Code")
			fmt.Println("  /cc-deck.capture              # Discover tools and settings")
			fmt.Println("  /cc-deck.build --target container  # Build container image")
			fmt.Println("  /cc-deck.build --target ssh        # Provision SSH target")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing setup directory")
	cmd.Flags().StringVar(&target, "target", "", "Comma-separated targets: container, ssh, or container,ssh")

	return cmd
}

func newSetupVerifyCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "verify [dir]",
		Short: "Smoke-test a provisioned target",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVerify(resolveSetupDir(args))
		},
	}
}

func runVerify(dir string) error {
	manifestPath := filepath.Join(dir, "cc-deck-setup.yaml")
	m, err := setup.LoadManifest(manifestPath)
	if err != nil {
		return err
	}

	runtime, err := setup.DetectRuntime()
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

func newSetupDiffCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "diff [dir]",
		Short: "Show manifest changes since last artifact generation",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiff(resolveSetupDir(args))
		},
	}
}

// resolveSetupDir returns just the setup directory (for verify, diff).
func resolveSetupDir(args []string) string {
	dir, _ := resolveSetupDirAndRoot(args)
	return dir
}

// resolveSetupDirAndRoot returns the setup directory and the project root.
// Commands go into projectRoot/.claude/commands/, setup artifacts into
// setupDir (.cc-deck/setup/).
func resolveSetupDirAndRoot(args []string) (setupDir string, projectRoot string) {
	if len(args) > 0 {
		return args[0], filepath.Dir(args[0])
	}
	cwd, err := os.Getwd()
	if err != nil {
		return filepath.Join(".", ".cc-deck", "setup"), "."
	}
	if root, findErr := project.FindProjectConfig(cwd); findErr == nil {
		return filepath.Join(root, ".cc-deck", "setup"), root
	}
	if root, gitErr := project.FindGitRoot(cwd); gitErr == nil {
		return filepath.Join(root, ".cc-deck", "setup"), root
	}
	return filepath.Join(cwd, ".cc-deck", "setup"), cwd
}

func runDiff(dir string) error {
	manifestPath := filepath.Join(dir, "cc-deck-setup.yaml")
	m, err := setup.LoadManifest(manifestPath)
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
		fmt.Println("\nNo differences detected. Manifest and artifacts appear in sync.")
	} else {
		fmt.Println("\nRegenerate with: claude /cc-deck.build")
	}

	return nil
}

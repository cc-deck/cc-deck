package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/project"
	"github.com/cc-deck/cc-deck/internal/build"
	"github.com/cc-deck/cc-deck/internal/ssh"
)

func NewBuildCmd(flags *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Prepare images and provision hosts",
		Long:  "Commands for initializing, verifying, and diffing cc-deck build profiles.",
	}

	cmd.AddCommand(newBuildInitCmd(flags))
	cmd.AddCommand(newBuildRunCmd(flags))
	cmd.AddCommand(newBuildVerifyCmd(flags))
	cmd.AddCommand(newBuildDiffCmd(flags))

	return cmd
}

func newBuildInitCmd(_ *GlobalFlags) *cobra.Command {
	var force bool
	var target string

	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Initialize a build directory",
		Long: `Scaffold a new build directory with a build.yaml manifest
and Claude Code commands for AI-driven workspace configuration.

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
			dir, projectRoot := resolveBuildDirAndRoot(args)

			var targets []string
			if target != "" {
				for _, t := range strings.Split(target, ",") {
					t = strings.TrimSpace(t)
					if t != "container" && t != "ssh" {
						return fmt.Errorf("invalid target %q: must be container, ssh, or container,ssh", t)
					}
					targets = append(targets, t)
				}
			} else {
				targets = []string{"container", "ssh"}
			}

			if err := build.InitSetupDir(dir, projectRoot, force, targets); err != nil {
				return err
			}
			fmt.Printf("Build directory initialized: %s\n\n", dir)
			fmt.Printf("  Manifest:  %s/build.yaml\n", dir)
			fmt.Printf("  Commands:  %s/.claude/commands/cc-deck.*.md\n", projectRoot)
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Printf("  cd %s\n", projectRoot)
			fmt.Println("  claude                        # Open in Claude Code")
			fmt.Println("  /cc-deck.capture              # Discover tools and settings")
			for _, t := range targets {
				switch t {
				case "container":
					fmt.Println("  /cc-deck.build --target container  # Build container image")
				case "ssh":
					fmt.Println("  /cc-deck.build --target ssh        # Provision SSH target")
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing build directory")
	cmd.Flags().StringVar(&target, "target", "", "Targets to scaffold: container, ssh, or container,ssh (default: both)")

	return cmd
}

func newBuildRunCmd(_ *GlobalFlags) *cobra.Command {
	var target string
	var push bool

	cmd := &cobra.Command{
		Use:   "run [dir]",
		Short: "Execute build artifacts",
		Long: `Execute pre-generated build artifacts (Containerfile or Ansible playbooks)
directly from the CLI, without Claude Code involvement.

Target type is auto-detected from artifacts present in the build directory:
  Containerfile only         → container build via podman/docker
  site.yml + inventory.ini   → SSH provisioning via ansible-playbook
  Both present               → use --target to select one

Use --push to push the container image after a successful build.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := resolveBuildDir(args)

			detected, err := detectRunTarget(dir, target)
			if err != nil {
				return err
			}

			if err := validateRunFlags(detected, push); err != nil {
				return err
			}

			switch detected {
			case "container":
				return runContainerBuild(dir, push)
			case "ssh":
				return runSSHProvision(dir)
			default:
				return fmt.Errorf("invalid target %q: must be container or ssh", detected)
			}
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Force target type: container or ssh")
	cmd.Flags().BoolVar(&push, "push", false, "Push image after build (container only)")

	return cmd
}

// detectRunTarget determines the run target from artifacts or explicit flag.
func detectRunTarget(dir string, explicit string) (string, error) {
	if explicit != "" {
		if explicit != "container" && explicit != "ssh" {
			return "", fmt.Errorf("invalid target %q: must be container or ssh", explicit)
		}
		return explicit, nil
	}

	hasContainerfile := fileExists(filepath.Join(dir, "container", "Containerfile"))
	hasSSH := fileExists(filepath.Join(dir, "ssh", "site.yml")) && fileExists(filepath.Join(dir, "ssh", "inventory.ini"))

	switch {
	case hasContainerfile && hasSSH:
		return "", fmt.Errorf("both container and SSH artifacts found; use --target to select one")
	case hasContainerfile:
		return "container", nil
	case hasSSH:
		return "ssh", nil
	default:
		return "", fmt.Errorf("no build artifacts found in %s; run /cc-deck.build to generate them", dir)
	}
}

// validateRunFlags checks flag compatibility with the detected target.
func validateRunFlags(target string, push bool) error {
	if push && target != "container" {
		return fmt.Errorf("--push is only valid for container targets")
	}
	return nil
}

func runContainerBuild(dir string, push bool) error {
	manifestPath := filepath.Join(dir, "build.yaml")
	m, err := build.LoadManifest(manifestPath)
	if err != nil {
		return err
	}

	runtime, err := build.DetectRuntime()
	if err != nil {
		return err
	}

	imageRef := m.ImageRef()
	if imageRef == "" {
		return fmt.Errorf("no container target configured in manifest")
	}

	fmt.Printf("Building image: %s\n", imageRef)

	buildCmd := exec.Command(runtime, "build", "-t", imageRef, "-f", "container/Containerfile", ".")
	buildCmd.Dir = dir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	buildCmd.Stdin = os.Stdin

	if err := buildCmd.Run(); err != nil {
		return exitError(err)
	}

	if push {
		ct := m.Targets.Container
		if ct.Registry == "" {
			return fmt.Errorf("targets.container.registry not set in manifest")
		}
		pushRef := strings.TrimRight(ct.Registry, "/") + "/" + imageRef
		fmt.Printf("Pushing image: %s\n", pushRef)

		tagCmd := exec.Command(runtime, "tag", imageRef, pushRef)
		tagCmd.Stdout = os.Stdout
		tagCmd.Stderr = os.Stderr
		if err := tagCmd.Run(); err != nil {
			return exitError(err)
		}

		pushCmd := exec.Command(runtime, "push", pushRef)
		pushCmd.Stdout = os.Stdout
		pushCmd.Stderr = os.Stderr

		if err := pushCmd.Run(); err != nil {
			return exitError(err)
		}
	}

	return nil
}

func runSSHProvision(dir string) error {
	ansiblePath, err := exec.LookPath("ansible-playbook")
	if err != nil {
		return fmt.Errorf("ansible-playbook not found in PATH; install with: pip install ansible")
	}

	fmt.Println("Running Ansible playbook")

	sshDir := filepath.Join(dir, "ssh")
	cmd := exec.Command(ansiblePath, "-i", "inventory.ini", "site.yml")
	cmd.Dir = sshDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return exitError(err)
	}

	return nil
}

// exitError extracts the exit code from an exec error and returns it as an
// exec.ExitError so the CLI framework can pass it through.
func exitError(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr
	}
	return err
}

func newBuildVerifyCmd(_ *GlobalFlags) *cobra.Command {
	var target string

	cmd := &cobra.Command{
		Use:   "verify [dir]",
		Short: "Smoke-test a provisioned target",
		Long: `Verify that a provisioned target has the expected tools installed.

For container targets, runs checks inside the built image via podman.
For SSH targets, runs checks on the remote host via SSH.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := resolveBuildDir(args)
			switch target {
			case "container":
				return runContainerVerify(dir)
			case "ssh":
				return runSSHVerify(dir)
			case "":
				return fmt.Errorf("--target is required: specify container or ssh")
			default:
				return fmt.Errorf("invalid target %q: must be container or ssh", target)
			}
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Target to verify: container or ssh")
	_ = cmd.MarkFlagRequired("target")

	return cmd
}

func runContainerVerify(dir string) error {
	manifestPath := filepath.Join(dir, "build.yaml")
	m, err := build.LoadManifest(manifestPath)
	if err != nil {
		return err
	}

	runtime, err := build.DetectRuntime()
	if err != nil {
		return err
	}

	imageRef := m.ImageRef()
	if imageRef == "" {
		return fmt.Errorf("no container target configured in manifest")
	}
	fmt.Printf("Verifying image: %s\n\n", imageRef)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	return runChecks(m, func(command string) (string, error) {
		verifyCmd := exec.CommandContext(ctx, runtime, "run", "--rm", imageRef, "sh", "-c", command)
		output, err := verifyCmd.CombinedOutput()
		return strings.TrimSpace(string(output)), err
	})
}

func runSSHVerify(dir string) error {
	manifestPath := filepath.Join(dir, "build.yaml")
	m, err := build.LoadManifest(manifestPath)
	if err != nil {
		return err
	}

	if m.Targets == nil || m.Targets.SSH == nil {
		return fmt.Errorf("no SSH target configured in manifest")
	}

	st := m.Targets.SSH
	host := st.Host
	if st.User != "" && !strings.Contains(host, "@") {
		host = st.User + "@" + host
	}
	port := st.Port
	identity := st.IdentityFile

	client := ssh.NewClient(host, port, identity, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("Verifying SSH target: %s\n\n", host)

	return runChecks(m, func(command string) (string, error) {
		return client.Run(ctx, command)
	})
}

// runChecks executes tool checks using the provided command runner.
func runChecks(m *build.Manifest, run func(command string) (string, error)) error {
	type check struct {
		name    string
		command string
	}

	checks := []check{
		{"cc-deck version", "cc-deck version"},
		{"Claude Code available", "claude --version"},
		{"Zellij available", "zellij --version"},
	}

	for _, tool := range m.Tools {
		name := strings.ToLower(tool.Name)
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

	passed := 0
	failed := 0

	for _, c := range checks {
		result, err := run(c.command)
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

func newBuildDiffCmd(_ *GlobalFlags) *cobra.Command {
	var target string

	cmd := &cobra.Command{
		Use:   "diff [dir]",
		Short: "Show manifest changes since last artifact generation",
		Long: `Compare the current manifest against generated artifacts and report drift.

For container targets, compares against the Containerfile.
For SSH targets, compares against Ansible role task files.
If --target is omitted, auto-detects from existing artifacts.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := resolveBuildDir(args)
			return runDiff(dir, target)
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Target to diff: container or ssh (auto-detected if omitted)")

	return cmd
}

// resolveBuildDir returns just the setup directory (for verify, diff).
func resolveBuildDir(args []string) string {
	dir, _ := resolveBuildDirAndRoot(args)
	return dir
}

// resolveBuildDirAndRoot returns the setup directory and the project root.
// Commands go into projectRoot/.claude/commands/, setup artifacts into
// setupDir (.cc-deck/setup/).
func resolveBuildDirAndRoot(args []string) (setupDir string, projectRoot string) {
	if len(args) > 0 {
		// Explicit dir: the project root is two levels up from .cc-deck/setup/
		// (or the parent of the dir if it's not the conventional path).
		dir := args[0]
		parent := filepath.Dir(dir)
		if filepath.Base(parent) == ".cc-deck" {
			return dir, filepath.Dir(parent)
		}
		return dir, parent
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

func runDiff(dir string, target string) error {
	manifestPath := filepath.Join(dir, "build.yaml")
	m, err := build.LoadManifest(manifestPath)
	if err != nil {
		return err
	}

	// Auto-detect target if not specified
	if target == "" {
		hasContainerfile := fileExists(filepath.Join(dir, "container", "Containerfile"))
		hasRoles := fileExists(filepath.Join(dir, "ssh", "roles"))
		switch {
		case hasContainerfile && hasRoles:
			// Both exist, diff both
			var errs []error
			fmt.Println("=== Container Target ===")
			if err := diffContainer(dir, m); err != nil {
				errs = append(errs, fmt.Errorf("container: %w", err))
			}
			fmt.Println("\n=== SSH Target ===")
			if err := diffSSH(dir, m); err != nil {
				errs = append(errs, fmt.Errorf("ssh: %w", err))
			}
			return errors.Join(errs...)
		case hasContainerfile:
			target = "container"
		case hasRoles:
			target = "ssh"
		default:
			return fmt.Errorf("no artifacts found. Run /cc-deck.build first")
		}
	}

	switch target {
	case "container":
		return diffContainer(dir, m)
	case "ssh":
		return diffSSH(dir, m)
	default:
		return fmt.Errorf("invalid target %q: must be container or ssh", target)
	}
}

func diffContainer(dir string, m *build.Manifest) error {
	containerfilePath := filepath.Join(dir, "container", "Containerfile")
	cfData, err := os.ReadFile(containerfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no Containerfile found, run /cc-deck.build --target container first")
		}
		return err
	}
	cfContent := string(cfData)

	hasChanges := false

	fmt.Println("Tools:")
	for _, tool := range m.PackageTools() {
		toolLower := strings.ToLower(tool.Name)
		words := strings.Fields(toolLower)
		found := false
		for _, w := range words {
			if len(w) > 2 && strings.Contains(strings.ToLower(cfContent), w) {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("  + %s (in manifest, not in Containerfile)\n", tool.Name)
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

	fmt.Println("\nGitHub Release Tools:")
	for _, gt := range m.GithubReleaseTools() {
		if !strings.Contains(cfContent, gt.Repo) {
			fmt.Printf("  + %s (%s) (in manifest, not in Containerfile)\n", gt.Name, gt.Repo)
			hasChanges = true
		}
	}

	if !hasChanges {
		fmt.Println("\nNo differences detected. Manifest and Containerfile appear in sync.")
	} else {
		fmt.Println("\nRegenerate with: claude /cc-deck.build --target container")
	}

	return nil
}

func diffSSH(dir string, m *build.Manifest) error {
	rolesDir := filepath.Join(dir, "ssh", "roles")
	if _, err := os.Stat(rolesDir); os.IsNotExist(err) {
		return fmt.Errorf("no ssh/roles/ directory found, run /cc-deck.build --target ssh first")
	}

	hasChanges := false

	// Check tools role for manifest tools and github tools
	toolsTaskFile := filepath.Join(rolesDir, "tools", "tasks", "main.yml")
	toolsContent, toolsErr := os.ReadFile(toolsTaskFile)
	if toolsErr != nil && !os.IsNotExist(toolsErr) {
		fmt.Printf("WARNING: could not read %s: %v\n", toolsTaskFile, toolsErr)
	}
	if toolsErr == nil {
		fmt.Println("Tools:")
		for _, tool := range m.PackageTools() {
			toolLower := strings.ToLower(tool.Name)
			words := strings.Fields(toolLower)
			found := false
			for _, w := range words {
				if len(w) > 2 && strings.Contains(strings.ToLower(string(toolsContent)), w) {
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("  + %s (in manifest, not in roles/tools)\n", tool.Name)
				hasChanges = true
			}
		}

		fmt.Println("\nGitHub Release Tools:")
		for _, gt := range m.GithubReleaseTools() {
			if !strings.Contains(string(toolsContent), gt.Repo) {
				fmt.Printf("  + %s (%s) (in manifest, not in roles/tools)\n", gt.Name, gt.Repo)
				hasChanges = true
			}
		}
	}

	// Check plugins
	ccDeckTaskFile := filepath.Join(rolesDir, "cc_deck", "tasks", "main.yml")
	ccDeckContent, ccDeckErr := os.ReadFile(ccDeckTaskFile)
	if ccDeckErr != nil && !os.IsNotExist(ccDeckErr) {
		fmt.Printf("WARNING: could not read %s: %v\n", ccDeckTaskFile, ccDeckErr)
	}
	if ccDeckErr == nil {
		fmt.Println("\nPlugins:")
		for _, p := range m.Plugins {
			if !strings.Contains(string(ccDeckContent), p.Name) {
				fmt.Printf("  + %s (%s) (in manifest, not in roles/cc_deck)\n", p.Name, p.Source)
				hasChanges = true
			}
		}
	}

	if !hasChanges {
		fmt.Println("\nNo differences detected. Manifest and role tasks appear in sync.")
	} else {
		fmt.Println("\nRegenerate with: claude /cc-deck.build --target ssh")
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

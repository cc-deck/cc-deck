package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/network"
)

// NewDomainsCmd creates the domains command with subcommands.
func NewDomainsCmd(_ *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domains",
		Short: "Manage domain groups for network filtering",
		Long: `Manage domain groups used for network filtering in containerized sessions.

Domain groups are named collections of domain patterns (e.g., "python" includes
pypi.org and related hosts). Groups can be built-in, user-defined, or extended.`,
	}

	cmd.AddCommand(newDomainsInitCmd())
	cmd.AddCommand(newDomainsListCmd())
	cmd.AddCommand(newDomainsShowCmd())
	cmd.AddCommand(newDomainsBlockedCmd())
	cmd.AddCommand(newDomainsAddCmd())
	cmd.AddCommand(newDomainsRemoveCmd())

	return cmd
}

func newDomainsInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Seed domains.yaml with built-in group definitions",
		Long: `Create or update ~/.config/cc-deck/domains.yaml with commented
built-in domain group definitions as a starting point for customization.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDomainsInit(force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing file")
	return cmd
}

func runDomainsInit(force bool) error {
	configPath := network.UserConfigPath()

	// Check if file exists
	if _, err := os.Stat(configPath); err == nil && !force {
		fmt.Fprintf(os.Stderr, "File already exists: %s\n", configPath)
		fmt.Fprintf(os.Stderr, "Use --force to overwrite, or edit the file directly.\n")
		return fmt.Errorf("domains.yaml already exists (use --force to overwrite)")
	}

	// Generate commented built-in definitions
	var sb strings.Builder
	sb.WriteString("# cc-deck domain groups configuration\n")
	sb.WriteString("# Uncomment and modify groups to customize network filtering.\n")
	sb.WriteString("#\n")
	sb.WriteString("# To extend a built-in group (add domains without replacing):\n")
	sb.WriteString("#   python:\n")
	sb.WriteString("#     extends: builtin\n")
	sb.WriteString("#     domains:\n")
	sb.WriteString("#       - pypi.internal.corp\n")
	sb.WriteString("#\n")
	sb.WriteString("# To override a built-in group (replace entirely):\n")
	sb.WriteString("#   python:\n")
	sb.WriteString("#     domains:\n")
	sb.WriteString("#       - pypi.internal.corp\n")
	sb.WriteString("#\n")
	sb.WriteString("# To create a custom group with includes:\n")
	sb.WriteString("#   dev-stack:\n")
	sb.WriteString("#     includes:\n")
	sb.WriteString("#       - python\n")
	sb.WriteString("#       - golang\n")
	sb.WriteString("#     domains:\n")
	sb.WriteString("#       - artifacts.internal.corp\n")
	sb.WriteString("\n")

	// Load built-in groups via resolver
	resolver := network.NewResolver(nil)
	for _, name := range network.BuiltinGroupNames() {
		domains, err := resolver.ExpandGroup(name)
		if err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("# %s:\n", name))
		sb.WriteString("#   domains:\n")
		for _, d := range domains {
			sb.WriteString(fmt.Sprintf("#     - %s\n", d))
		}
		sb.WriteString("\n")
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if err := os.WriteFile(configPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("writing domains.yaml: %w", err)
	}

	fmt.Printf("Created %s\n", configPath)
	return nil
}

func newDomainsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all available domain groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDomainsList()
		},
	}
}

func runDomainsList() error {
	userGroups, err := network.LoadUserConfig()
	if err != nil {
		return fmt.Errorf("loading domain config: %w", err)
	}

	resolver := network.NewResolver(userGroups)
	names := resolver.AllGroupNames()

	fmt.Printf("%-16s %-10s %s\n", "GROUP", "SOURCE", "DOMAINS")
	for _, name := range names {
		source := resolver.GroupSource(name)
		domains, err := resolver.ExpandGroup(name)
		if err != nil {
			fmt.Printf("%-16s %-10s (error: %v)\n", name, source, err)
			continue
		}
		fmt.Printf("%-16s %-10s %d\n", name, source, len(domains))
	}

	return nil
}

func newDomainsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <group>",
		Short: "Show expanded domains for a group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDomainsShow(args[0])
		},
	}
}

func runDomainsShow(groupName string) error {
	userGroups, err := network.LoadUserConfig()
	if err != nil {
		return fmt.Errorf("loading domain config: %w", err)
	}

	resolver := network.NewResolver(userGroups)
	source := resolver.GroupSource(groupName)
	if source == "" {
		return fmt.Errorf("unknown group %q; available groups: %s",
			groupName, strings.Join(resolver.AllGroupNames(), ", "))
	}

	domains, err := resolver.ExpandGroup(groupName)
	if err != nil {
		return err
	}

	fmt.Printf("Group: %s (%s)\n\n", groupName, source)
	for _, d := range domains {
		fmt.Printf("  %s\n", d)
	}

	return nil
}

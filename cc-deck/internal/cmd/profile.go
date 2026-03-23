package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/cc-deck/cc-deck/internal/config"
)

// NewProfileCmd creates the profile cobra command with subcommands.
func NewProfileCmd(globalFlags *GlobalFlags) *cobra.Command {
	profileCmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage credential profiles",
		Long:  "Create, list, switch, and inspect credential profiles for AI backends.",
	}

	profileCmd.AddCommand(
		newProfileAddCmd(globalFlags),
		newProfileListCmd(globalFlags),
		newProfileUseCmd(globalFlags),
		newProfileShowCmd(globalFlags),
	)

	return profileCmd
}

func newProfileAddCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "add <name>",
		Short: "Add a credential profile",
		Long: `Interactively create a new credential profile.

Prompts for backend type (anthropic or vertex), credential references,
and optional settings like model.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileAdd(args[0], gf)
		},
	}
}

func newProfileListCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all credential profiles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileList(gf)
		},
	}
}

func newProfileUseCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the default credential profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileUse(args[0], gf)
		},
	}
}

func newProfileShowCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show details of a credential profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileShow(args[0], gf)
		},
	}
}

func runProfileAdd(name string, gf *GlobalFlags) error {
	cfg, err := config.Load(gf.ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Check if profile already exists
	if _, err := cfg.GetProfile(name); err == nil {
		return fmt.Errorf("profile %q already exists (delete it first or choose a different name)", name)
	}

	// Interactive prompt
	profile, err := config.PromptProfile(os.Stdin, os.Stdout)
	if err != nil {
		return fmt.Errorf("creating profile: %w", err)
	}

	if err := cfg.AddProfile(name, profile); err != nil {
		return fmt.Errorf("adding profile: %w", err)
	}

	// Set as default if it's the first profile
	if len(cfg.Profiles) == 1 {
		cfg.DefaultProfile = name
		fmt.Fprintf(os.Stdout, "Set %q as default profile.\n", name)
	}

	if err := cfg.Save(gf.ConfigFile); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Profile %q added.\n", name)
	return nil
}

// profileListEntry is used for JSON/YAML serialization of profile list output.
type profileListEntry struct {
	Name    string `json:"name" yaml:"name"`
	Backend string `json:"backend" yaml:"backend"`
	Default bool   `json:"default" yaml:"default"`
}

func runProfileList(gf *GlobalFlags) error {
	cfg, err := config.Load(gf.ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	names := cfg.ListProfiles()
	if len(names) == 0 {
		fmt.Fprintln(os.Stdout, "No profiles configured. Run 'cc-deck profile add <name>' to create one.")
		return nil
	}

	switch gf.Output {
	case "json":
		entries := make([]profileListEntry, 0, len(names))
		for _, name := range names {
			p := cfg.Profiles[name]
			entries = append(entries, profileListEntry{
				Name:    name,
				Backend: string(p.Backend),
				Default: name == cfg.DefaultProfile,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)

	case "yaml":
		entries := make([]profileListEntry, 0, len(names))
		for _, name := range names {
			p := cfg.Profiles[name]
			entries = append(entries, profileListEntry{
				Name:    name,
				Backend: string(p.Backend),
				Default: name == cfg.DefaultProfile,
			})
		}
		return yaml.NewEncoder(os.Stdout).Encode(entries)

	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tBACKEND\tDEFAULT")
		for _, name := range names {
			p := cfg.Profiles[name]
			defaultMarker := ""
			if name == cfg.DefaultProfile {
				defaultMarker = "*"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", name, p.Backend, defaultMarker)
		}
		return w.Flush()
	}
}

func runProfileUse(name string, gf *GlobalFlags) error {
	cfg, err := config.Load(gf.ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := cfg.SetDefaultProfile(name); err != nil {
		return err
	}

	if err := cfg.Save(gf.ConfigFile); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Default profile set to %q.\n", name)
	return nil
}

func runProfileShow(name string, gf *GlobalFlags) error {
	cfg, err := config.Load(gf.ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	p, err := cfg.GetProfile(name)
	if err != nil {
		return err
	}

	defaultMarker := ""
	if name == cfg.DefaultProfile {
		defaultMarker = " (default)"
	}

	fmt.Fprintf(os.Stdout, "Profile: %s%s\n", name, defaultMarker)
	fmt.Fprintf(os.Stdout, "  Backend:  %s\n", p.Backend)

	switch p.Backend {
	case config.BackendAnthropic:
		fmt.Fprintf(os.Stdout, "  API Key Secret:  %s\n", p.APIKeySecret)
	case config.BackendVertex:
		fmt.Fprintf(os.Stdout, "  Project:  %s\n", p.Project)
		fmt.Fprintf(os.Stdout, "  Region:   %s\n", p.Region)
		if p.CredentialsSecret != "" {
			fmt.Fprintf(os.Stdout, "  Credentials Secret:  %s\n", p.CredentialsSecret)
		} else {
			fmt.Fprintf(os.Stdout, "  Credentials:  Workload Identity\n")
		}
	}

	if p.Model != "" {
		fmt.Fprintf(os.Stdout, "  Model:    %s\n", p.Model)
	}
	if p.Permissions != "" {
		fmt.Fprintf(os.Stdout, "  Permissions:  %s\n", p.Permissions)
	}
	if len(p.AllowedEgress) > 0 {
		fmt.Fprintf(os.Stdout, "  Allowed Egress:  %v\n", p.AllowedEgress)
	}
	if p.GitCredentialType != "" {
		fmt.Fprintf(os.Stdout, "  Git Credential Type:  %s\n", p.GitCredentialType)
		fmt.Fprintf(os.Stdout, "  Git Credential Secret:  %s\n", p.GitCredentialSecret)
	}

	return nil
}

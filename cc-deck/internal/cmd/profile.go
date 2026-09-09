package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/profile"
	"github.com/cc-deck/cc-deck/internal/ws"
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
		newProfileDeleteCmd(globalFlags),
		newProfileSyncCmd(globalFlags),
	)

	return profileCmd
}

// profileAddFlags holds the flag values for the profile add command.
type profileAddFlags struct {
	harness         string
	backend         string
	model           string
	apiKeyEnv       string
	apiKeyFile      string
	credentialsFile string
	login           bool
	project         string
	region          string
	env             []string
	color           string
	icon            string
}

func newProfileAddCmd(gf *GlobalFlags) *cobra.Command {
	var flags profileAddFlags
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a credential profile",
		Long: `Create a new credential profile.

When flags are provided, the profile is created non-interactively.
Without flags, prompts interactively for each setting.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileAdd(args[0], gf, &flags)
		},
	}
	cmd.Flags().StringVar(&flags.harness, "harness", "", "harness type (claude, codex, opencode)")
	cmd.Flags().StringVar(&flags.backend, "backend", "", "backend type (anthropic, vertex, openai)")
	cmd.Flags().StringVar(&flags.model, "model", "", "model name")
	cmd.Flags().StringVar(&flags.apiKeyEnv, "api-key-env", "", "environment variable holding the API key")
	cmd.Flags().StringVar(&flags.apiKeyFile, "api-key-file", "", "file path holding the API key")
	cmd.Flags().StringVar(&flags.credentialsFile, "credentials-file", "", "path to credentials file (vertex ADC JSON)")
	cmd.Flags().BoolVar(&flags.login, "login", false, "use browser login for authentication")
	cmd.Flags().StringVar(&flags.project, "project", "", "GCP project ID (vertex)")
	cmd.Flags().StringVar(&flags.region, "region", "", "GCP region (vertex)")
	cmd.Flags().StringSliceVar(&flags.env, "env", nil, "extra environment variables as K=V (repeatable)")
	cmd.Flags().StringVar(&flags.color, "color", "", "profile color as #RRGGBB")
	cmd.Flags().StringVar(&flags.icon, "icon", "", "profile icon (single glyph)")
	return cmd
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

// newProfileUseCmd sets the default_profile in config.yaml. This controls
// which profile is used for Kubernetes deploy default selection and git
// credential resolution. It does not affect wrappers (FR-024): each wrapper
// is a standalone script that always uses its own profile's credentials.
func newProfileUseCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the default credential profile",
		Long: `Set the default credential profile in config.yaml.

This controls which profile is used for Kubernetes deploy default selection
and git credential resolution. It does not affect profile wrappers; each
wrapper is a standalone script that uses its own credentials.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileUse(args[0], gf)
		},
	}
}

func newProfileDeleteCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a credential profile",
		Long: `Delete a credential profile from config.yaml.

If the deleted profile is the default, default_profile is cleared.
Run 'cc-deck config profile sync' afterward to remove the wrapper script.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileDelete(args[0], gf)
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

func runProfileAdd(name string, gf *GlobalFlags, flags *profileAddFlags) error {
	cfg, err := config.Load(gf.ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Check if profile already exists
	if _, err := cfg.GetProfile(name); err == nil {
		return fmt.Errorf("profile %q already exists (delete it first or choose a different name)", name)
	}

	var p config.Profile
	if flags.hasAnyFlag() {
		// Non-interactive: build profile from flags.
		p, err = flags.buildProfile()
		if err != nil {
			return fmt.Errorf("invalid flags: %w", err)
		}
	} else {
		// Interactive prompt
		p, err = config.PromptProfile(os.Stdin, os.Stdout)
		if err != nil {
			return fmt.Errorf("creating profile: %w", err)
		}
	}

	if err := cfg.AddProfile(name, p); err != nil {
		return fmt.Errorf("adding profile: %w", err)
	}

	// Cross-profile validation via Config.Validate().
	findings := cfg.Validate()
	for _, f := range findings {
		if f.Severity == "error" {
			// Roll back: remove the profile we just added.
			delete(cfg.Profiles, name)
			return fmt.Errorf("validation error: %s", f.Message)
		}
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

// hasAnyFlag returns true when any profile add flag was explicitly set.
func (f *profileAddFlags) hasAnyFlag() bool {
	return f.harness != "" || f.backend != "" || f.model != "" ||
		f.apiKeyEnv != "" || f.apiKeyFile != "" || f.credentialsFile != "" ||
		f.login || f.project != "" || f.region != "" ||
		len(f.env) > 0 || f.color != "" || f.icon != ""
}

// buildProfile constructs a config.Profile from the flag values.
func (f *profileAddFlags) buildProfile() (config.Profile, error) {
	p := config.Profile{
		Harness: f.harness,
		Model:   f.model,
		Project: f.project,
		Region:  f.region,
		Color:   f.color,
		Icon:    f.icon,
	}

	if f.backend != "" {
		p.Backend = config.BackendType(f.backend)
	}

	// Build auth config from flags.
	authCount := 0
	if f.apiKeyEnv != "" {
		authCount++
	}
	if f.apiKeyFile != "" {
		authCount++
	}
	if f.login {
		authCount++
	}
	if authCount > 1 {
		return p, fmt.Errorf("specify at most one of --api-key-env, --api-key-file, or --login")
	}

	if authCount > 0 || f.credentialsFile != "" {
		p.Auth = &config.AuthConfig{}
		if f.apiKeyEnv != "" {
			p.Auth.APIKey = &config.CredentialSource{Env: f.apiKeyEnv}
		}
		if f.apiKeyFile != "" {
			p.Auth.APIKey = &config.CredentialSource{File: f.apiKeyFile}
		}
		if f.login {
			p.Auth.Login = true
		}
		if f.credentialsFile != "" {
			p.Auth.Credentials = &config.CredentialSource{File: f.credentialsFile}
		}
	}

	// Parse env K=V pairs.
	if len(f.env) > 0 {
		p.Env = make(map[string]string, len(f.env))
		for _, kv := range f.env {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) != 2 {
				return p, fmt.Errorf("invalid --env value %q (expected K=V)", kv)
			}
			p.Env[parts[0]] = parts[1]
		}
	}

	return p, nil
}

// profileListEntry is used for JSON/YAML serialization of profile list output.
type profileListEntry struct {
	Name    string `json:"name" yaml:"name"`
	Harness string `json:"harness" yaml:"harness"`
	Backend string `json:"backend" yaml:"backend"`
	Auth    string `json:"auth" yaml:"auth"`
	Model   string `json:"model,omitempty" yaml:"model,omitempty"`
	Default bool   `json:"default" yaml:"default"`
}

// authSummary returns a short description of the profile's auth configuration.
func authSummary(p config.Profile) string {
	auth := p.EffectiveAuth()
	if auth.Login {
		return "login"
	}
	if auth.APIKey != nil {
		switch auth.APIKey.Kind() {
		case config.SourceEnv:
			return "env:" + auth.APIKey.Env
		case config.SourceFile:
			return "file"
		case config.SourceSecret:
			return "secret"
		}
	}
	if auth.Credentials != nil {
		switch auth.Credentials.Kind() {
		case config.SourceFile:
			return "file"
		case config.SourceSecret:
			return "secret"
		}
	}
	return ""
}

func runProfileList(gf *GlobalFlags) error {
	cfg, err := config.Load(gf.ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	names := cfg.ListProfiles()
	if len(names) == 0 {
		fmt.Fprintln(os.Stdout, "No profiles configured. Run 'cc-deck config profile add <name>' to create one.")
		return nil
	}

	buildEntry := func(name string) profileListEntry {
		p := cfg.Profiles[name]
		return profileListEntry{
			Name:    name,
			Harness: p.HarnessName(),
			Backend: string(p.EffectiveBackend()),
			Auth:    authSummary(p),
			Model:   p.Model,
			Default: name == cfg.DefaultProfile,
		}
	}

	switch gf.Output {
	case "json":
		entries := make([]profileListEntry, 0, len(names))
		for _, name := range names {
			entries = append(entries, buildEntry(name))
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)

	case "yaml":
		entries := make([]profileListEntry, 0, len(names))
		for _, name := range names {
			entries = append(entries, buildEntry(name))
		}
		return yaml.NewEncoder(os.Stdout).Encode(entries)

	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tHARNESS\tBACKEND\tAUTH\tMODEL\tDEFAULT")
		for _, name := range names {
			e := buildEntry(name)
			defaultMarker := ""
			if e.Default {
				defaultMarker = "*"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", e.Name, e.Harness, e.Backend, e.Auth, e.Model, defaultMarker)
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

func runProfileDelete(name string, gf *GlobalFlags) error {
	cfg, err := config.Load(gf.ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := cfg.DeleteProfile(name); err != nil {
		return err
	}

	if err := cfg.Save(gf.ConfigFile); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Profile %q deleted.\n", name)
	fmt.Fprintln(os.Stderr, "Run 'cc-deck config profile sync' to remove the wrapper script.")
	return nil
}

func newProfileSyncCmd(gf *GlobalFlags) *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Render profile wrappers and update shell PATH",
		Long: `Render one wrapper script per valid profile into ~/.local/share/cc-deck/bin,
prepare per-profile config directories with hooks, and ensure the bin
directory is on PATH via managed blocks in .bashrc and .zshrc.

Profiles that cannot be rendered (harness not installed, secret-only
sources) are skipped with reasons.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if workspace != "" {
				return runProfileSyncWorkspace(workspace, gf)
			}
			return runProfileSync(gf)
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "target workspace for remote provisioning (SSH or OpenShell)")
	return cmd
}

func runProfileSync(gf *GlobalFlags) error {
	cfg, err := config.Load(gf.ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("detecting home directory: %w", err)
	}

	result, err := profile.Sync(cfg, home)
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	for _, name := range result.Written {
		fmt.Fprintf(os.Stdout, "  written: %s\n", name)
	}
	for _, name := range result.Removed {
		fmt.Fprintf(os.Stdout, "  removed: %s\n", name)
	}
	for _, s := range result.Skipped {
		fmt.Fprintf(os.Stderr, "  skipped: %s (%s)\n", s.Profile, s.Reason)
	}
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "  warning: %s\n", w)
	}

	if result.RCChanged {
		fmt.Fprintln(os.Stderr, "\nShell rc files updated. Open a new terminal or run:")
		fmt.Fprintln(os.Stderr, "  source ~/.bashrc  # or source ~/.zshrc")
	}

	if len(result.Written) == 0 && len(result.Removed) == 0 {
		fmt.Fprintln(os.Stdout, "Already up to date.")
	}

	return nil
}

func runProfileSyncWorkspace(workspace string, gf *GlobalFlags) error {
	cfg, err := config.Load(gf.ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	ctx := context.Background()
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	target, err := ws.BuildProfileTarget(ctx, workspace, store, defs)
	if err != nil {
		return err
	}

	result, err := profile.Provision(cfg, target)
	if err != nil {
		return fmt.Errorf("provision: %w", err)
	}

	for _, name := range result.Written {
		fmt.Fprintf(os.Stdout, "  written: %s\n", name)
	}
	for _, name := range result.Removed {
		fmt.Fprintf(os.Stdout, "  removed: %s\n", name)
	}
	for _, s := range result.Skipped {
		fmt.Fprintf(os.Stderr, "  skipped: %s (%s)\n", s.Profile, s.Reason)
	}
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "  warning: %s\n", w)
	}

	if result.RCChanged {
		fmt.Fprintln(os.Stderr, "\nShell rc files updated on remote.")
	}

	if len(result.Written) == 0 && len(result.Removed) == 0 {
		fmt.Fprintln(os.Stdout, "Already up to date.")
	}

	// FR-026: For OpenShell workspaces, compare recorded providers with
	// what the current profiles would generate. If profiles were added
	// after workspace creation, their providers will be missing.
	inst, instErr := store.FindInstanceByName(workspace)
	if instErr == nil && inst.Type == ws.WorkspaceTypeOpenShell && inst.OpenShell != nil {
		needed := ws.NeededProfileProviders(cfg, workspace)
		recorded := make(map[string]bool)
		for _, p := range inst.OpenShell.Providers {
			recorded[p] = true
		}
		var missing []string
		for _, n := range needed {
			if !recorded[n] {
				missing = append(missing, n)
			}
		}
		if len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "\nWARNING: %d profile provider(s) not in the workspace's provider list:\n", len(missing))
			for _, m := range missing {
				fmt.Fprintf(os.Stderr, "  - %s\n", m)
			}
			fmt.Fprintf(os.Stderr, "Re-create the workspace to include them:\n")
			fmt.Fprintf(os.Stderr, "  cc-deck ws delete %s && cc-deck ws new %s\n", workspace, workspace)
		}
	}

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
	fmt.Fprintf(os.Stdout, "  Harness:  %s\n", p.HarnessName())
	fmt.Fprintf(os.Stdout, "  Backend:  %s\n", p.EffectiveBackend())

	// Auth sources (references only, never values).
	auth := p.EffectiveAuth()
	if auth.Login {
		fmt.Fprintf(os.Stdout, "  Auth:     login\n")
	}
	if auth.APIKey != nil {
		switch auth.APIKey.Kind() {
		case config.SourceEnv:
			fmt.Fprintf(os.Stdout, "  Auth:     env:%s\n", auth.APIKey.Env)
		case config.SourceFile:
			fmt.Fprintf(os.Stdout, "  Auth:     file:%s\n", auth.APIKey.File)
		case config.SourceSecret:
			fmt.Fprintf(os.Stdout, "  Auth:     secret:%s\n", auth.APIKey.Secret)
		}
	}
	if auth.Credentials != nil {
		switch auth.Credentials.Kind() {
		case config.SourceFile:
			fmt.Fprintf(os.Stdout, "  Credentials:  file:%s\n", auth.Credentials.File)
		case config.SourceSecret:
			fmt.Fprintf(os.Stdout, "  Credentials:  secret:%s\n", auth.Credentials.Secret)
		}
	}

	if p.Project != "" {
		fmt.Fprintf(os.Stdout, "  Project:  %s\n", p.Project)
	}
	if p.Region != "" {
		fmt.Fprintf(os.Stdout, "  Region:   %s\n", p.Region)
	}
	if p.Model != "" {
		fmt.Fprintf(os.Stdout, "  Model:    %s\n", p.Model)
	}
	if len(p.Env) > 0 {
		fmt.Fprintf(os.Stdout, "  Env:\n")
		for k, v := range p.Env {
			fmt.Fprintf(os.Stdout, "    %s=%s\n", k, v)
		}
	}
	if p.Color != "" {
		fmt.Fprintf(os.Stdout, "  Color:    %s\n", p.Color)
	} else {
		fmt.Fprintf(os.Stdout, "  Color:    %s (derived)\n", profile.Derive(name))
	}
	if p.Icon != "" {
		fmt.Fprintf(os.Stdout, "  Icon:     %s\n", p.Icon)
	}

	// Resolved wrapper name.
	a := agent.Get(p.HarnessName())
	if a != nil {
		fmt.Fprintf(os.Stdout, "  Wrapper:  %s-%s\n", a.Binary(), name)
	}

	// Legacy fields (shown when present for backward compat).
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

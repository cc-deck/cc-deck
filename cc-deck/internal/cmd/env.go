package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/env"
	"github.com/cc-deck/cc-deck/internal/project"
)

// NewEnvCmd creates the env parent command with all subcommands.
func NewEnvCmd(gf *GlobalFlags) *cobra.Command {
	envCmd := &cobra.Command{
		Use:   "env",
		Short: "Manage environments",
		Long:  "Create, manage, and interact with cc-deck environments (local, container, Kubernetes).",
	}

	envCmd.AddCommand(
		newEnvInitCmd(gf),
		newEnvCreateCmd(gf),
		newEnvAttachCmd(gf),
		newEnvDeleteCmd(gf),
		newEnvListCmd(gf),
		newEnvStatusCmd(gf),
		newEnvStartCmd(gf),
		newEnvStopCmd(gf),
		newEnvExecCmd(gf),
		newEnvPushCmd(gf),
		newEnvPullCmd(gf),
		newEnvHarvestCmd(gf),
		newEnvLogsCmd(gf),
	)

	return envCmd
}

// --- init ---

type initFlags struct {
	envType        string
	image          string
	auth           string
	allowedDomains []string
	name           string
}

func newEnvInitCmd(_ *GlobalFlags) *cobra.Command {
	var inf initFlags

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize project-local environment definition",
		Long: `Scaffold .cc-deck/environment.yaml and .cc-deck/.gitignore in the current
project. The definition is committed to version control so team members
can run 'cc-deck env create' without specifying flags.

This command does NOT provision any containers or runtime resources.
Use 'cc-deck env create' after init to start the environment.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvInit(&inf)
		},
	}

	cmd.Flags().StringVarP(&inf.envType, "type", "t", "", "Environment type (compose, container) [required]")
	cmd.Flags().StringVar(&inf.image, "image", "", "Container image")
	cmd.Flags().StringVar(&inf.auth, "auth", "auto", "Auth mode: auto, none, api, vertex, bedrock")
	cmd.Flags().StringSliceVar(&inf.allowedDomains, "allowed-domains", nil, "Domain groups for network filtering, repeatable")
	cmd.Flags().StringVar(&inf.name, "name", "", "Environment name (default: directory basename)")
	_ = cmd.MarkFlagRequired("type")

	return cmd
}

func runEnvInit(inf *initFlags) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	// Determine project root and name.
	projectRoot := cwd
	gitRoot, gitErr := project.FindGitRoot(cwd)
	if gitErr == nil {
		projectRoot = gitRoot
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: Not a git repository; .cc-deck/.gitignore will have no effect until git is initialized.\n")
	}

	// Check if definition already exists.
	if _, loadErr := env.LoadProjectDefinition(projectRoot); loadErr == nil {
		return fmt.Errorf(".cc-deck/environment.yaml already exists in %s", projectRoot)
	}

	// Resolve environment name.
	envName := inf.name
	if envName == "" {
		envName = project.ProjectName(projectRoot)
	}
	if err := env.ValidateEnvName(envName); err != nil {
		return err
	}

	// Build definition.
	def := &env.EnvironmentDefinition{
		Name:           envName,
		Type:           env.EnvironmentType(inf.envType),
		Image:          inf.image,
		Auth:           inf.auth,
		AllowedDomains: inf.allowedDomains,
	}

	// Save definition (also creates .cc-deck/.gitignore).
	if err := env.SaveProjectDefinition(projectRoot, def); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Created .cc-deck/environment.yaml in %s\n", projectRoot)
	fmt.Fprintf(os.Stdout, "Commit .cc-deck/ to share the environment definition with your team.\n")
	return nil
}

// --- create ---

type createFlags struct {
	envType        string
	image          string
	ports          []string
	allPorts       bool
	storage        string
	path           string
	credential     []string
	mount          []string
	auth           string
	allowedDomains []string
	gitignore      bool
}

func newEnvCreateCmd(gf *GlobalFlags) *cobra.Command {
	var cf createFlags

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new environment",
		Long: `Create a new cc-deck environment. When run inside a project with
.cc-deck/environment.yaml, the name and settings are read from the
definition automatically. CLI flags override the definition values.

If no definition exists in a git repository, one is scaffolded from
CLI flags before provisioning (equivalent to running env init first).

Environment types:
  local       Zellij session on the host machine (default)
  container   Container environment managed by podman
  compose     Multi-container environment via podman-compose
  k8s-deploy  Kubernetes StatefulSet (not yet implemented)
  k8s-sandbox Ephemeral Kubernetes Pod (not yet implemented)

Container/Compose flags:
  --image            Container image to use
  --port             Port mapping (host:container), repeatable
  --all-ports        Expose all container ports
  --storage          Storage type: named-volume (container default), host-path (compose default)
  --path             Project directory (compose: defaults to cwd)
  --credential       Credential as KEY=VALUE, repeatable
  --mount            Bind mount as src:dst[:ro], repeatable
  --auth             Auth mode: auto (default), none, api, vertex, bedrock

Compose-specific flags:
  --allowed-domains  Domain groups for network filtering (repeatable)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			return runEnvCreate(gf, name, &cf, cmd)
		},
	}

	cmd.Flags().StringVarP(&cf.envType, "type", "t", "", "Environment type (local, container, compose, k8s-deploy, k8s-sandbox)")
	cmd.Flags().StringVar(&cf.image, "image", "", "Container image to use")
	cmd.Flags().StringSliceVar(&cf.ports, "port", nil, "Port mapping (host:container), repeatable")
	cmd.Flags().BoolVar(&cf.allPorts, "all-ports", false, "Expose all container ports")
	cmd.Flags().StringVar(&cf.storage, "storage", "", "Storage type: named-volume, host-path, empty-dir")
	cmd.Flags().StringVar(&cf.path, "path", "", "Project directory (compose: defaults to cwd)")
	cmd.Flags().StringSliceVar(&cf.credential, "credential", nil, "Credential as KEY=VALUE, repeatable")
	cmd.Flags().StringSliceVar(&cf.mount, "mount", nil, "Bind mount as src:dst[:ro], repeatable")
	cmd.Flags().StringVar(&cf.auth, "auth", "auto", "Auth mode: auto, none, api, vertex, bedrock")
	cmd.Flags().StringSliceVar(&cf.allowedDomains, "allowed-domains", nil, "Domain groups for network filtering (compose only), repeatable")
	cmd.Flags().BoolVar(&cf.gitignore, "gitignore", false, "Auto-add .cc-deck/ to .gitignore (compose only, deprecated)")

	return cmd
}

func runEnvCreate(gf *GlobalFlags, name string, cf *createFlags, cmd *cobra.Command) error {
	store := env.NewStateStore("")
	defs := env.NewDefinitionStore("")

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	// Try to find project-local definition.
	var projectRoot string
	var projDef *env.EnvironmentDefinition
	if root, findErr := project.FindProjectConfig(cwd); findErr == nil {
		projectRoot = root
		if def, loadErr := env.LoadProjectDefinition(root); loadErr == nil {
			projDef = def
		}
	} else if root, gitErr := project.FindGitRoot(cwd); gitErr == nil {
		// In a git repo but no .cc-deck/environment.yaml.
		projectRoot = root
	}

	// Resolve environment name (T016).
	if name == "" {
		if projDef != nil {
			name = projDef.Name
			fmt.Fprintf(os.Stderr, "Using environment %q from %s/.cc-deck/\n", name, projectRoot)
		} else if projectRoot != "" {
			// In a git repo with no definition: auto-scaffold (FR-025, T018).
			name = project.ProjectName(projectRoot)
			fmt.Fprintf(os.Stderr, "No .cc-deck/environment.yaml found. Scaffolding from CLI flags.\n")
		} else {
			return fmt.Errorf("no environment name specified and no .cc-deck/environment.yaml found in project hierarchy")
		}
	}

	if err := env.ValidateEnvName(name); err != nil {
		return err
	}

	// Resolve type: CLI flag > project definition > default (T017).
	typeChanged := cmd.Flags().Changed("type")
	envType := env.EnvironmentType(cf.envType)
	if !typeChanged && projDef != nil && projDef.Type != "" {
		envType = projDef.Type // Auto-detect from definition (FR-013).
	}
	if envType == "" {
		envType = env.EnvironmentTypeLocal
	}

	// Project-local vs global precedence check (FR-026, T020).
	if projDef != nil {
		if globalDef, globalErr := defs.FindByName(name); globalErr == nil {
			fmt.Fprintf(os.Stderr, "WARNING: Project-local definition shadows global definition %q (type: %s)\n",
				globalDef.Name, globalDef.Type)
		}
	}

	// Auto-scaffold definition if in git repo with no definition (FR-025, T018).
	if projDef == nil && projectRoot != "" {
		scaffoldDef := &env.EnvironmentDefinition{
			Name:           name,
			Type:           envType,
			Image:          cf.image,
			Auth:           cf.auth,
			AllowedDomains: cf.allowedDomains,
		}
		if err := env.SaveProjectDefinition(projectRoot, scaffoldDef); err != nil {
			return fmt.Errorf("scaffolding project definition: %w", err)
		}
		projDef = scaffoldDef
		fmt.Fprintf(os.Stderr, "Created .cc-deck/environment.yaml in %s\n", projectRoot)
	}

	// Apply project-local definition values, with CLI flags taking precedence (T017).
	if projDef != nil {
		if cf.image == "" && projDef.Image != "" {
			cf.image = projDef.Image
		}
		if !cmd.Flags().Changed("auth") && projDef.Auth != "" {
			cf.auth = projDef.Auth
		}
		if len(cf.allowedDomains) == 0 && len(projDef.AllowedDomains) > 0 {
			cf.allowedDomains = projDef.AllowedDomains
		}
		if len(cf.ports) == 0 && len(projDef.Ports) > 0 {
			cf.ports = projDef.Ports
		}
		if len(cf.mount) == 0 && len(projDef.Mounts) > 0 {
			cf.mount = projDef.Mounts
		}
		if len(cf.credential) == 0 && len(projDef.Credentials) > 0 {
			cf.credential = projDef.Credentials
		}
		if cf.path == "" && projDef.ProjectDir != "" {
			cf.path = projDef.ProjectDir
		}
	}

	e, err := env.NewEnvironment(envType, name, store, defs)
	if err != nil {
		return err
	}

	// Set container-specific options.
	if ce, ok := e.(*env.ContainerEnvironment); ok {
		ce.Auth = env.AuthMode(cf.auth)
		ce.Ports = cf.ports
		ce.AllPorts = cf.allPorts
		ce.Mounts = cf.mount

		if len(cf.credential) > 0 {
			ce.Credentials = make(map[string]string)
			for _, c := range cf.credential {
				parts := splitCredential(c)
				if parts == nil {
					return fmt.Errorf("invalid credential format %q, expected KEY=VALUE", c)
				}
				ce.Credentials[parts[0]] = parts[1]
			}
		}
	}

	// Set compose-specific options.
	if ce, ok := e.(*env.ComposeEnvironment); ok {
		ce.Auth = env.AuthMode(cf.auth)
		ce.Ports = cf.ports
		ce.AllPorts = cf.allPorts
		ce.Mounts = cf.mount
		ce.AllowedDomains = cf.allowedDomains
		ce.ProjectDir = cf.path
		ce.Gitignore = cf.gitignore

		if len(cf.credential) > 0 {
			ce.Credentials = make(map[string]string)
			for _, c := range cf.credential {
				parts := splitCredential(c)
				if parts == nil {
					return fmt.Errorf("invalid credential format %q, expected KEY=VALUE", c)
				}
				ce.Credentials[parts[0]] = parts[1]
			}
		}
	}

	// Resolve image: CLI flag > project definition > config default.
	image := cf.image
	if image == "" && envType == env.EnvironmentTypeContainer {
		if cfg, loadErr := config.Load(""); loadErr == nil && cfg.Defaults.Container.Image != "" {
			image = cfg.Defaults.Container.Image
		}
	}

	opts := env.CreateOpts{
		Image: image,
	}
	if cf.storage != "" {
		opts.Storage.Type = env.StorageType(cf.storage)
	} else if envType == env.EnvironmentTypeCompose {
		opts.Storage.Type = env.StorageTypeHostPath
	} else {
		if cfg, loadErr := config.Load(""); loadErr == nil && cfg.Defaults.Container.Storage != "" {
			opts.Storage.Type = env.StorageType(cfg.Defaults.Container.Storage)
		}
	}
	if cf.path != "" {
		opts.Storage.HostPath = cf.path
	}

	if err := e.Create(cmd_context(), opts); err != nil {
		return err
	}

	// Auto-register project in global registry (FR-007, T019).
	if projectRoot != "" {
		if regErr := store.RegisterProject(projectRoot); regErr != nil {
			fmt.Fprintf(os.Stderr, "WARNING: could not register project: %v\n", regErr)
		}

		// Ensure .cc-deck/.gitignore exists (FR-030).
		_ = env.EnsureCCDeckGitignore(projectRoot)

		// Store CLI overrides in status.yaml (FR-019, T019).
		overrides := collectOverrides(cmd, cf, projDef)
		if len(overrides) > 0 || projDef != nil {
			statusStore := env.NewProjectStatusStore(projectRoot)
			status, _ := statusStore.Load()
			status.State = env.EnvironmentStateStopped
			status.ContainerName = "cc-deck-" + name
			if status.CreatedAt.IsZero() {
				status.CreatedAt = time.Now()
			}
			if len(overrides) > 0 {
				status.Overrides = overrides
			}
			if saveErr := statusStore.Save(status); saveErr != nil {
				fmt.Fprintf(os.Stderr, "WARNING: could not save project status: %v\n", saveErr)
			}
		}
	}

	fmt.Fprintf(os.Stdout, "Environment %q created (type: %s)\n", name, envType)
	return nil
}

// collectOverrides returns CLI flag values that differ from the project definition.
func collectOverrides(cmd *cobra.Command, cf *createFlags, projDef *env.EnvironmentDefinition) map[string]string {
	if projDef == nil {
		return nil
	}
	overrides := make(map[string]string)
	if cmd.Flags().Changed("image") && cf.image != projDef.Image {
		overrides["image"] = cf.image
	}
	if cmd.Flags().Changed("auth") && cf.auth != projDef.Auth {
		overrides["auth"] = cf.auth
	}
	if cmd.Flags().Changed("type") && string(env.EnvironmentType(cf.envType)) != string(projDef.Type) {
		overrides["type"] = cf.envType
	}
	if len(overrides) == 0 {
		return nil
	}
	return overrides
}

// splitCredential splits a KEY=VALUE string. Returns nil if no '=' found.
func splitCredential(s string) []string {
	idx := -1
	for i, c := range s {
		if c == '=' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	return []string{s[:idx], s[idx+1:]}
}

// --- attach ---

func newEnvAttachCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "attach <name>",
		Short: "Attach to an environment",
		Long:  "Open an interactive session for the named environment.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvAttach(args[0])
		},
	}
}

func runEnvAttach(name string) error {
	store := env.NewStateStore("")
	defs := env.NewDefinitionStore("")

	e, err := resolveEnvironment(name, store, defs)
	if err != nil {
		return err
	}

	return e.Attach(cmd_context())
}

// --- delete ---

func newEnvDeleteCmd(_ *GlobalFlags) *cobra.Command {
	var force bool
	var keepVolumes bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an environment",
		Long: `Delete the named environment and remove it from the state store.
If the environment is running, use --force to stop and delete it.
For container environments, use --keep-volumes to preserve data volumes.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvDelete(args[0], force, keepVolumes)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force delete a running environment")
	cmd.Flags().BoolVar(&keepVolumes, "keep-volumes", false, "Keep data volumes when deleting container environments")

	return cmd
}

func runEnvDelete(name string, force bool, keepVolumes bool) error {
	store := env.NewStateStore("")
	defs := env.NewDefinitionStore("")

	e, err := resolveEnvironment(name, store, defs)
	if err != nil {
		// No state record found. Try to clean up orphaned podman resources
		// (container/volume) that match the naming convention. This handles
		// the case where state was lost (e.g., XDG path migration) but
		// podman resources still exist.
		cleaned := env.CleanupOrphanedContainer(cmd_context(), name, keepVolumes)
		if cleaned {
			// Also remove definition if it exists.
			_ = defs.Remove(name)
			fmt.Fprintf(os.Stdout, "Environment %q cleaned up (orphaned resources removed)\n", name)
			return nil
		}
		return err
	}

	if ce, ok := e.(*env.ContainerEnvironment); ok {
		ce.KeepVolumes = keepVolumes
	}

	if err := e.Delete(cmd_context(), force); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Environment %q deleted\n", name)
	return nil
}

// --- list ---

func newEnvListCmd(gf *GlobalFlags) *cobra.Command {
	var filterType string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List environments",
		Long:    "List all cc-deck environments with their current status.",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvList(gf, filterType)
		},
	}

	cmd.Flags().StringVarP(&filterType, "type", "t", "", "Filter by environment type")

	return cmd
}

func runEnvList(gf *GlobalFlags, filterType string) error {
	store := env.NewStateStore("")
	defs := env.NewDefinitionStore("")

	// Reconcile environments with actual state.
	_ = env.ReconcileLocalEnvs(store)
	_ = env.ReconcileContainerEnvs(store, defs)
	_ = env.ReconcileComposeEnvs(store)

	var filter *env.ListFilter
	if filterType != "" {
		t := env.EnvironmentType(filterType)
		filter = &env.ListFilter{Type: &t}
	}

	// List v1 records (local environments).
	records, err := store.List(filter)
	if err != nil {
		return err
	}

	// List v2 instances (container environments).
	instances, err := store.ListInstances()
	if err != nil {
		return err
	}

	// List definitions to show "not created" entries.
	allDefs, err := defs.List(filter)
	if err != nil {
		return err
	}

	// Build a set of instance names for dedup.
	instanceNames := make(map[string]bool)
	for _, inst := range instances {
		instanceNames[inst.Name] = true
	}

	switch gf.Output {
	case "json", "yaml":
		return writeEnvStructured(gf.Output, records, instances, allDefs, instanceNames, filterType)
	default:
		return writeEnvTable(records, instances, allDefs, instanceNames, filterType)
	}
}

// envListEntry is a unified representation for JSON/YAML output.
type envListEntry struct {
	Name         string `json:"name" yaml:"name"`
	Type         string `json:"type" yaml:"type"`
	State        string `json:"state" yaml:"state"`
	Storage      string `json:"storage,omitempty" yaml:"storage,omitempty"`
	Image        string `json:"image,omitempty" yaml:"image,omitempty"`
	LastAttached string `json:"last_attached,omitempty" yaml:"last_attached,omitempty"`
	Age          string `json:"age,omitempty" yaml:"age,omitempty"`
}

func writeEnvStructured(format string, records []*env.EnvironmentRecord, instances []*env.EnvironmentInstance, allDefs []*env.EnvironmentDefinition, instanceNames map[string]bool, filterType string) error {
	var entries []envListEntry

	for _, r := range records {
		entries = append(entries, envListEntry{
			Name:         r.Name,
			Type:         string(r.Type),
			State:        string(r.State),
			Storage:      storageDisplay(r),
			LastAttached: formatRelativeTime(r.LastAttached),
			Age:          formatDuration(time.Since(r.CreatedAt)),
		})
	}

	for _, inst := range instances {
		image := ""
		instType := string(inst.Type)
		if instType == "" {
			instType = "container"
		}
		storage := "named-volume"
		if inst.Container != nil {
			image = inst.Container.Image
		}
		if inst.Compose != nil {
			storage = "host-path"
		}
		entries = append(entries, envListEntry{
			Name:         inst.Name,
			Type:         instType,
			State:        string(inst.State),
			Storage:      storage,
			Image:        image,
			LastAttached: formatRelativeTime(inst.LastAttached),
			Age:          formatDuration(time.Since(inst.CreatedAt)),
		})
	}

	// Add definitions without instances as "not created".
	for _, def := range allDefs {
		if instanceNames[def.Name] {
			continue
		}
		// Skip if already in v1 records.
		found := false
		for _, r := range records {
			if r.Name == def.Name {
				found = true
				break
			}
		}
		if found {
			continue
		}
		if filterType != "" && string(def.Type) != filterType {
			continue
		}
		entries = append(entries, envListEntry{
			Name:    def.Name,
			Type:    string(def.Type),
			State:   "not created",
			Storage: "-",
		})
	}

	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	default:
		return yaml.NewEncoder(os.Stdout).Encode(entries)
	}
}

func writeEnvTable(records []*env.EnvironmentRecord, instances []*env.EnvironmentInstance, allDefs []*env.EnvironmentDefinition, instanceNames map[string]bool, filterType string) error {
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tSTATUS\tSTORAGE\tLAST ATTACHED\tAGE")

	hasEntries := false

	for _, r := range records {
		storage := storageDisplay(r)
		lastAttached := formatRelativeTime(r.LastAttached)
		age := formatDuration(time.Since(r.CreatedAt))

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Name,
			r.Type,
			r.State,
			storage,
			lastAttached,
			age,
		)
		hasEntries = true
	}

	for _, inst := range instances {
		instType := inst.Type
		if instType == "" {
			instType = env.EnvironmentTypeContainer
		}
		if filterType != "" && filterType != string(instType) {
			continue
		}
		storage := "named-volume"
		if inst.Compose != nil {
			storage = "host-path"
		}
		lastAttached := formatRelativeTime(inst.LastAttached)
		age := formatDuration(time.Since(inst.CreatedAt))

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			inst.Name,
			instType,
			inst.State,
			storage,
			lastAttached,
			age,
		)
		hasEntries = true
	}

	// Show definitions without instances as "not created".
	for _, d := range allDefs {
		if instanceNames[d.Name] {
			continue
		}
		// Also skip if it's a local type (those use v1 records).
		if d.Type == env.EnvironmentTypeLocal {
			continue
		}
		if filterType != "" && filterType != string(d.Type) {
			continue
		}
		storage := "-"
		if d.Storage != nil {
			storage = string(d.Storage.Type)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			d.Name,
			d.Type,
			"not created",
			storage,
			"never",
			"-",
		)
		hasEntries = true
	}

	if !hasEntries {
		_ = tw.Flush()
		fmt.Println("No environments found. Use 'cc-deck env create' to get started.")
		return nil
	}

	return tw.Flush()
}

func storageDisplay(r *env.EnvironmentRecord) string {
	if r.Type == env.EnvironmentTypeLocal {
		return "host"
	}
	if r.Storage != nil {
		return string(r.Storage.Type)
	}
	return "-"
}

func formatRelativeTime(t *time.Time) string {
	if t == nil {
		return "never"
	}
	return formatDuration(time.Since(*t)) + " ago"
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		days := int(d.Hours()) / 24
		return fmt.Sprintf("%dd", days)
	}
}

// --- status ---

func newEnvStatusCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status <name>",
		Short: "Show environment status",
		Long:  "Display detailed status information for the named environment.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvStatus(gf, args[0])
		},
	}
}

// envStatusOutput is used for JSON/YAML marshaling of status information.
type envStatusOutput struct {
	Name         string               `json:"name" yaml:"name"`
	Type         env.EnvironmentType  `json:"type" yaml:"type"`
	State        env.EnvironmentState `json:"state" yaml:"state"`
	Storage      string               `json:"storage" yaml:"storage"`
	Uptime       string               `json:"uptime" yaml:"uptime"`
	LastAttached string               `json:"last_attached" yaml:"last_attached"`
	Sessions     []env.SessionInfo    `json:"sessions,omitempty" yaml:"sessions,omitempty"`
	Image        string               `json:"image,omitempty" yaml:"image,omitempty"`
}

func runEnvStatus(gf *GlobalFlags, name string) error {
	store := env.NewStateStore("")
	defs := env.NewDefinitionStore("")

	e, err := resolveEnvironment(name, store, defs)
	if err != nil {
		return err
	}

	status, err := e.Status(cmd_context())
	if err != nil {
		return err
	}

	envType := e.Type()
	storage := "-"
	lastAttached := "never"
	image := ""

	if envType == env.EnvironmentTypeLocal {
		record, findErr := store.FindByName(name)
		if findErr == nil {
			storage = storageDisplay(record)
			lastAttached = formatRelativeTime(record.LastAttached)
		}
	} else if envType == env.EnvironmentTypeContainer || envType == env.EnvironmentTypeCompose {
		inst, findErr := store.FindInstanceByName(name)
		if findErr == nil {
			lastAttached = formatRelativeTime(inst.LastAttached)
			if inst.Container != nil {
				storage = "named-volume"
				image = inst.Container.Image
			}
			if inst.Compose != nil {
				storage = "host-path"
			}
		}
	}

	uptime := formatDuration(time.Since(status.Since))

	switch gf.Output {
	case "json":
		out := envStatusOutput{
			Name:         name,
			Type:         envType,
			State:        status.State,
			Storage:      storage,
			Uptime:       uptime,
			LastAttached: lastAttached,
			Sessions:     status.Sessions,
			Image:        image,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	case "yaml":
		out := envStatusOutput{
			Name:         name,
			Type:         envType,
			State:        status.State,
			Storage:      storage,
			Uptime:       uptime,
			LastAttached: lastAttached,
			Sessions:     status.Sessions,
			Image:        image,
		}
		return yaml.NewEncoder(os.Stdout).Encode(out)
	default:
		return writeEnvStatusText(name, envType, status, storage, uptime, lastAttached, image)
	}
}

func writeEnvStatusText(name string, envType env.EnvironmentType, status *env.EnvironmentStatus, storage, uptime, lastAttached, image string) error {
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintf(tw, "Environment:\t%s\n", name)
	fmt.Fprintf(tw, "Type:\t%s\n", envType)
	fmt.Fprintf(tw, "Status:\t%s\n", status.State)
	fmt.Fprintf(tw, "Storage:\t%s\n", storage)
	fmt.Fprintf(tw, "Uptime:\t%s\n", uptime)
	fmt.Fprintf(tw, "Attached:\t%s\n", lastAttached)
	if image != "" {
		fmt.Fprintf(tw, "Image:\t%s\n", image)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	if status.State == env.EnvironmentStateRunning && len(status.Sessions) > 0 {
		fmt.Println()
		fmt.Println("Agent Sessions:")
		stw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(stw, "  NAME\tSTATUS\tBRANCH\tLAST EVENT")
		for _, s := range status.Sessions {
			lastEvent := ""
			if !s.LastEvent.IsZero() {
				lastEvent = formatRelativeTime(&s.LastEvent) + " ago"
			}
			fmt.Fprintf(stw, "  %s\t%s\t%s\t%s\n",
				s.Name,
				s.Activity,
				s.Branch,
				lastEvent,
			)
		}
		if err := stw.Flush(); err != nil {
			return err
		}
	}

	return nil
}

// --- start ---

func newEnvStartCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "start <name>",
		Short: "Start a stopped environment",
		Long:  "Bring a stopped environment back to a running state.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvStart(args[0])
		},
	}
}

func runEnvStart(name string) error {
	store := env.NewStateStore("")
	defs := env.NewDefinitionStore("")

	e, err := resolveEnvironment(name, store, defs)
	if err != nil {
		return err
	}

	if err := e.Start(cmd_context()); err != nil {
		return err
	}

	// For local environments, update the v1 record state too.
	if e.Type() == env.EnvironmentTypeLocal {
		record, findErr := store.FindByName(name)
		if findErr == nil {
			record.State = env.EnvironmentStateRunning
			_ = store.Update(record)
		}
	}

	fmt.Fprintf(os.Stdout, "Environment %q started\n", name)
	return nil
}

// --- stop ---

func newEnvStopCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop a running environment",
		Long:  "Gracefully stop a running environment.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvStop(args[0])
		},
	}
}

func runEnvStop(name string) error {
	store := env.NewStateStore("")
	defs := env.NewDefinitionStore("")

	e, err := resolveEnvironment(name, store, defs)
	if err != nil {
		return err
	}

	if err := e.Stop(cmd_context()); err != nil {
		return err
	}

	// For local environments, update the v1 record state too.
	if e.Type() == env.EnvironmentTypeLocal {
		record, findErr := store.FindByName(name)
		if findErr == nil {
			record.State = env.EnvironmentStateStopped
			_ = store.Update(record)
		}
	}

	fmt.Fprintf(os.Stdout, "Environment %q stopped\n", name)
	return nil
}

// --- exec ---

func newEnvExecCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "exec <name> -- <cmd...>",
		Short: "Run a command inside an environment",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvExec(args[0], args[1:])
		},
	}
}

func runEnvExec(name string, cmdArgs []string) error {
	store := env.NewStateStore("")
	defs := env.NewDefinitionStore("")

	e, err := resolveEnvironment(name, store, defs)
	if err != nil {
		return err
	}

	if len(cmdArgs) == 0 {
		return fmt.Errorf("no command specified; use: cc-deck env exec %s -- <cmd>", name)
	}

	return e.Exec(cmd_context(), cmdArgs)
}

// --- push ---

func newEnvPushCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "push <name> <local-path> [remote-path]",
		Short: "Push local files into an environment",
		Args:  cobra.RangeArgs(1, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			localPath := ""
			remotePath := ""
			if len(args) > 1 {
				localPath = args[1]
			}
			if len(args) > 2 {
				remotePath = args[2]
			}
			return runEnvPush(args[0], localPath, remotePath)
		},
	}
}

func runEnvPush(name, localPath, remotePath string) error {
	store := env.NewStateStore("")
	defs := env.NewDefinitionStore("")

	e, err := resolveEnvironment(name, store, defs)
	if err != nil {
		return err
	}

	if err := e.Push(cmd_context(), env.SyncOpts{
		LocalPath:  localPath,
		RemotePath: remotePath,
	}); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Pushed to environment %q\n", name)
	return nil
}

// --- pull ---

func newEnvPullCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "pull <name> <remote-path> [local-path]",
		Short: "Pull files from an environment",
		Args:  cobra.RangeArgs(1, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			remotePath := ""
			localPath := ""
			if len(args) > 1 {
				remotePath = args[1]
			}
			if len(args) > 2 {
				localPath = args[2]
			}
			return runEnvPull(args[0], remotePath, localPath)
		},
	}
}

func runEnvPull(name, remotePath, localPath string) error {
	store := env.NewStateStore("")
	defs := env.NewDefinitionStore("")

	e, err := resolveEnvironment(name, store, defs)
	if err != nil {
		return err
	}

	if err := e.Pull(cmd_context(), env.SyncOpts{
		LocalPath:  localPath,
		RemotePath: remotePath,
	}); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Pulled from environment %q\n", name)
	return nil
}

// --- harvest ---

func newEnvHarvestCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "harvest <name>",
		Short: "Extract work products from an environment",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvHarvest(args[0])
		},
	}
}

func runEnvHarvest(name string) error {
	store := env.NewStateStore("")
	defs := env.NewDefinitionStore("")

	e, err := resolveEnvironment(name, store, defs)
	if err != nil {
		return err
	}

	return e.Harvest(cmd_context(), env.HarvestOpts{})
}

// --- logs ---

func newEnvLogsCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "logs <name>",
		Short: "View environment logs",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("logs: not yet implemented")
		},
	}
}

// resolveEnvironment finds an environment by name, checking both v1 records
// and v2 instances, and returns the appropriate Environment implementation.
func resolveEnvironment(name string, store *env.FileStateStore, defs *env.DefinitionStore) (env.Environment, error) {
	// Try v1 record first (local environments).
	if record, err := store.FindByName(name); err == nil {
		return env.NewEnvironment(record.Type, name, store, defs)
	}

	// Try v2 instance (container/compose environments).
	if inst, err := store.FindInstanceByName(name); err == nil {
		instType := env.EnvironmentTypeContainer
		if inst.Type != "" {
			instType = inst.Type
		} else if inst.Compose != nil {
			instType = env.EnvironmentTypeCompose
		}
		return env.NewEnvironment(instType, name, store, defs)
	}

	return nil, fmt.Errorf("environment %q not found", name)
}

// cmd_context returns a background context for CLI operations.
func cmd_context() context.Context {
	return context.Background()
}

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/env"
	"github.com/cc-deck/cc-deck/internal/project"
	sshPkg "github.com/cc-deck/cc-deck/internal/ssh"
)

// NewEnvCmd creates the env parent command with all subcommands.
func NewEnvCmd(gf *GlobalFlags) *cobra.Command {
	envCmd := &cobra.Command{
		Use:   "env",
		Short: "Manage environments",
		Long: `Environments are isolated workspaces where Claude Code sessions run.
Each environment has its own filesystem, tools, and configuration.

Use --type to select the runtime backend when creating an environment:

  local       Zellij session on the host machine (default)
  container   Single container managed by podman
  compose     Multi-container setup via podman-compose
  ssh         Remote machine over SSH
  k8s-deploy  Kubernetes deployment (planned)
  k8s-sandbox Ephemeral Kubernetes pod (planned)

Most commands accept an environment name, or auto-detect it from
a .cc-deck/environment.yaml file in the current project.`,
	}

	envCmd.AddGroup(
		&cobra.Group{ID: "lifecycle", Title: "Lifecycle:"},
		&cobra.Group{ID: "info", Title: "Info:"},
		&cobra.Group{ID: "data", Title: "Data Transfer:"},
		&cobra.Group{ID: "maintenance", Title: "Maintenance:"},
	)

	// Lifecycle: create → attach → start → stop → delete
	addToGroup(envCmd, "lifecycle",
		newEnvCreateCmd(gf),
		newEnvAttachCmd(gf),
		newEnvStartCmd(gf),
		newEnvStopCmd(gf),
		newEnvDeleteCmd(gf),
		newEnvRefreshCredsCmd(gf),
	)

	// Info
	addToGroup(envCmd, "info",
		newEnvListCmd(gf),
		newEnvStatusCmd(gf),
		newEnvLogsCmd(gf),
	)

	// Data transfer
	addToGroup(envCmd, "data",
		newEnvExecCmd(gf),
		newEnvPushCmd(gf),
		newEnvPullCmd(gf),
		newEnvHarvestCmd(gf),
	)

	// Maintenance
	addToGroup(envCmd, "maintenance",
		newEnvPruneCmd(),
	)

	return envCmd
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
	variant        string
	global         bool
	local          bool
	// SSH-specific flags
	host         string
	sshPort      int
	identityFile string
	jumpHost     string
	sshConfig    string
	workspace    string
}

func newEnvCreateCmd(gf *GlobalFlags) *cobra.Command {
	var cf createFlags

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new environment",
		Long: `Provision a new workspace for Claude Code sessions. Pick a --type to
control where the environment runs: locally in Zellij, inside a
container, or as a multi-container compose stack.

When run inside a git repository that contains .cc-deck/environment.yaml,
the name, type, and settings are loaded from that file automatically.
CLI flags override definition values. In a git repo without a definition,
one is scaffolded for you so your team can share it via version control.

Environment types (--type):
  local       Zellij session on the host machine (default)
  container   Single container managed by podman
  compose     Multi-container setup via podman-compose
  ssh         Remote machine over SSH
  k8s-deploy  Kubernetes deployment (planned)
  k8s-sandbox Ephemeral Kubernetes pod (planned)`,
		Example: `  # Create a local Zellij environment
  cc-deck env create my-project

  # Create a container environment with a custom image
  cc-deck env create my-project --type container --image quay.io/cc-deck/cc-deck-demo

  # Create a container with port forwarding and Vertex AI auth
  cc-deck env create api-dev --type container --port 8080:8080 --auth vertex

  # Create a compose environment with network filtering
  cc-deck env create my-app --type compose --allowed-domains python,github

  # Create an SSH environment on a remote machine
  cc-deck env create remote-dev --type ssh --host user@dev.example.com

  # Create from a project definition (auto-detected in cwd)
  cd ~/projects/my-app && cc-deck env create`,
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
	cmd.Flags().StringVar(&cf.variant, "variant", "", "Variant name for multiple instances from the same definition")

	// SSH-specific flags
	cmd.Flags().StringVar(&cf.host, "host", "", "SSH target host (user@host, SSH only)")
	cmd.Flags().IntVar(&cf.sshPort, "ssh-port", 0, "SSH port (SSH only)")
	cmd.Flags().StringVar(&cf.identityFile, "identity-file", "", "Path to SSH private key (SSH only)")
	cmd.Flags().StringVar(&cf.jumpHost, "jump-host", "", "SSH jump/bastion host (SSH only)")
	cmd.Flags().StringVar(&cf.sshConfig, "ssh-config", "", "Custom SSH config file path (SSH only)")
	cmd.Flags().StringVar(&cf.workspace, "workspace", "", "Remote workspace directory (SSH only, default: ~/workspace)")

	// Definition resolution flags
	cmd.Flags().BoolVar(&cf.global, "global", false, "Force resolution from global definition store")
	cmd.Flags().BoolVar(&cf.local, "local", false, "Force resolution from project-local definition")
	cmd.MarkFlagsMutuallyExclusive("global", "local")

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
	} else if name != "" {
		// Not in a git repo and no workspace config found, but explicit
		// name provided. Use cwd as workspace root for scaffolding.
		projectRoot = cwd
	}

	// Resolve environment name.
	if name == "" {
		if projDef != nil {
			name = projDef.Name
			fmt.Fprintf(os.Stderr, "Using environment %q from %s/.cc-deck/\n", name, projectRoot)
		} else if projectRoot != "" {
			// In a git repo with no definition: scaffold from CLI flags.
			name = project.ProjectName(projectRoot)
		} else {
			return fmt.Errorf("no environment name specified and no .cc-deck/environment.yaml found in project hierarchy")
		}
	}

	if err := env.ValidateEnvName(name); err != nil {
		return err
	}

	// Handle --global and --local flags (FR-012, FR-013, FR-014, FR-015).
	var usedGlobalDef bool
	var resolvedDef *env.EnvironmentDefinition

	if cf.global {
		globalDef, globalErr := defs.FindByName(name)
		if globalErr != nil {
			return fmt.Errorf("no global definition found for %q", name)
		}
		resolvedDef = globalDef
		usedGlobalDef = true
		projDef = nil
	} else if cf.local {
		if projDef == nil {
			return fmt.Errorf("no project-local definition found (no .cc-deck/environment.yaml)")
		}
		if name != projDef.Name {
			return fmt.Errorf("project-local definition is %q, not %q; use --global or omit --local", projDef.Name, name)
		}
		// Use project-local definition, ignore any global definitions.
	} else if projDef != nil && name != projDef.Name {
		// Explicit name differs from project-local definition: ignore project-local (FR-001).
		projDef = nil
		if globalDef, globalErr := defs.FindByName(name); globalErr == nil {
			// Use global definition's type and settings (FR-002).
			resolvedDef = globalDef
			usedGlobalDef = true
		}
		// If not found in global store, fall through to default type (FR-003).
	}

	// Resolve type: CLI flag > resolved definition > project definition > default (T017).
	typeChanged := cmd.Flags().Changed("type")
	envType := env.EnvironmentType(cf.envType)
	if !typeChanged {
		if resolvedDef != nil && resolvedDef.Type != "" {
			envType = resolvedDef.Type
		} else if projDef != nil && projDef.Type != "" {
			envType = projDef.Type
		}
	}
	if envType == "" {
		envType = env.EnvironmentTypeLocal
	}

	// Project-local vs global precedence check (FR-026, T020).
	if projDef != nil && !usedGlobalDef {
		if globalDef, globalErr := defs.FindByName(name); globalErr == nil {
			fmt.Fprintf(os.Stderr, "WARNING: Project-local definition shadows global definition %q (type: %s)\n",
				globalDef.Name, globalDef.Type)
		}
	}

	// Scaffold definition if no definition exists yet.
	// Skip scaffolding when using a global definition (FR-002a).
	if projDef == nil && projectRoot != "" && !usedGlobalDef {
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
		if _, gitErr := project.FindGitRoot(projectRoot); gitErr == nil {
			fmt.Fprintf(os.Stderr, "Commit .cc-deck/ to share the definition with your team.\n")
		}
	}

	// Apply definition values (from resolved global or project-local def), with CLI flags taking precedence.
	activeDef := projDef
	if resolvedDef != nil {
		activeDef = resolvedDef
	}
	if activeDef != nil {
		if cf.image == "" && activeDef.Image != "" {
			cf.image = activeDef.Image
		}
		if !cmd.Flags().Changed("auth") && activeDef.Auth != "" {
			cf.auth = activeDef.Auth
		}
		if len(cf.allowedDomains) == 0 && len(activeDef.AllowedDomains) > 0 {
			cf.allowedDomains = activeDef.AllowedDomains
		}
		if len(cf.ports) == 0 && len(activeDef.Ports) > 0 {
			cf.ports = activeDef.Ports
		}
		if len(cf.mount) == 0 && len(activeDef.Mounts) > 0 {
			cf.mount = activeDef.Mounts
		}
		if len(cf.credential) == 0 && len(activeDef.Credentials) > 0 {
			cf.credential = activeDef.Credentials
		}
		if cf.path == "" && activeDef.ProjectDir != "" {
			cf.path = activeDef.ProjectDir
		}
		// SSH-specific: inherit from definition.
		if cf.host == "" && activeDef.Host != "" {
			cf.host = activeDef.Host
		}
		if cf.sshPort == 0 && activeDef.Port != 0 {
			cf.sshPort = activeDef.Port
		}
		if cf.identityFile == "" && activeDef.IdentityFile != "" {
			cf.identityFile = activeDef.IdentityFile
		}
		if cf.jumpHost == "" && activeDef.JumpHost != "" {
			cf.jumpHost = activeDef.JumpHost
		}
		if cf.sshConfig == "" && activeDef.SSHConfig != "" {
			cf.sshConfig = activeDef.SSHConfig
		}
		if cf.workspace == "" && activeDef.Workspace != "" {
			cf.workspace = activeDef.Workspace
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

	// For SSH environments, ensure the definition has SSH fields populated
	// so SSHEnvironment.Create() can load them.
	if envType == env.EnvironmentTypeSSH {
		sshDef := &env.EnvironmentDefinition{
			Name:         name,
			Type:         env.EnvironmentTypeSSH,
			Auth:         cf.auth,
			Host:         cf.host,
			Port:         cf.sshPort,
			IdentityFile: cf.identityFile,
			JumpHost:     cf.jumpHost,
			SSHConfig:    cf.sshConfig,
			Workspace:    cf.workspace,
			Credentials:  cf.credential,
		}
		// Try update first (existing global definition), fall back to add.
		if err := defs.Update(sshDef); err != nil {
			if err := defs.Add(sshDef); err != nil {
				return fmt.Errorf("saving SSH definition: %w", err)
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

		// Store CLI overrides and variant in status.yaml (FR-019, FR-010).
		overrides := collectOverrides(cmd, cf, projDef)
		{
			statusStore := env.NewProjectStatusStore(projectRoot)
			status, _ := statusStore.Load()
			status.State = env.EnvironmentStateStopped
			containerName := "cc-deck-" + name
			if cf.variant != "" {
				containerName += "-" + cf.variant
				status.Variant = cf.variant
			}
			status.ContainerName = containerName
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

func newAttachCmdCore(_ *GlobalFlags) *cobra.Command {
	var branch string

	cmd := &cobra.Command{
		Use:   "attach [name]",
		Short: "Attach to an environment",
		Long: `Open an interactive session for the named environment.
When no name is provided, resolves from .cc-deck/environment.yaml in the project.
Use --branch to land in a specific worktree directory inside the container.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := env.NewStateStore("")
			name, _, err := resolveEnvironmentName(args, store)
			if err != nil {
				return err
			}
			if branch != "" {
				fmt.Fprintf(os.Stderr, "NOTE: --branch %q requested (worktree attach not yet wired to container exec)\n", branch)
			}
			return runEnvAttach(name)
		},
	}

	cmd.Flags().StringVar(&branch, "branch", "", "Attach and land in a specific worktree directory (FR-022)")

	return cmd
}

func newEnvAttachCmd(gf *GlobalFlags) *cobra.Command {
	return newAttachCmdCore(gf)
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
		Use:   "delete [name]",
		Short: "Delete an environment",
		Long: `Delete the named environment and remove it from the state store.
If the environment is running, use --force to stop and delete it.
For container environments, use --keep-volumes to preserve data volumes.
When no name is provided, resolves from .cc-deck/environment.yaml in the project.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := env.NewStateStore("")
			name, _, err := resolveEnvironmentName(args, store)
			if err != nil {
				return err
			}
			return runEnvDelete(name, force, keepVolumes)
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
			_ = defs.Remove(name)
			fmt.Fprintf(os.Stdout, "Environment %q cleaned up (orphaned resources removed)\n", name)
			return nil
		}

		// No container resources either. If a definition exists, remove it
		// (stale "not created" entry visible in env list).
		if defErr := defs.Remove(name); defErr == nil {
			fmt.Fprintf(os.Stdout, "Environment %q definition removed\n", name)
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

// projectListEntry represents a project-local environment in list output.
type projectListEntry struct {
	Name    string
	Type    env.EnvironmentType
	Status  string
	Path    string
	Missing bool
}

// worktreeListEntry represents a git worktree sub-entry in list output.
type worktreeListEntry struct {
	ProjectName string
	Path        string
	Branch      string
}

func newListCmdCore(gf *GlobalFlags) *cobra.Command {
	var filterType string
	var showWorktrees bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List environments",
		Long: `List all cc-deck environments with their current status.
Shows both global and project-local environments in a unified view.
Project paths and MISSING status are shown for registered projects.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvList(gf, filterType, showWorktrees)
		},
	}

	cmd.Flags().StringVarP(&filterType, "type", "t", "", "Filter by environment type")
	cmd.Flags().BoolVarP(&showWorktrees, "worktrees", "w", false, "Show git worktrees within each project")

	return cmd
}

func newEnvListCmd(gf *GlobalFlags) *cobra.Command {
	return newListCmdCore(gf)
}

func runEnvList(gf *GlobalFlags, filterType string, showWorktrees bool) error {
	store := env.NewStateStore("")
	defs := env.NewDefinitionStore("")

	// Reconcile environments with actual state.
	_ = env.ReconcileLocalEnvs(store)
	_ = env.ReconcileContainerEnvs(store, defs)
	_ = env.ReconcileComposeEnvs(store)
	_ = env.ReconcileSSHEnvs(store)

	var filter *env.ListFilter
	if filterType != "" {
		t := env.EnvironmentType(filterType)
		filter = &env.ListFilter{Type: &t}
	}

	// List all environment instances.
	instances, err := store.ListInstances(filter)
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

	// Collect project-local environments from the global registry (FR-006, FR-012).
	var projectEnvs []projectListEntry
	projects, _ := store.ListProjects()
	for _, p := range projects {
		if _, statErr := os.Stat(p.Path); statErr != nil {
			// Path no longer exists: MISSING (FR-008).
			projectEnvs = append(projectEnvs, projectListEntry{
				Name:    filepath.Base(p.Path),
				Path:    p.Path,
				Missing: true,
				Status:  "MISSING",
			})
			continue
		}
		def, loadErr := env.LoadProjectDefinition(p.Path)
		if loadErr != nil {
			continue
		}
		if filterType != "" && string(def.Type) != filterType {
			continue
		}
		// Skip if already shown via global state.
		if instanceNames[def.Name] {
			continue
		}
		statusStore := env.NewProjectStatusStore(p.Path)
		status, _ := statusStore.Load()
		state := "not created"
		if status.State != "" {
			state = string(status.State)
		}
		projectEnvs = append(projectEnvs, projectListEntry{
			Name:   def.Name,
			Type:   def.Type,
			Status: state,
			Path:   p.Path,
		})
	}

	// Collect worktree info if requested (FR-020).
	var worktrees []worktreeListEntry
	if showWorktrees {
		for _, p := range projects {
			if _, statErr := os.Stat(p.Path); statErr != nil {
				continue
			}
			wts, wtErr := project.ListWorktrees(p.Path)
			if wtErr != nil {
				continue
			}
			pName := filepath.Base(p.Path)
			for _, wt := range wts {
				worktrees = append(worktrees, worktreeListEntry{
					ProjectName: pName,
					Path:        wt.Path,
					Branch:      wt.Branch,
				})
			}
		}
	}

	switch gf.Output {
	case "json", "yaml":
		return writeEnvStructured(gf.Output, instances, allDefs, instanceNames, filterType, defs, projectEnvs)
	default:
		return writeEnvTableWithProjects(instances, allDefs, instanceNames, filterType, projectEnvs, worktrees, defs)
	}
}

// envListEntry is a unified representation for JSON/YAML output.
type envListEntry struct {
	Name         string `json:"name" yaml:"name"`
	Type         string `json:"type" yaml:"type"`
	State        string `json:"state" yaml:"state"`
	Source       string `json:"source" yaml:"source"`
	Storage      string `json:"storage,omitempty" yaml:"storage,omitempty"`
	Image        string `json:"image,omitempty" yaml:"image,omitempty"`
	LastAttached string `json:"last_attached,omitempty" yaml:"last_attached,omitempty"`
	Age          string `json:"age,omitempty" yaml:"age,omitempty"`
}

// buildSourceMap pre-computes the source origin for each environment name.
// Project-local entries take precedence over global definitions.
func buildSourceMap(defs *env.DefinitionStore, projectEnvs []projectListEntry) map[string]string {
	sourceMap := make(map[string]string)
	// Load global definitions once.
	if allGlobal, err := defs.List(nil); err == nil {
		for _, d := range allGlobal {
			sourceMap[d.Name] = "global"
		}
	}
	// Project-local entries override global.
	for _, pe := range projectEnvs {
		sourceMap[pe.Name] = "project"
	}
	return sourceMap
}

func writeEnvStructured(format string, instances []*env.EnvironmentInstance, allDefs []*env.EnvironmentDefinition, instanceNames map[string]bool, filterType string, defs *env.DefinitionStore, projectEnvs []projectListEntry) error {
	sourceMap := buildSourceMap(defs, projectEnvs)
	var entries []envListEntry

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
			Source:       sourceMap[inst.Name],
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
		if filterType != "" && string(def.Type) != filterType {
			continue
		}
		entries = append(entries, envListEntry{
			Name:    def.Name,
			Type:    string(def.Type),
			State:   "not created",
			Source:  "global",
			Storage: "-",
		})
	}

	// Add project-local entries that are not yet in entries.
	for _, pe := range projectEnvs {
		entries = append(entries, envListEntry{
			Name:   pe.Name,
			Type:   string(pe.Type),
			State:  pe.Status,
			Source: "project",
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

// writeEnvTableWithProjects writes the env list table with project-local entries.
func writeEnvTableWithProjects(instances []*env.EnvironmentInstance, allDefs []*env.EnvironmentDefinition, instanceNames map[string]bool, filterType string, projectEnvs []projectListEntry, worktrees []worktreeListEntry, defs *env.DefinitionStore) error {
	sourceMap := buildSourceMap(defs, projectEnvs)
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

	fmt.Fprintln(tw, "NAME\tTYPE\tSTATUS\tSOURCE\tSTORAGE\tLAST ATTACHED\tAGE")

	hasEntries := false
	printRow := func(name string, envType, state, source, storage, lastAttached, age string) {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", name, envType, state, source, storage, lastAttached, age)
		hasEntries = true
	}

	// Global environment instances.
	for _, inst := range instances {
		instType := inst.Type
		if instType == "" {
			instType = env.EnvironmentTypeContainer
		}
		if filterType != "" && filterType != string(instType) {
			continue
		}
		storage := "named-volume"
		if inst.Type == env.EnvironmentTypeLocal {
			storage = "host"
		} else if inst.Compose != nil {
			storage = "host-path"
		} else if inst.SSH != nil {
			storage = "-"
		}
		printRow(inst.Name, string(instType), string(inst.State), sourceMap[inst.Name], storage,
			formatRelativeTime(inst.LastAttached), formatDuration(time.Since(inst.CreatedAt)))
	}

	// Global definitions without instances as "not created".
	for _, d := range allDefs {
		if instanceNames[d.Name] {
			continue
		}
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
		printRow(d.Name, string(d.Type), "not created", "global", storage, "never", "-")
	}

	// Project-local environments (FR-012, FR-008).
	for _, pe := range projectEnvs {
		printRow(pe.Name, string(pe.Type), pe.Status, "project", "-", "-", "-")
	}

	// Worktree sub-entries (FR-020).
	for _, wt := range worktrees {
		printRow("  "+wt.Branch, "", "", "", "", "", "")
	}

	if !hasEntries {
		_ = tw.Flush()
		fmt.Println("No environments found. Use 'cc-deck env create' to get started.")
		return nil
	}

	return tw.Flush()
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

func newStatusCmdCore(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status [name]",
		Short: "Show environment status",
		Long: `Display detailed status information for the named environment.
When no name is provided, resolves from .cc-deck/environment.yaml in the project.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := env.NewStateStore("")
			name, _, err := resolveEnvironmentName(args, store)
			if err != nil {
				return err
			}
			return runEnvStatus(gf, name)
		},
	}
}

func newEnvStatusCmd(gf *GlobalFlags) *cobra.Command {
	return newStatusCmdCore(gf)
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
	ProjectPath  string               `json:"project_path,omitempty" yaml:"project_path,omitempty"`
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

	if inst, findErr := store.FindInstanceByName(name); findErr == nil {
		lastAttached = formatRelativeTime(inst.LastAttached)
		if inst.Type == env.EnvironmentTypeLocal {
			storage = "host"
		} else if inst.Container != nil {
			storage = "named-volume"
			image = inst.Container.Image
		}
		if inst.Compose != nil {
			storage = "host-path"
		}
		if inst.SSH != nil {
			storage = "remote"
		}
	}

	uptime := formatDuration(time.Since(status.Since))

	// Look up project path from project registry (FR-009).
	var projectPath string
	projects, _ := store.ListProjects()
	for _, p := range projects {
		def, loadErr := env.LoadProjectDefinition(p.Path)
		if loadErr != nil {
			continue
		}
		if def.Name == name {
			projectPath = p.Path
			break
		}
	}

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
			ProjectPath:  projectPath,
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
			ProjectPath:  projectPath,
		}
		return yaml.NewEncoder(os.Stdout).Encode(out)
	default:
		return writeEnvStatusText(name, envType, status, storage, uptime, lastAttached, image, projectPath)
	}
}

func writeEnvStatusText(name string, envType env.EnvironmentType, status *env.EnvironmentStatus, storage, uptime, lastAttached, image, projectPath string) error {
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
	if projectPath != "" {
		fmt.Fprintf(tw, "Project:\t%s\n", projectPath)
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
				lastEvent = formatRelativeTime(&s.LastEvent)
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

func newStartCmdCore(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "start [name]",
		Short: "Start a stopped environment",
		Long: `Bring a stopped environment back to a running state.
When no name is provided, resolves from .cc-deck/environment.yaml in the project.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := env.NewStateStore("")
			name, _, err := resolveEnvironmentName(args, store)
			if err != nil {
				return err
			}
			return runEnvStart(name)
		},
	}
}

func newEnvStartCmd(gf *GlobalFlags) *cobra.Command {
	return newStartCmdCore(gf)
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

	fmt.Fprintf(os.Stdout, "Environment %q started\n", name)
	return nil
}

// --- stop ---

func newStopCmdCore(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "stop [name]",
		Short: "Stop a running environment",
		Long: `Gracefully stop a running environment.
When no name is provided, resolves from .cc-deck/environment.yaml in the project.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := env.NewStateStore("")
			name, _, err := resolveEnvironmentName(args, store)
			if err != nil {
				return err
			}
			return runEnvStop(name)
		},
	}
}

func newEnvStopCmd(gf *GlobalFlags) *cobra.Command {
	return newStopCmdCore(gf)
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

// --- prune ---

func newEnvPruneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prune",
		Short: "Remove stale project registry entries",
		Long: `Remove entries from the global project registry whose directories
no longer exist. This cleans up entries for projects that have been
moved or deleted.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvPrune()
		},
	}
}

func runEnvPrune() error {
	store := env.NewStateStore("")
	count, err := store.PruneStaleProjects()
	if err != nil {
		return err
	}
	if count == 0 {
		fmt.Fprintln(os.Stdout, "No stale projects found.")
	} else {
		fmt.Fprintf(os.Stdout, "Removed %d stale project(s).\n", count)
	}
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

func newLogsCmdCore(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "logs <name>",
		Short: "View environment logs",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("logs: not yet implemented")
		},
	}
}

func newEnvLogsCmd(gf *GlobalFlags) *cobra.Command {
	return newLogsCmdCore(gf)
}

// --- refresh-creds ---

func newEnvRefreshCredsCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh-creds [name]",
		Short: "Push fresh credentials to a remote SSH environment",
		Long: `Refresh the credential file on a remote SSH environment without
attaching. This is useful for keeping long-running sessions alive
when local credentials rotate.

Only applicable to SSH environments. For auth=none, reports that
credential management is disabled.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := env.NewStateStore("")
			name, _, err := resolveEnvironmentName(args, store)
			if err != nil {
				return err
			}
			return runEnvRefreshCreds(name)
		},
	}
}

func runEnvRefreshCreds(name string) error {
	store := env.NewStateStore("")
	defs := env.NewDefinitionStore("")

	e, err := resolveEnvironment(name, store, defs)
	if err != nil {
		return err
	}

	if e.Type() != env.EnvironmentTypeSSH {
		return fmt.Errorf("refresh-creds is only supported for SSH environments (got: %s)", e.Type())
	}

	def, err := defs.FindByName(name)
	if err != nil {
		return err
	}

	if def.Auth == "none" {
		fmt.Fprintf(os.Stdout, "Credential management is disabled for environment %q (auth=none)\n", name)
		return nil
	}

	inst, err := store.FindInstanceByName(name)
	if err != nil {
		return err
	}

	if inst.SSH == nil {
		return fmt.Errorf("SSH fields missing for environment %q", name)
	}

	client := sshPkg.NewClient(inst.SSH.Host, inst.SSH.Port, inst.SSH.IdentityFile, inst.SSH.JumpHost, inst.SSH.SSHConfig)

	creds, err := sshPkg.BuildCredentialSet(def.Auth, def.Credentials, def.Env)
	if err != nil {
		return fmt.Errorf("building credentials: %w", err)
	}

	if len(creds) == 0 {
		fmt.Fprintf(os.Stdout, "No credentials found to refresh for environment %q\n", name)
		return nil
	}

	if err := sshPkg.WriteCredentialFile(cmd_context(), client, creds); err != nil {
		return fmt.Errorf("writing credentials: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Credentials refreshed for environment %q\n", name)
	return nil
}

// resolveEnvironmentName resolves an environment name from arguments or
// project-local config. When name is empty, walks to find .cc-deck/environment.yaml
// at the git root. Auto-registers discovered projects (FR-007). Displays
// resolution message (FR-018). Ensures .cc-deck/.gitignore (FR-030).
func resolveEnvironmentName(args []string, store *env.FileStateStore) (name string, projectRoot string, err error) {
	if len(args) > 0 && args[0] != "" {
		return args[0], "", nil
	}

	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return "", "", fmt.Errorf("getting current directory: %w", cwdErr)
	}

	root, findErr := project.FindProjectConfig(cwd)
	if findErr != nil {
		return "", "", fmt.Errorf("no environment name specified and no .cc-deck/environment.yaml found in project hierarchy")
	}

	def, loadErr := env.LoadProjectDefinition(root)
	if loadErr != nil {
		return "", "", fmt.Errorf("loading project definition from %s: %w", root, loadErr)
	}

	fmt.Fprintf(os.Stderr, "Using environment %q from %s/.cc-deck/\n", def.Name, root)

	// Auto-register project on walk-based discovery (FR-007).
	if regErr := store.RegisterProject(root); regErr != nil {
		fmt.Fprintf(os.Stderr, "WARNING: could not register project: %v\n", regErr)
	}

	// Self-heal .gitignore (FR-030).
	_ = env.EnsureCCDeckGitignore(root)

	return def.Name, root, nil
}

// resolveEnvironment finds an environment by name and returns the appropriate
// Environment implementation.
func resolveEnvironment(name string, store *env.FileStateStore, defs *env.DefinitionStore) (env.Environment, error) {
	if inst, err := store.FindInstanceByName(name); err == nil {
		instType := env.EnvironmentTypeContainer
		if inst.Type != "" {
			instType = inst.Type
		} else if inst.Compose != nil {
			instType = env.EnvironmentTypeCompose
		} else if inst.SSH != nil {
			instType = env.EnvironmentTypeSSH
		}
		return env.NewEnvironment(instType, name, store, defs)
	}

	return nil, fmt.Errorf("environment %q not found", name)
}

// addToGroup registers commands under a named group for help output.
func addToGroup(parent *cobra.Command, groupID string, cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		cmd.GroupID = groupID
		parent.AddCommand(cmd)
	}
}

// cmd_context returns a background context for CLI operations.
func cmd_context() context.Context {
	return context.Background()
}

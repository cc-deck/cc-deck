package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/ws"
	"github.com/cc-deck/cc-deck/internal/project"
	sshPkg "github.com/cc-deck/cc-deck/internal/ssh"
)

func NewWsCmd(gf *GlobalFlags) *cobra.Command {
	wsCmd := &cobra.Command{
		Use:     "ws",
		Aliases: []string{"workspace"},
		Short:   "Manage workspaces",
		Long: `Workspaces are isolated units where Claude Code sessions run.
Each workspace has its own filesystem, tools, and configuration.

Use --type to select the runtime backend when creating a workspace:

  local       Zellij session on the host machine (default)
  container   Single container managed by podman
  compose     Multi-container setup via podman-compose
  ssh         Remote machine over SSH
  k8s-deploy  Persistent Kubernetes workspace with StatefulSet
  k8s-sandbox Ephemeral Kubernetes pod (planned)

Most commands accept a workspace name, or auto-resolve by
matching the current directory against workspace definitions.`,
	}

	wsCmd.AddGroup(
		&cobra.Group{ID: "lifecycle", Title: "Lifecycle:"},
		&cobra.Group{ID: "info", Title: "Info:"},
		&cobra.Group{ID: "data", Title: "Data Transfer:"},
		&cobra.Group{ID: "maintenance", Title: "Maintenance:"},
	)

	// Lifecycle: create → attach → kill-session → start → stop → delete
	addToGroup(wsCmd, "lifecycle",
		newWsNewCmd(gf),
		newWsUpdateCmd(gf),
		newWsAttachCmd(gf),
		newWsKillSessionCmd(gf),
		newWsStartCmd(gf),
		newWsStopCmd(gf),
		newWsDeleteCmd(gf),
		newWsRefreshCredsCmd(gf),
	)

	// Info
	addToGroup(wsCmd, "info",
		newWsListCmd(gf),
		newWsStatusCmd(gf),
		newWsLogsCmd(gf),
	)

	// Data transfer
	addToGroup(wsCmd, "data",
		newWsExecCmd(gf),
		newWsPushCmd(gf),
		newWsPullCmd(gf),
		newWsHarvestCmd(gf),
	)

	// Maintenance
	addToGroup(wsCmd, "maintenance",
		newWsPruneCmd(),
	)

	return wsCmd
}

// --- create ---

type newFlags struct {
	wsType         string
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
	// SSH-specific flags
	host         string
	sshPort      int
	identityFile string
	jumpHost     string
	sshConfig    string
	workspace    string

	// k8s-deploy flags
	namespace      string
	kubeconfig     string
	k8sContext     string
	storageSize    string
	storageClass   string
	existingSecret string
	secretStore    string
	secretStoreRef string
	secretPath     string
	buildDir       string
	noNetworkPolicy bool
	allowDomain    []string
	allowGroup     []string
	keepVolumes    bool
	timeout        string

	// Repo cloning flags
	repos    []string
	branches []string

	// Idempotent update
	update bool
}

func newWsNewCmd(gf *GlobalFlags) *cobra.Command {
	var cf newFlags

	cmd := &cobra.Command{
		Use:   "new [name]",
		Short: "Create a new workspace",
		Long: `Provision a new workspace for Claude Code sessions. Pick a --type to
control where the workspace runs: locally in Zellij, inside a
container, or as a multi-container compose stack.

When a .cc-deck/workspace-template.yaml file exists in the project,
the template's name, type, and settings are used as defaults. CLI
flags override template values. Use --type to select which template
variant to use when multiple variants are defined.

All workspace definitions are stored centrally in the global
definition store (~/.config/cc-deck/workspaces.yaml).

Workspace types (--type):
  local       Zellij session on the host machine (default)
  container   Single container managed by podman
  compose     Multi-container setup via podman-compose
  ssh         Remote machine over SSH
  k8s-deploy  Persistent Kubernetes workspace with StatefulSet
  k8s-sandbox Ephemeral Kubernetes pod (planned)`,
		Example: `  # Create a local Zellij workspace
  cc-deck ws new my-project

  # Create a container workspace with a custom image
  cc-deck ws new my-project --type container --image quay.io/cc-deck/cc-deck-demo

  # Create from a project template (auto-detected in cwd)
  cd ~/projects/my-app && cc-deck ws new --type ssh

  # Create an SSH workspace on a remote machine
  cc-deck ws new remote-dev --type ssh --host user@dev.example.com`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			return runWsNew(gf, name, &cf, cmd)
		},
	}

	cmd.Flags().StringVarP(&cf.wsType, "type", "t", "", "Workspace type (local, container, compose, k8s-deploy, k8s-sandbox)")
	cmd.Flags().StringVar(&cf.auth, "auth", "auto", "Auth mode: auto, none, api, vertex, bedrock")
	cmd.Flags().StringSliceVar(&cf.credential, "credential", nil, "Credential as KEY=VALUE, repeatable")
	cmd.Flags().StringArrayVar(&cf.repos, "repo", nil, "Git repo URL to clone into workspace, repeatable")
	cmd.Flags().StringArrayVar(&cf.branches, "branch", nil, "Branch for corresponding --repo, repeatable")
	cmd.Flags().StringVar(&cf.variant, "variant", "", "Variant name for multiple instances from same definition")
	cmd.Flags().BoolVar(&cf.update, "update", false, "Update existing workspace (deprecated: use ws update)")
	_ = cmd.Flags().MarkDeprecated("update", "use 'cc-deck ws update' instead")

	// Container/compose flags
	cmd.Flags().StringVar(&cf.image, "image", "", "Container image (container, compose)")
	cmd.Flags().StringSliceVar(&cf.ports, "port", nil, "Port mapping host:container (container, compose)")
	cmd.Flags().BoolVar(&cf.allPorts, "all-ports", false, "Expose all container ports (container)")
	cmd.Flags().StringVar(&cf.storage, "storage", "", "Storage type: named-volume, host-path, empty-dir")
	cmd.Flags().StringSliceVar(&cf.mount, "mount", nil, "Bind mount src:dst[:ro] (container, compose)")
	cmd.Flags().StringVar(&cf.path, "path", "", "Project directory (compose)")
	cmd.Flags().StringSliceVar(&cf.allowedDomains, "allowed-domains", nil, "Domain groups for network filtering (compose)")

	// SSH flags
	cmd.Flags().StringVar(&cf.host, "host", "", "SSH target host (user@host)")
	cmd.Flags().IntVar(&cf.sshPort, "ssh-port", 0, "SSH port (default: 22)")
	cmd.Flags().StringVar(&cf.identityFile, "identity-file", "", "Path to SSH private key")
	cmd.Flags().StringVar(&cf.jumpHost, "jump-host", "", "SSH jump/bastion host")
	cmd.Flags().StringVar(&cf.sshConfig, "ssh-config", "", "Custom SSH config file path")
	cmd.Flags().StringVar(&cf.workspace, "workspace", "", "Remote workspace directory (default: ~/workspace)")

	// Kubernetes flags
	cmd.Flags().StringVarP(&cf.namespace, "namespace", "n", "", "Kubernetes namespace")
	cmd.Flags().StringVar(&cf.kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	cmd.Flags().StringVar(&cf.k8sContext, "context", "", "Kubeconfig context name")
	cmd.Flags().StringVar(&cf.storageSize, "storage-size", "", "PVC size, e.g. 10Gi (default: 10Gi)")
	cmd.Flags().StringVar(&cf.storageClass, "storage-class", "", "Kubernetes StorageClass name")
	cmd.Flags().StringVar(&cf.existingSecret, "existing-secret", "", "Pre-existing K8s Secret name")
	cmd.Flags().StringVar(&cf.secretStore, "secret-store", "", "ESO SecretStore type")
	cmd.Flags().StringVar(&cf.secretStoreRef, "secret-store-ref", "", "ESO SecretStore name")
	cmd.Flags().StringVar(&cf.secretPath, "secret-path", "", "ESO secret path")
	cmd.Flags().StringVar(&cf.buildDir, "build-dir", "", "Build directory with cc-deck-image.yaml")
	cmd.Flags().BoolVar(&cf.noNetworkPolicy, "no-network-policy", false, "Skip NetworkPolicy creation")
	cmd.Flags().StringSliceVar(&cf.allowDomain, "allow-domain", nil, "Allowed domain for NetworkPolicy")
	cmd.Flags().StringSliceVar(&cf.allowGroup, "allow-group", nil, "Domain group for NetworkPolicy")
	cmd.Flags().BoolVar(&cf.keepVolumes, "keep-volumes", false, "Keep PVCs when deleting")
	cmd.Flags().StringVar(&cf.timeout, "timeout", "", "Pod readiness timeout, e.g. 5m (default: 5m)")

	// Deprecated
	cmd.Flags().BoolVar(&cf.gitignore, "gitignore", false, "")
	_ = cmd.Flags().MarkHidden("gitignore")

	// Group flags by annotation for custom help display
	annotateFlags(cmd, "Container/Compose", "image", "port", "all-ports", "storage", "mount", "path", "allowed-domains")
	annotateFlags(cmd, "SSH", "host", "ssh-port", "identity-file", "jump-host", "ssh-config", "workspace")
	annotateFlags(cmd, "Kubernetes", "namespace", "kubeconfig", "context", "storage-size", "storage-class",
		"existing-secret", "secret-store", "secret-store-ref", "secret-path", "build-dir",
		"no-network-policy", "allow-domain", "allow-group", "keep-volumes", "timeout")

	cmd.SetHelpFunc(groupedHelp)

	return cmd
}

func runWsNew(gf *GlobalFlags, name string, cf *newFlags, cmd *cobra.Command) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	if strings.Contains(cwd, "/.cc-deck/") {
		return fmt.Errorf("refusing to create workspace inside a .cc-deck/ directory (%s)", cwd)
	}

	// Find project root for template loading and project-dir association.
	var projectRoot string
	if root, gitErr := project.FindGitRoot(cwd); gitErr == nil {
		projectRoot = root
	}

	// Try to load a workspace template.
	var tmpl *ws.WorkspaceTemplate
	if projectRoot != "" {
		if loaded, loadErr := ws.LoadTemplate(projectRoot); loadErr != nil {
			return fmt.Errorf("loading template: %w", loadErr)
		} else if loaded != nil {
			if validErr := ws.ValidateTemplate(loaded); validErr != nil {
				return validErr
			}
			tmpl = loaded
		}
	}

	// Resolve workspace name: explicit arg > template name > directory basename.
	if name == "" {
		if tmpl != nil {
			name = tmpl.Name
		} else if projectRoot != "" {
			name = project.ProjectName(projectRoot)
		} else {
			return fmt.Errorf("no workspace name specified; provide a name or run from a project directory")
		}
	}

	if err := ws.ValidateWsName(name); err != nil {
		return err
	}

	// Resolve type and build definition from template or flags.
	typeChanged := cmd.Flags().Changed("type")
	wsType := ws.WorkspaceType(cf.wsType)

	var activeDef *ws.WorkspaceDefinition

	if tmpl != nil {
		// Template-based creation.
		variantKey := cf.wsType
		if !typeChanged {
			if len(tmpl.Variants) == 1 {
				for k := range tmpl.Variants {
					variantKey = k
				}
			} else {
				keys := make([]string, 0, len(tmpl.Variants))
				for k := range tmpl.Variants {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				return fmt.Errorf("template defines multiple variants; use --type to select one: %s", strings.Join(keys, ", "))
			}
		}

		variant, ok := tmpl.Variants[variantKey]
		if !ok {
			keys := make([]string, 0, len(tmpl.Variants))
			for k := range tmpl.Variants {
				keys = append(keys, k)
			}
			return fmt.Errorf("template has no variant for type %q; available: %s", variantKey, strings.Join(keys, ", "))
		}

		wsType = ws.WorkspaceType(variantKey)

		// Extract and resolve placeholders.
		variantData, marshalErr := yaml.Marshal(variant)
		if marshalErr != nil {
			return fmt.Errorf("marshaling template variant: %w", marshalErr)
		}

		placeholders := ws.ExtractPlaceholders(variantData)
		if len(placeholders) > 0 {
			fmt.Fprintf(os.Stderr, "Template placeholders:\n")
			reader := bufio.NewReader(os.Stdin)
			answers, promptErr := ws.PromptForPlaceholders(placeholders, reader)
			if promptErr != nil {
				return promptErr
			}
			variantData = ws.ResolvePlaceholders(variantData, answers)

			// Re-parse variant with resolved values.
			if unmarshalErr := yaml.Unmarshal(variantData, &variant); unmarshalErr != nil {
				return fmt.Errorf("parsing resolved template: %w", unmarshalErr)
			}
		}

		activeDef = ws.VariantToDefinition(name, wsType, &variant)
	}

	if wsType == "" {
		wsType = ws.WorkspaceTypeLocal
	}

	// Apply CLI flag overrides over template values.
	if activeDef == nil {
		activeDef = &ws.WorkspaceDefinition{Name: name, Type: wsType}
	}

	if cmd.Flags().Changed("type") {
		activeDef.Type = wsType
	}
	applyFlagOverrides(cmd, cf, activeDef)

	// Set project-dir for all workspace types.
	activeDef.ProjectDir = project.CanonicalPath(cwd)

	// Store definition centrally with collision handling.
	finalName, addErr := defs.AddWithCollisionHandling(activeDef)
	if addErr != nil {
		return addErr
	}
	if finalName != name {
		fmt.Fprintf(os.Stderr, "Name collision: stored as %q\n", finalName)
		name = finalName
		activeDef.Name = finalName
	}

	e, err := ws.NewWorkspace(wsType, name, store, defs)
	if err != nil {
		return err
	}

	// Set type-specific options on the workspace object.
	setWorkspaceOptions(e, cf, cmd, activeDef)

	// Parse --repo/--branch flags into RepoEntries.
	if len(cf.branches) > len(cf.repos) {
		return fmt.Errorf("--branch requires a preceding --repo (got %d --branch but only %d --repo)", len(cf.branches), len(cf.repos))
	}
	var cliRepos []ws.RepoEntry
	for i, repoURL := range cf.repos {
		entry := ws.RepoEntry{URL: repoURL}
		if i < len(cf.branches) {
			entry.Branch = cf.branches[i]
		}
		cliRepos = append(cliRepos, entry)
	}

	// Auto-detect current git repo and collect extra remotes.
	var autoDetectedRepo *ws.RepoEntry
	var extraRemotes map[string]string
	if gitRoot, gitErr := project.FindGitRoot(cwd); gitErr == nil {
		remotes := parseGitRemotes(cwd)
		if originURL, ok := remotes["origin"]; ok {
			autoDetectedRepo = &ws.RepoEntry{URL: originURL}
			extraRemotes = make(map[string]string)
			for rName, url := range remotes {
				if rName != "origin" {
					extraRemotes[rName] = url
				}
			}
		}
		_ = gitRoot
	}

	// Merge repos: definition + CLI + auto-detected.
	var allRepos []ws.RepoEntry
	allRepos = append(allRepos, activeDef.Repos...)
	allRepos = append(allRepos, cliRepos...)
	if autoDetectedRepo != nil {
		allRepos = append(allRepos, *autoDetectedRepo)
	}
	allRepos = ws.DeduplicateRepos(allRepos)

	if len(allRepos) > 0 {
		var autoURL string
		if autoDetectedRepo != nil {
			autoURL = ws.NormalizeURL(autoDetectedRepo.URL)
		}
		switch te := e.(type) {
		case *ws.SSHWorkspace:
			te.Repos = allRepos
			te.ExtraRemotes = extraRemotes
			te.AutoDetectedURL = autoURL
		case *ws.ContainerWorkspace:
			te.Repos = allRepos
			te.ExtraRemotes = extraRemotes
			te.AutoDetectedURL = autoURL
		case *ws.ComposeWorkspace:
			te.Repos = allRepos
			te.ExtraRemotes = extraRemotes
			te.AutoDetectedURL = autoURL
		case *ws.K8sDeployWorkspace:
			te.Repos = allRepos
			te.ExtraRemotes = extraRemotes
			te.AutoDetectedURL = autoURL
		}
	}

	// Resolve image: CLI flag > definition > config default.
	image := cf.image
	if image == "" && activeDef.Image != "" {
		image = activeDef.Image
	}
	if image == "" && wsType == ws.WorkspaceTypeContainer {
		if cfg, loadErr := config.Load(""); loadErr == nil && cfg.Defaults.Container.Image != "" {
			image = cfg.Defaults.Container.Image
		}
	}

	opts := ws.CreateOpts{
		Image: image,
	}
	if cf.storage != "" {
		opts.Storage.Type = ws.StorageType(cf.storage)
	} else if wsType == ws.WorkspaceTypeCompose {
		opts.Storage.Type = ws.StorageTypeHostPath
	} else {
		if cfg, loadErr := config.Load(""); loadErr == nil && cfg.Defaults.Container.Storage != "" {
			opts.Storage.Type = ws.StorageType(cfg.Defaults.Container.Storage)
		}
	}
	if cf.path != "" {
		opts.Storage.HostPath = cf.path
	}

	// Warn and skip repos for local workspaces.
	if wsType == ws.WorkspaceTypeLocal && len(activeDef.Repos) > 0 {
		log.Printf("WARNING: repos are not supported for local workspaces; ignoring %d repo(s)", len(activeDef.Repos))
	}

	// Handle --update: if workspace already exists with the same type,
	// update its definition and instance in place instead of erroring.
	if cf.update {
		if existing, findErr := store.FindInstanceByName(name); findErr == nil {
			if existing.Type != wsType {
				return fmt.Errorf("workspace %q exists as type %s, cannot update to %s", name, existing.Type, wsType)
			}
			if existing.SSH != nil {
				if cf.host != "" {
					existing.SSH.Host = cf.host
				}
				if cf.sshPort != 0 {
					existing.SSH.Port = cf.sshPort
				}
				if cf.identityFile != "" {
					existing.SSH.IdentityFile = cf.identityFile
				}
				if cf.jumpHost != "" {
					existing.SSH.JumpHost = cf.jumpHost
				}
				if cf.sshConfig != "" {
					existing.SSH.SSHConfig = cf.sshConfig
				}
				if cf.workspace != "" {
					existing.SSH.Workspace = cf.workspace
				}
			}
			if err := store.UpdateInstance(existing); err != nil {
				return fmt.Errorf("updating workspace: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Workspace %q updated (type: %s)\n", name, wsType)
			return nil
		}
	}

	if err := e.Create(cmd_context(), opts); err != nil {
		_ = defs.Remove(activeDef.Name)
		return err
	}

	fmt.Fprintf(os.Stdout, "Workspace %q created (type: %s)\n", name, wsType)
	return nil
}

// applyFlagOverrides applies explicit CLI flag values over definition defaults.
func applyFlagOverrides(cmd *cobra.Command, cf *newFlags, def *ws.WorkspaceDefinition) {
	if cf.image != "" {
		def.Image = cf.image
	}
	if cmd.Flags().Changed("auth") {
		def.Auth = cf.auth
	}
	if len(cf.allowedDomains) > 0 {
		def.AllowedDomains = cf.allowedDomains
	}
	if len(cf.ports) > 0 {
		def.Ports = cf.ports
	}
	if len(cf.mount) > 0 {
		def.Mounts = cf.mount
	}
	if len(cf.credential) > 0 {
		def.Credentials = cf.credential
	}
	if cf.host != "" {
		def.Host = cf.host
	}
	if cf.sshPort != 0 {
		def.Port = cf.sshPort
	}
	if cf.identityFile != "" {
		def.IdentityFile = cf.identityFile
	}
	if cf.jumpHost != "" {
		def.JumpHost = cf.jumpHost
	}
	if cf.sshConfig != "" {
		def.SSHConfig = cf.sshConfig
	}
	if cf.workspace != "" {
		def.Workspace = cf.workspace
	}
	if cf.namespace != "" {
		def.Namespace = cf.namespace
	}
	if cf.kubeconfig != "" {
		def.Kubeconfig = cf.kubeconfig
	}
	if cf.k8sContext != "" {
		def.K8sContext = cf.k8sContext
	}
	if cf.storageSize != "" {
		def.StorageSize = cf.storageSize
	}
	if cf.storageClass != "" {
		def.StorageClass = cf.storageClass
	}
}

// setWorkspaceOptions configures type-specific fields on a workspace object.
func setWorkspaceOptions(e ws.Workspace, cf *newFlags, cmd *cobra.Command, def *ws.WorkspaceDefinition) {
	if ce, ok := e.(*ws.ContainerWorkspace); ok {
		ce.Auth = ws.AuthMode(def.Auth)
		ce.Ports = def.Ports
		ce.AllPorts = cf.allPorts
		ce.Mounts = def.Mounts
		if len(def.Credentials) > 0 {
			ce.Credentials = make(map[string]string)
			for _, c := range def.Credentials {
				parts := splitCredential(c)
				if parts != nil {
					ce.Credentials[parts[0]] = parts[1]
				}
			}
		}
	}

	if ke, ok := e.(*ws.K8sDeployWorkspace); ok {
		ke.Auth = ws.AuthMode(def.Auth)
		ke.Namespace = def.Namespace
		ke.Kubeconfig = def.Kubeconfig
		ke.Context = def.K8sContext
		ke.StorageSize = def.StorageSize
		ke.StorageClass = def.StorageClass
		ke.ExistingSecret = cf.existingSecret
		ke.SecretStore = cf.secretStore
		ke.SecretStoreRef = cf.secretStoreRef
		ke.SecretPath = cf.secretPath
		ke.BuildDir = cf.buildDir
		ke.NoNetworkPolicy = cf.noNetworkPolicy
		ke.AllowDomains = cf.allowDomain
		ke.AllowGroups = cf.allowGroup
		ke.KeepVolumes = cf.keepVolumes
		if cf.timeout != "" {
			d, parseErr := time.ParseDuration(cf.timeout)
			if parseErr == nil {
				ke.Timeout = d
			}
		}
		if len(def.Credentials) > 0 {
			ke.Credentials = make(map[string]string)
			for _, c := range def.Credentials {
				parts := splitCredential(c)
				if parts != nil {
					ke.Credentials[parts[0]] = parts[1]
				}
			}
		}
		if len(ke.AllowDomains) == 0 && len(def.AllowedDomains) > 0 {
			ke.AllowDomains = def.AllowedDomains
		}
	}

	if ce, ok := e.(*ws.ComposeWorkspace); ok {
		ce.Auth = ws.AuthMode(def.Auth)
		ce.Ports = def.Ports
		ce.AllPorts = cf.allPorts
		ce.Mounts = def.Mounts
		ce.AllowedDomains = def.AllowedDomains
		ce.ProjectDir = def.ProjectDir
		ce.Gitignore = cf.gitignore
		if len(def.Credentials) > 0 {
			ce.Credentials = make(map[string]string)
			for _, c := range def.Credentials {
				parts := splitCredential(c)
				if parts != nil {
					ce.Credentials[parts[0]] = parts[1]
				}
			}
		}
	}
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
	var reset bool

	cmd := &cobra.Command{
		Use:   "attach [name]",
		Short: "Attach to a workspace",
		Long: `Open an interactive session for the named workspace.
When no name is provided, auto-resolves from workspace definitions in the central store.
Use --reset to kill the existing Zellij session and start fresh.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := ws.NewStateStore("")
			name, _, err := resolveWorkspaceName(args, store)
			if err != nil {
				return err
			}
			if reset {
				return runWsAttachReset(name)
			}
			return runWsAttach(name)
		},
	}

	cmd.Flags().BoolVar(&reset, "reset", false, "Kill existing session and start fresh")

	return cmd
}

func newWsAttachCmd(gf *GlobalFlags) *cobra.Command {
	return newAttachCmdCore(gf)
}

func runWsAttachReset(name string) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	e, err := resolveWorkspace(name, store, defs)
	if err != nil {
		return err
	}

	_ = e.KillSession(cmd_context())

	fmt.Fprintf(os.Stderr, "Session reset. Attaching...\n")
	return e.Attach(cmd_context())
}

func runWsAttach(name string) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	e, err := resolveWorkspace(name, store, defs)
	if err != nil {
		return err
	}

	return e.Attach(cmd_context())
}

// --- update ---

func newWsUpdateCmd(_ *GlobalFlags) *cobra.Command {
	var syncRepos bool

	cmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Update workspace settings or sync repos",
		Long: `Update an existing workspace or sync repos from the workspace definition.

When --sync-repos is set, reads repos from the workspace definition in the central store
and clones any that don't exist on the remote. Already-cloned repos
are skipped (idempotent).`,
		Example: `  # Sync repos from workspace definition to the remote
  cc-deck ws update marovo --sync-repos

  # Auto-resolve workspace name from project config
  cc-deck ws update --sync-repos`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := ws.NewStateStore("")
			name, _, err := resolveWorkspaceName(args, store)
			if err != nil {
				return err
			}
			return runWsUpdate(name, syncRepos)
		},
	}

	cmd.Flags().BoolVar(&syncRepos, "sync-repos", false, "Clone missing repos from workspace definition")

	return cmd
}

func runWsUpdate(name string, syncRepos bool) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	inst, err := store.FindInstanceByName(name)
	if err != nil {
		return fmt.Errorf("workspace %q not found; create it first with ws new", name)
	}

	if syncRepos {
		// Load repos from central definition store.
		var repos []ws.RepoEntry
		if globalDef, globalErr := defs.FindByName(name); globalErr == nil {
			repos = globalDef.Repos
		}
		if len(repos) == 0 {
			return fmt.Errorf("no repos defined in workspace definition for %q", name)
		}

		e, resolveErr := resolveWorkspace(name, store, defs)
		if resolveErr != nil {
			return resolveErr
		}

		if sshEnv, ok := e.(*ws.SSHWorkspace); ok {
			return sshEnv.SyncRepos(cmd_context(), repos)
		}

		// For container/compose/k8s, repos are cloned via exec
		return fmt.Errorf("--sync-repos is currently supported for SSH workspaces only")
	}

	fmt.Fprintf(os.Stdout, "Workspace %q (type: %s, state: %s)\n", inst.Name, inst.Type, inst.State)
	fmt.Fprintln(os.Stdout, "Use --sync-repos to sync repositories from workspace definition.")
	return nil
}

// --- delete ---

func newWsDeleteCmd(_ *GlobalFlags) *cobra.Command {
	var force bool
	var keepVolumes bool

	cmd := &cobra.Command{
		Use:   "delete [name]",
		Aliases: []string{"rm"},
		Short: "Destroy a workspace",
		Long: `Destroy the named workspace and remove it from the state store.
If the workspace is running, use --force to stop and destroy it.
For container workspaces, use --keep-volumes to preserve data volumes.
When no name is provided, auto-resolves from workspace definitions in the central store.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := ws.NewStateStore("")
			name, _, err := resolveWorkspaceName(args, store)
			if err != nil {
				return err
			}
			return runWsDelete(name, force, keepVolumes)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force delete a running workspace")
	cmd.Flags().BoolVar(&keepVolumes, "keep-volumes", false, "Keep data volumes when deleting container workspaces")

	return cmd
}

func runWsDelete(name string, force bool, keepVolumes bool) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	e, err := resolveWorkspace(name, store, defs)
	if err != nil {
		// No state record found. Try to clean up orphaned podman resources.
		cleaned := ws.CleanupOrphanedContainer(cmd_context(), name, keepVolumes)
		if cleaned {
			_ = defs.Remove(name)
			fmt.Fprintf(os.Stdout, "Workspace %q cleaned up (orphaned resources removed)\n", name)
			return nil
		}

		// If a definition exists, remove it.
		if defErr := defs.Remove(name); defErr == nil {
			fmt.Fprintf(os.Stdout, "Workspace %q definition removed\n", name)
			return nil
		}

		return err
	}

	if ce, ok := e.(*ws.ContainerWorkspace); ok {
		ce.KeepVolumes = keepVolumes
	}
	if ke, ok := e.(*ws.K8sDeployWorkspace); ok {
		ke.KeepVolumes = keepVolumes
	}

	if err := e.Delete(cmd_context(), force); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Workspace %q deleted\n", name)
	return nil
}

// --- list ---


func newListCmdCore(gf *GlobalFlags) *cobra.Command {
	var filterType string
	var verbose bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List workspaces",
		Long: `List all cc-deck workspaces with their current status.
All workspace definitions are stored centrally. The PROJECT column
shows the project directory basename for workspaces associated with
a project, or "-" for standalone workspaces.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWsList(gf, filterType, false, verbose)
		},
	}

	cmd.Flags().StringVarP(&filterType, "type", "t", "", "Filter by workspace type")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show additional columns")

	return cmd
}

func newWsListCmd(gf *GlobalFlags) *cobra.Command {
	return newListCmdCore(gf)
}

func runWsList(gf *GlobalFlags, filterType string, showWorktrees bool, verbose bool) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	// Reconcile workspaces with actual state.
	_ = ws.ReconcileLocalWorkspaces(store)
	_ = ws.ReconcileContainerWorkspaces(store, defs)
	_ = ws.ReconcileComposeWorkspaces(store)
	_ = ws.ReconcileSSHWorkspaces(store)
	_ = ws.ReconcileK8sDeployWorkspaces(store)

	var filter *ws.ListFilter
	if filterType != "" {
		t := ws.WorkspaceType(filterType)
		filter = &ws.ListFilter{Type: &t}
	}

	// List all workspace instances.
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

	// Build project map: workspace name -> project basename from definition's ProjectDir.
	projectMap := buildProjectMap(allDefs)

	switch gf.Output {
	case "json", "yaml":
		return writeWsStructured(gf.Output, instances, allDefs, instanceNames, filterType, projectMap)
	default:
		return writeWsTableWithProjects(instances, allDefs, instanceNames, filterType, projectMap, verbose)
	}
}

// wsListEntry is a unified representation for JSON/YAML output.
type wsListEntry struct {
	Name         string `json:"name" yaml:"name"`
	Type         string `json:"type" yaml:"type"`
	State        string `json:"state" yaml:"state"`
	Project      string `json:"project" yaml:"project"`
	Storage      string `json:"storage,omitempty" yaml:"storage,omitempty"`
	Image        string `json:"image,omitempty" yaml:"image,omitempty"`
	LastAttached string `json:"last_attached,omitempty" yaml:"last_attached,omitempty"`
	Age          string `json:"age,omitempty" yaml:"age,omitempty"`
}

// buildProjectMap builds a map from workspace name to project basename
// derived from the definition's ProjectDir field.
func buildProjectMap(allDefs []*ws.WorkspaceDefinition) map[string]string {
	projectMap := make(map[string]string)
	for _, d := range allDefs {
		if d.ProjectDir != "" {
			projectMap[d.Name] = filepath.Base(d.ProjectDir)
		} else {
			projectMap[d.Name] = "-"
		}
	}
	return projectMap
}

func buildProjectPathMap(allDefs []*ws.WorkspaceDefinition) map[string]string {
	pathMap := make(map[string]string)
	for _, d := range allDefs {
		if d.ProjectDir != "" {
			pathMap[d.Name] = d.ProjectDir
		}
	}
	return pathMap
}

func writeWsStructured(format string, instances []*ws.WorkspaceInstance, allDefs []*ws.WorkspaceDefinition, instanceNames map[string]bool, filterType string, projectMap map[string]string) error {
	var entries []wsListEntry

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
		if inst.K8s != nil {
			storage = "pvc"
		}
		proj := projectMap[inst.Name]
		if proj == "" {
			proj = "-"
		}
		entries = append(entries, wsListEntry{
			Name:         inst.Name,
			Type:         instType,
			State:        formatWorkspaceState(inst),
			Project:      proj,
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
		proj := projectMap[def.Name]
		if proj == "" {
			proj = "-"
		}
		entries = append(entries, wsListEntry{
			Name:    def.Name,
			Type:    string(def.Type),
			State:   "not created",
			Project: proj,
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

// writeWsTableWithProjects writes the ws list table with PROJECT column.
func writeWsTableWithProjects(instances []*ws.WorkspaceInstance, allDefs []*ws.WorkspaceDefinition, instanceNames map[string]bool, filterType string, projectMap map[string]string, verbose bool) error {
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

	var pathMap map[string]string
	if verbose {
		pathMap = buildProjectPathMap(allDefs)
	}

	type row struct {
		name, wsType, state, proj, storage, lastAttached, age, path string
	}
	var rows []row

	for _, inst := range instances {
		instType := inst.Type
		if instType == "" {
			instType = ws.WorkspaceTypeContainer
		}
		if filterType != "" && filterType != string(instType) {
			continue
		}
		storage := "named-volume"
		if inst.Type == ws.WorkspaceTypeLocal {
			storage = "host"
		} else if inst.Compose != nil {
			storage = "host-path"
		} else if inst.SSH != nil {
			storage = "-"
		} else if inst.K8s != nil {
			storage = "pvc"
		}
		proj := projectMap[inst.Name]
		if proj == "" {
			proj = "-"
		}
		stateDisplay := formatWorkspaceState(inst)
		r := row{inst.Name, string(instType), stateDisplay, proj, storage,
			formatRelativeTime(inst.LastAttached), formatDuration(time.Since(inst.CreatedAt)), ""}
		if verbose && pathMap[inst.Name] != "" {
			r.path = pathMap[inst.Name]
		}
		rows = append(rows, r)
	}

	for _, d := range allDefs {
		if instanceNames[d.Name] {
			continue
		}
		if d.Type == ws.WorkspaceTypeLocal {
			continue
		}
		if filterType != "" && filterType != string(d.Type) {
			continue
		}
		storage := "-"
		if d.Storage != nil {
			storage = string(d.Storage.Type)
		}
		proj := projectMap[d.Name]
		if proj == "" {
			proj = "-"
		}
		r := row{d.Name, string(d.Type), "not created", proj, storage, "never", "-", ""}
		if verbose && pathMap[d.Name] != "" {
			r.path = pathMap[d.Name]
		}
		rows = append(rows, r)
	}

	if len(rows) == 0 {
		fmt.Println("No workspaces found. Use 'cc-deck ws new' to get started.")
		return nil
	}

	if verbose {
		fmt.Fprintln(tw, "NAME\tTYPE\tSTATUS\tPROJECT\tSTORAGE\tLAST ATTACHED\tAGE\tPROJECT PATH")
		for _, r := range rows {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				r.name, r.wsType, r.state, r.proj, r.storage, r.lastAttached, r.age, r.path)
		}
	} else {
		fmt.Fprintln(tw, "NAME\tTYPE\tSTATUS\tPROJECT\tSTORAGE\tLAST ATTACHED\tAGE")
		for _, r := range rows {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				r.name, r.wsType, r.state, r.proj, r.storage, r.lastAttached, r.age)
		}
	}

	return tw.Flush()
}

// formatWorkspaceState returns a display string for the two-dimensional state.
func formatWorkspaceState(inst *ws.WorkspaceInstance) string {
	if inst.InfraState != nil {
		infra := string(*inst.InfraState)
		if inst.SessionState == ws.SessionStateExists {
			return infra + ", session: exists"
		}
		return infra
	}
	if inst.SessionState == ws.SessionStateExists {
		return "session: exists"
	}
	return "no session"
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
		Short: "Show workspace status",
		Long: `Display detailed status information for the named workspace.
When no name is provided, auto-resolves from workspace definitions in the central store.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := ws.NewStateStore("")
			name, _, err := resolveWorkspaceName(args, store)
			if err != nil {
				return err
			}
			return runWsStatus(gf, name)
		},
	}
}

func newWsStatusCmd(gf *GlobalFlags) *cobra.Command {
	return newStatusCmdCore(gf)
}

// wsStatusOutput is used for JSON/YAML marshaling of status information.
type wsStatusOutput struct {
	Name         string            `json:"name" yaml:"name"`
	Type         ws.WorkspaceType  `json:"type" yaml:"type"`
	InfraState   *ws.InfraStateValue   `json:"infra_state,omitempty" yaml:"infra_state,omitempty"`
	SessionState ws.SessionStateValue  `json:"session_state" yaml:"session_state"`
	Storage      string            `json:"storage" yaml:"storage"`
	Uptime       string            `json:"uptime" yaml:"uptime"`
	LastAttached string            `json:"last_attached" yaml:"last_attached"`
	Sessions     []ws.SessionInfo  `json:"sessions,omitempty" yaml:"sessions,omitempty"`
	Image        string            `json:"image,omitempty" yaml:"image,omitempty"`
	ProjectPath  string            `json:"project_path,omitempty" yaml:"project_path,omitempty"`
}

func runWsStatus(gf *GlobalFlags, name string) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	e, err := resolveWorkspace(name, store, defs)
	if err != nil {
		return err
	}

	status, err := e.Status(cmd_context())
	if err != nil {
		return err
	}

	wsType := e.Type()
	storage := "-"
	lastAttached := "never"
	image := ""

	if inst, findErr := store.FindInstanceByName(name); findErr == nil {
		lastAttached = formatRelativeTime(inst.LastAttached)
		if inst.Type == ws.WorkspaceTypeLocal {
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
		if inst.K8s != nil {
			storage = "pvc"
		}
	}

	uptime := ""
	if status.Since != nil {
		uptime = formatDuration(time.Since(*status.Since))
	}

	// Look up project path from central definition store.
	var projectPath string
	if def, defErr := defs.FindByName(name); defErr == nil && def.ProjectDir != "" {
		projectPath = def.ProjectDir
	}

	switch gf.Output {
	case "json":
		out := wsStatusOutput{
			Name:         name,
			Type:         wsType,
			InfraState:   status.InfraState,
			SessionState: status.SessionState,
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
		out := wsStatusOutput{
			Name:         name,
			Type:         wsType,
			InfraState:   status.InfraState,
			SessionState: status.SessionState,
			Storage:      storage,
			Uptime:       uptime,
			LastAttached: lastAttached,
			Sessions:     status.Sessions,
			Image:        image,
			ProjectPath:  projectPath,
		}
		return yaml.NewEncoder(os.Stdout).Encode(out)
	default:
		return writeWsStatusText(name, wsType, status, storage, uptime, lastAttached, image, projectPath)
	}
}

func writeWsStatusText(name string, wsType ws.WorkspaceType, status *ws.WorkspaceStatus, storage, uptime, lastAttached, image, projectPath string) error {
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintf(tw, "Workspace:\t%s\n", name)
	fmt.Fprintf(tw, "Type:\t%s\n", wsType)
	if status.InfraState != nil {
		fmt.Fprintf(tw, "Infra:\t%s\n", *status.InfraState)
	}
	fmt.Fprintf(tw, "Session:\t%s\n", status.SessionState)
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

	if status.SessionState == ws.SessionStateExists && len(status.Sessions) > 0 {
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

// --- kill-session ---

func newWsKillSessionCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "kill-session [name]",
		Short: "Kill the Zellij session without affecting infrastructure",
		Long: `Kill the Zellij session for the named workspace. The underlying
infrastructure (container, pod) remains running. The next attach
will create a fresh session with the cc-deck layout.

When no name is provided, auto-resolves from workspace definitions.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := ws.NewStateStore("")
			name, _, err := resolveWorkspaceName(args, store)
			if err != nil {
				return err
			}
			return runWsKillSession(name)
		},
	}
}

func runWsKillSession(name string) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	e, err := resolveWorkspace(name, store, defs)
	if err != nil {
		return err
	}

	if err := e.KillSession(cmd_context()); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Session killed for workspace %q\n", name)
	return nil
}

// --- start ---

func newStartCmdCore(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "start [name]",
		Short: "Start a stopped workspace",
		Long: `Bring a stopped workspace back to a running state.
When no name is provided, auto-resolves from workspace definitions in the central store.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := ws.NewStateStore("")
			name, _, err := resolveWorkspaceName(args, store)
			if err != nil {
				return err
			}
			return runWsStart(name)
		},
	}
}

func newWsStartCmd(gf *GlobalFlags) *cobra.Command {
	return newStartCmdCore(gf)
}

func runWsStart(name string) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	e, err := resolveWorkspace(name, store, defs)
	if err != nil {
		return err
	}

	im, ok := e.(ws.InfraManager)
	if !ok {
		fmt.Fprintf(os.Stderr, "%s workspaces have no infrastructure to start. Use 'cc-deck ws attach %s' to connect.\n", e.Type(), name)
		return nil
	}

	if err := im.Start(cmd_context()); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Workspace %q started\n", name)
	return nil
}

// --- stop ---

func newStopCmdCore(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "stop [name]",
		Short: "Stop a running workspace",
		Long: `Gracefully stop a running workspace.
When no name is provided, auto-resolves from workspace definitions in the central store.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := ws.NewStateStore("")
			name, _, err := resolveWorkspaceName(args, store)
			if err != nil {
				return err
			}
			return runWsStop(name)
		},
	}
}

func newWsStopCmd(gf *GlobalFlags) *cobra.Command {
	return newStopCmdCore(gf)
}

func runWsStop(name string) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	e, err := resolveWorkspace(name, store, defs)
	if err != nil {
		return err
	}

	im, ok := e.(ws.InfraManager)
	if !ok {
		fmt.Fprintf(os.Stderr, "%s workspaces have no infrastructure to stop. Use 'cc-deck ws kill-session %s' to end the session.\n", e.Type(), name)
		return nil
	}

	if err := im.Stop(cmd_context()); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Workspace %q stopped\n", name)
	return nil
}

// --- exec ---

func newExecCmdCore(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "exec <name> -- <cmd...>",
		Short: "Run a command inside a workspace",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWsExec(args[0], args[1:])
		},
	}
}

func newWsExecCmd(gf *GlobalFlags) *cobra.Command {
	return newExecCmdCore(gf)
}

func runWsExec(name string, cmdArgs []string) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	e, err := resolveWorkspace(name, store, defs)
	if err != nil {
		return err
	}

	if len(cmdArgs) == 0 {
		return fmt.Errorf("no command specified; use: cc-deck ws exec %s -- <cmd>", name)
	}

	return e.Exec(cmd_context(), cmdArgs)
}

// --- push ---

func newWsPushCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "push <name> <local-path> [remote-path]",
		Short: "Push local files into a workspace",
		Long: `Copy local files into the named workspace. Works with all workspace
types including local workspaces (filesystem copy), containers
(podman cp), SSH (rsync), and Kubernetes (tar-over-exec).`,
		Args: cobra.RangeArgs(1, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			localPath := ""
			remotePath := ""
			if len(args) > 1 {
				localPath = args[1]
			}
			if len(args) > 2 {
				remotePath = args[2]
			}
			return runWsPush(args[0], localPath, remotePath)
		},
	}
}

func runWsPush(name, localPath, remotePath string) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	e, err := resolveWorkspace(name, store, defs)
	if err != nil {
		return err
	}

	if err := e.Push(cmd_context(), ws.SyncOpts{
		LocalPath:  localPath,
		RemotePath: remotePath,
	}); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Pushed to workspace %q\n", name)
	return nil
}

// --- pull ---

func newWsPullCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "pull <name> <remote-path> [local-path]",
		Short: "Pull files from a workspace",
		Long: `Copy files from the named workspace to local storage. Works with all
workspace types including local workspaces (filesystem copy),
containers (podman cp), SSH (rsync), and Kubernetes (tar-over-exec).`,
		Args: cobra.RangeArgs(1, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			remotePath := ""
			localPath := ""
			if len(args) > 1 {
				remotePath = args[1]
			}
			if len(args) > 2 {
				localPath = args[2]
			}
			return runWsPull(args[0], remotePath, localPath)
		},
	}
}

func runWsPull(name, remotePath, localPath string) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	e, err := resolveWorkspace(name, store, defs)
	if err != nil {
		return err
	}

	if err := e.Pull(cmd_context(), ws.SyncOpts{
		LocalPath:  localPath,
		RemotePath: remotePath,
	}); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Pulled from workspace %q\n", name)
	return nil
}

// --- prune ---

func newWsPruneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prune",
		Short: "Remove stale workspace definitions",
		Long: `Clean up workspace definitions whose instances no longer exist.
This is a no-op placeholder; workspace definitions are managed
centrally and do not accumulate stale entries.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stdout, "Nothing to prune. Workspace definitions are managed centrally.")
			return nil
		},
	}
}

// --- harvest ---

func newWsHarvestCmd(_ *GlobalFlags) *cobra.Command {
	var branch string
	var createPR bool
	var repoPath string

	cmd := &cobra.Command{
		Use:   "harvest <name>",
		Short: "Extract work products from a workspace",
		Long: `Fetch git commits from the remote workspace into the local repository.
Works with SSH, K8s, container, and compose workspace types.

The --branch flag creates a local branch from the fetched commits.
The --create-pr flag pushes the branch and creates a GitHub PR.
The --path flag specifies a subdirectory within the workspace (useful
when the workspace contains multiple repos).`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWsHarvest(args[0], branch, repoPath, createPR)
		},
	}

	cmd.Flags().StringVar(&branch, "branch", "", "Create a local branch with this name from fetched commits")
	cmd.Flags().BoolVar(&createPR, "create-pr", false, "Push branch and create a GitHub pull request")
	cmd.Flags().StringVar(&repoPath, "path", "", "Subdirectory within workspace (e.g., repo name)")

	return cmd
}

func runWsHarvest(name, branch, repoPath string, createPR bool) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	e, err := resolveWorkspace(name, store, defs)
	if err != nil {
		return err
	}

	return e.Harvest(cmd_context(), ws.HarvestOpts{
		Branch:   branch,
		CreatePR: createPR,
		Path:     repoPath,
	})
}

// --- logs ---

func newLogsCmdCore(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "logs <name>",
		Short: "View workspace logs",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("logs: not yet implemented")
		},
	}
}

func newWsLogsCmd(gf *GlobalFlags) *cobra.Command {
	return newLogsCmdCore(gf)
}

// --- refresh-creds ---

func newWsRefreshCredsCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh-creds [name]",
		Short: "Push fresh credentials to a remote SSH workspace",
		Long: `Refresh the credential file on a remote SSH workspace without
attaching. This is useful for keeping long-running sessions alive
when local credentials rotate.

Only applicable to SSH workspaces. For auth=none, reports that
credential management is disabled.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := ws.NewStateStore("")
			name, _, err := resolveWorkspaceName(args, store)
			if err != nil {
				return err
			}
			return runWsRefreshCreds(name)
		},
	}
}

func runWsRefreshCreds(name string) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	e, err := resolveWorkspace(name, store, defs)
	if err != nil {
		return err
	}

	if e.Type() != ws.WorkspaceTypeSSH {
		return fmt.Errorf("refresh-creds is only supported for SSH workspaces (got: %s)", e.Type())
	}

	def, err := defs.FindByName(name)
	if err != nil {
		return err
	}

	if def.Auth == "none" {
		fmt.Fprintf(os.Stdout, "Credential management is disabled for workspace %q (auth=none)\n", name)
		return nil
	}

	inst, err := store.FindInstanceByName(name)
	if err != nil {
		return err
	}

	if inst.SSH == nil {
		return fmt.Errorf("SSH fields missing for workspace %q", name)
	}

	client := sshPkg.NewClient(inst.SSH.Host, inst.SSH.Port, inst.SSH.IdentityFile, inst.SSH.JumpHost, inst.SSH.SSHConfig)

	creds, err := sshPkg.BuildCredentialSet(def.Auth, def.Credentials, def.Env)
	if err != nil {
		return fmt.Errorf("building credentials: %w", err)
	}

	if len(creds) == 0 {
		fmt.Fprintf(os.Stdout, "No credentials found to refresh for workspace %q\n", name)
		return nil
	}

	if err := sshPkg.WriteCredentialFile(cmd_context(), client, creds); err != nil {
		return fmt.Errorf("writing credentials: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Credentials refreshed for workspace %q\n", name)
	return nil
}

// resolveWorkspaceName resolves a workspace name from arguments or the
// central definition store. Uses a two-phase lookup: (1) filter by
// project-dir ancestor match against cwd, (2) fall back to global pool
// with recency-based selection.
func resolveWorkspaceName(args []string, store *ws.FileStateStore) (name string, projectRoot string, err error) {
	if len(args) > 0 && args[0] != "" {
		return args[0], "", nil
	}

	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return "", "", fmt.Errorf("getting current directory: %w", cwdErr)
	}

	defs := ws.NewDefinitionStore("")

	// Phase 1: filter by project-dir ancestor match.
	matches, findErr := defs.FindByProjectDir(cwd)
	if findErr == nil && len(matches) > 0 {
		selected := selectByRecency(matches, store)
		if selected != "" {
			fmt.Fprintf(os.Stderr, "Using workspace %q\n", selected)
			return selected, "", nil
		}
		if len(matches) > 1 {
			names := make([]string, len(matches))
			for i, m := range matches {
				names[i] = m.Name
			}
			return "", "", fmt.Errorf("multiple workspaces match this project (%s); specify a name", strings.Join(names, ", "))
		}
		fmt.Fprintf(os.Stderr, "Using workspace %q\n", matches[0].Name)
		return matches[0].Name, "", nil
	}

	// Phase 2: fall back to all definitions.
	allDefs, listErr := defs.List(nil)
	if listErr != nil {
		return "", "", fmt.Errorf("listing workspaces: %w", listErr)
	}
	if len(allDefs) == 0 {
		return "", "", fmt.Errorf("no workspaces found; create one with 'cc-deck ws new'")
	}
	selected := selectByRecency(allDefs, store)
	if selected != "" {
		fmt.Fprintf(os.Stderr, "Using workspace %q\n", selected)
		return selected, "", nil
	}

	return "", "", fmt.Errorf("no workspace specified; run 'cc-deck ws list' to see available workspaces")
}

// selectByRecency picks a workspace from a list of definitions. If there is
// exactly one, returns its name. If multiple, selects the most recently
// attached. Returns empty string if no selection can be made.
func selectByRecency(defs []*ws.WorkspaceDefinition, store *ws.FileStateStore) string {
	if len(defs) == 1 {
		return defs[0].Name
	}

	var bestName string
	var bestTime *time.Time
	for _, d := range defs {
		inst, err := store.FindInstanceByName(d.Name)
		if err != nil || inst.LastAttached == nil {
			continue
		}
		if bestTime == nil || inst.LastAttached.After(*bestTime) {
			bestName = d.Name
			bestTime = inst.LastAttached
		}
	}
	return bestName
}

// resolveWorkspace finds a workspace by name and returns the appropriate
// Workspace implementation.
func resolveWorkspace(name string, store *ws.FileStateStore, defs *ws.DefinitionStore) (ws.Workspace, error) {
	if inst, err := store.FindInstanceByName(name); err == nil {
		instType := ws.WorkspaceTypeContainer
		if inst.Type != "" {
			instType = inst.Type
		} else if inst.Compose != nil {
			instType = ws.WorkspaceTypeCompose
		} else if inst.SSH != nil {
			instType = ws.WorkspaceTypeSSH
		} else if inst.K8s != nil {
			instType = ws.WorkspaceTypeK8sDeploy
		}
		return ws.NewWorkspace(instType, name, store, defs)
	}

	// Check central definition store for "not created" workspaces.
	if def, defErr := defs.FindByName(name); defErr == nil {
		return ws.NewWorkspace(def.Type, name, store, defs)
	}

	return nil, fmt.Errorf("workspace %q not found", name)
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

// parseGitRemotes runs "git remote -v" in the given directory and returns
// a map of remote names to fetch URLs.
func parseGitRemotes(dir string) map[string]string {
	cmd := exec.CommandContext(context.Background(), "git", "-C", dir, "remote", "-v")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	remotes := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "origin\thttps://github.com/org/repo.git (fetch)"
		// Only use fetch entries.
		if !strings.HasSuffix(line, "(fetch)") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			remotes[parts[0]] = parts[1]
		}
	}
	return remotes
}

// --- Grouped flag help ---

const flagGroupAnnotation = "cc-deck-group"

func annotateFlags(cmd *cobra.Command, group string, names ...string) {
	for _, name := range names {
		f := cmd.Flags().Lookup(name)
		if f != nil {
			if f.Annotations == nil {
				f.Annotations = make(map[string][]string)
			}
			f.Annotations[flagGroupAnnotation] = []string{group}
		}
	}
}

func groupedHelp(cmd *cobra.Command, _ []string) {
	w := cmd.OutOrStdout()
	if cmd.Long != "" {
		fmt.Fprintln(w, cmd.Long)
	}

	fmt.Fprintf(w, "\nUsage:\n  %s\n", cmd.UseLine())

	if cmd.HasExample() {
		fmt.Fprintf(w, "\nExamples:\n%s\n", cmd.Example)
	}

	// General flags (no group annotation)
	fmt.Fprintf(w, "\nGeneral Flags:\n")
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		if _, ok := f.Annotations[flagGroupAnnotation]; ok {
			return
		}
		printFlag(w, f)
	})

	// Grouped flags
	for _, group := range []string{"Container/Compose", "SSH", "Kubernetes"} {
		fmt.Fprintf(w, "\n%s Flags:\n", group)
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			groups, ok := f.Annotations[flagGroupAnnotation]
			if !ok || len(groups) == 0 || groups[0] != group {
				return
			}
			printFlag(w, f)
		})
	}
}

func printFlag(w io.Writer, f *pflag.Flag) {
	name := ""
	if f.Shorthand != "" {
		name = fmt.Sprintf("  -%s, --%s", f.Shorthand, f.Name)
	} else {
		name = fmt.Sprintf("      --%s", f.Name)
	}
	typeName := f.Value.Type()
	if typeName == "string" {
		name += " string"
	} else if typeName == "int" {
		name += " int"
	} else if typeName == "stringSlice" || typeName == "stringArray" {
		name += " strings"
	}
	padding := 36 - len(name)
	if padding < 2 {
		padding = 2
	}
	fmt.Fprintf(w, "%s%s%s", name, strings.Repeat(" ", padding), f.Usage)
	if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" && f.DefValue != "[]" {
		fmt.Fprintf(w, " (default %q)", f.DefValue)
	}
	fmt.Fprintln(w)
}

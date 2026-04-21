package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
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

Most commands accept a workspace name, or auto-detect it from
a .cc-deck/workspace.yaml file in the current project.`,
	}

	wsCmd.AddGroup(
		&cobra.Group{ID: "lifecycle", Title: "Lifecycle:"},
		&cobra.Group{ID: "info", Title: "Info:"},
		&cobra.Group{ID: "data", Title: "Data Transfer:"},
		&cobra.Group{ID: "maintenance", Title: "Maintenance:"},
	)

	// Lifecycle: create → attach → start → stop → delete
	addToGroup(wsCmd, "lifecycle",
		newWsNewCmd(gf),
		newWsAttachCmd(gf),
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
	global         bool
	local          bool
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

When run inside a git repository that contains .cc-deck/workspace.yaml,
the name, type, and settings are loaded from that file automatically.
CLI flags override definition values. In a git repo without a definition,
one is scaffolded for you so your team can share it via version control.

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

  # Create a container with port forwarding and Vertex AI auth
  cc-deck ws new api-dev --type container --port 8080:8080 --auth vertex

  # Create a compose workspace with network filtering
  cc-deck ws new my-app --type compose --allowed-domains python,github

  # Create an SSH workspace on a remote machine
  cc-deck ws new remote-dev --type ssh --host user@dev.example.com

  # Create from a project definition (auto-detected in cwd)
  cd ~/projects/my-app && cc-deck ws new`,
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

	// k8s-deploy flags
	cmd.Flags().StringVarP(&cf.namespace, "namespace", "n", "", "Kubernetes namespace (k8s-deploy)")
	cmd.Flags().StringVar(&cf.kubeconfig, "kubeconfig", "", "Path to kubeconfig file (k8s-deploy)")
	cmd.Flags().StringVar(&cf.k8sContext, "context", "", "Kubeconfig context name (k8s-deploy)")
	cmd.Flags().StringVar(&cf.storageSize, "storage-size", "", "PVC storage size, e.g. 10Gi (k8s-deploy, default: 10Gi)")
	cmd.Flags().StringVar(&cf.storageClass, "storage-class", "", "Kubernetes StorageClass name (k8s-deploy)")
	cmd.Flags().StringVar(&cf.existingSecret, "existing-secret", "", "Reference to pre-existing K8s Secret (k8s-deploy)")
	cmd.Flags().StringVar(&cf.secretStore, "secret-store", "", "ESO SecretStore type (k8s-deploy)")
	cmd.Flags().StringVar(&cf.secretStoreRef, "secret-store-ref", "", "ESO SecretStore name (k8s-deploy)")
	cmd.Flags().StringVar(&cf.secretPath, "secret-path", "", "ESO secret path (k8s-deploy)")
	cmd.Flags().StringVar(&cf.buildDir, "build-dir", "", "Build directory containing cc-deck-image.yaml (k8s-deploy)")
	cmd.Flags().BoolVar(&cf.noNetworkPolicy, "no-network-policy", false, "Skip NetworkPolicy creation (k8s-deploy)")
	cmd.Flags().StringSliceVar(&cf.allowDomain, "allow-domain", nil, "Additional allowed domain for NetworkPolicy, repeatable (k8s-deploy)")
	cmd.Flags().StringSliceVar(&cf.allowGroup, "allow-group", nil, "Domain group for NetworkPolicy, repeatable (k8s-deploy)")
	cmd.Flags().BoolVar(&cf.keepVolumes, "keep-volumes", false, "Keep PVCs when deleting (k8s-deploy)")
	cmd.Flags().StringVar(&cf.timeout, "timeout", "", "Pod readiness timeout, e.g. 5m (k8s-deploy, default: 5m)")

	// Repo cloning flags
	cmd.Flags().StringArrayVar(&cf.repos, "repo", nil, "Git repo URL to clone into workspace, repeatable")
	cmd.Flags().StringArrayVar(&cf.branches, "branch", nil, "Branch for the most recent --repo, repeatable")

	// Idempotent update
	cmd.Flags().BoolVar(&cf.update, "update", false, "Update existing workspace instead of erroring on conflict")

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

	// Try to find project-local definition.
	var projectRoot string
	var projDef *ws.WorkspaceDefinition
	if root, findErr := project.FindProjectConfig(cwd); findErr == nil {
		projectRoot = root
		if def, loadErr := ws.LoadProjectDefinition(root); loadErr == nil {
			projDef = def
		}
	} else if root, gitErr := project.FindGitRoot(cwd); gitErr == nil {
		// In a git repo but no .cc-deck/workspace.yaml.
		projectRoot = root
	} else if name != "" {
		// Not in a git repo and no workspace config found, but explicit
		// name provided. Use cwd as workspace root for scaffolding.
		projectRoot = cwd
	}

	// Resolve workspace name.
	if name == "" {
		if projDef != nil {
			name = projDef.Name
			fmt.Fprintf(os.Stderr, "Using workspace %q from %s/.cc-deck/\n", name, projectRoot)
		} else if projectRoot != "" {
			// In a git repo with no definition: scaffold from CLI flags.
			name = project.ProjectName(projectRoot)
		} else {
			return fmt.Errorf("no workspace name specified and no .cc-deck/workspace.yaml found in project hierarchy")
		}
	}

	if err := ws.ValidateWsName(name); err != nil {
		return err
	}

	// Handle --global and --local flags (FR-012, FR-013, FR-014, FR-015).
	var usedGlobalDef bool
	var resolvedDef *ws.WorkspaceDefinition

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
			return fmt.Errorf("no project-local definition found (missing .cc-deck/workspace.yaml)")
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
	wsType := ws.WorkspaceType(cf.wsType)
	if !typeChanged {
		if resolvedDef != nil && resolvedDef.Type != "" {
			wsType = resolvedDef.Type
		} else if projDef != nil && projDef.Type != "" {
			wsType = projDef.Type
		}
	}
	if wsType == "" {
		wsType = ws.WorkspaceTypeLocal
	}

	// Project-local vs global precedence check (FR-026, T020).
	if projDef != nil && !usedGlobalDef {
		if globalDef, globalErr := defs.FindByName(name); globalErr == nil {
			fmt.Fprintf(os.Stderr, "WARNING: Project-local definition shadows global definition %q (type: %s)\n",
				globalDef.Name, globalDef.Type)
		}
	}

	// Check for cross-project name collisions before scaffolding.
	if projectRoot != "" {
		canonRoot := project.CanonicalPath(projectRoot)
		if others, lookupErr := store.AllProjectWorkspaceNames(canonRoot); lookupErr == nil {
			if otherPath, dup := others[name]; dup {
				return fmt.Errorf("workspace %q already defined in project %s", name, otherPath)
			}
		}
	}

	// Scaffold definition if no definition exists yet.
	// Skip scaffolding when using a global definition (FR-002a).
	if projDef == nil && projectRoot != "" && !usedGlobalDef {
		scaffoldDef := &ws.WorkspaceDefinition{
			Name:           name,
			Type:           wsType,
			Image:          cf.image,
			Auth:           cf.auth,
			AllowedDomains: cf.allowedDomains,
		}
		if err := ws.SaveProjectDefinition(projectRoot, scaffoldDef); err != nil {
			return fmt.Errorf("scaffolding project definition: %w", err)
		}
		projDef = scaffoldDef
		fmt.Fprintf(os.Stderr, "Created %s/.cc-deck/workspace.yaml\n", projectRoot)
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

	e, err := ws.NewWorkspace(wsType, name, store, defs)
	if err != nil {
		return err
	}

	// Set container-specific options.
	if ce, ok := e.(*ws.ContainerWorkspace); ok {
		ce.Auth = ws.AuthMode(cf.auth)
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

	// Set k8s-deploy-specific options.
	if ke, ok := e.(*ws.K8sDeployWorkspace); ok {
		ke.Auth = ws.AuthMode(cf.auth)
		ke.Namespace = cf.namespace
		ke.Kubeconfig = cf.kubeconfig
		ke.Context = cf.k8sContext
		ke.StorageSize = cf.storageSize
		ke.StorageClass = cf.storageClass
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
			if parseErr != nil {
				return fmt.Errorf("invalid timeout %q: %w", cf.timeout, parseErr)
			}
			ke.Timeout = d
		}

		if len(cf.credential) > 0 {
			ke.Credentials = make(map[string]string)
			for _, c := range cf.credential {
				parts := splitCredential(c)
				if parts == nil {
					return fmt.Errorf("invalid credential format %q, expected KEY=VALUE", c)
				}
				ke.Credentials[parts[0]] = parts[1]
			}
		}

		// Apply definition precedence for k8s-deploy fields.
		if projDef != nil {
			if !cmd.Flags().Changed("namespace") && projDef.Namespace != "" {
				ke.Namespace = projDef.Namespace
			}
			if !cmd.Flags().Changed("kubeconfig") && projDef.Kubeconfig != "" {
				ke.Kubeconfig = projDef.Kubeconfig
			}
			if !cmd.Flags().Changed("context") && projDef.K8sContext != "" {
				ke.Context = projDef.K8sContext
			}
			if !cmd.Flags().Changed("storage-size") && projDef.StorageSize != "" {
				ke.StorageSize = projDef.StorageSize
			}
			if !cmd.Flags().Changed("storage-class") && projDef.StorageClass != "" {
				ke.StorageClass = projDef.StorageClass
			}
			if len(ke.AllowDomains) == 0 && len(projDef.AllowedDomains) > 0 {
				ke.AllowDomains = projDef.AllowedDomains
			}
		}
	}

	// Set compose-specific options.
	if ce, ok := e.(*ws.ComposeWorkspace); ok {
		ce.Auth = ws.AuthMode(cf.auth)
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

	// For SSH workspaces, ensure the definition has SSH fields populated
	// so SSHWorkspace.Create() can load them.
	if wsType == ws.WorkspaceTypeSSH {
		sshDef := &ws.WorkspaceDefinition{
			Name:         name,
			Type:         ws.WorkspaceTypeSSH,
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
			// Collect non-origin remotes for post-clone configuration.
			extraRemotes = make(map[string]string)
			for name, url := range remotes {
				if name != "origin" {
					extraRemotes[name] = url
				}
			}
		}
		_ = gitRoot // Used only for detection.
	}

	// Merge repos: definition + CLI + auto-detected. Definition entries come
	// first so they win deduplication (BR-006: explicit branch/target take
	// precedence over auto-detected).
	var allRepos []ws.RepoEntry
	if activeDef != nil {
		allRepos = append(allRepos, activeDef.Repos...)
	}
	allRepos = append(allRepos, cliRepos...)
	if autoDetectedRepo != nil {
		allRepos = append(allRepos, *autoDetectedRepo)
	}
	allRepos = ws.DeduplicateRepos(allRepos)

	// Inject merged repos into the workspace struct so Create() can access them.
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

	// Resolve image: CLI flag > project definition > config default.
	image := cf.image
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

	// Warn and skip repos for local workspaces (FR-014).
	if wsType == ws.WorkspaceTypeLocal && activeDef != nil && len(activeDef.Repos) > 0 {
		log.Printf("WARNING: repos are not supported for local workspaces; ignoring %d repo(s)", len(activeDef.Repos))
		activeDef.Repos = nil
	}

	// Handle --update: if workspace already exists with the same type,
	// update its definition and instance in place instead of erroring.
	if cf.update {
		if existing, findErr := store.FindInstanceByName(name); findErr == nil {
			if existing.Type != wsType {
				return fmt.Errorf("workspace %q exists as type %s, cannot update to %s", name, existing.Type, wsType)
			}
			// Update SSH fields on the existing instance.
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
		// Not found: fall through to normal creation.
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
		_ = ws.EnsureCCDeckGitignore(projectRoot)

		// Store CLI overrides and variant in status.yaml (FR-019, FR-010).
		overrides := collectOverrides(cmd, cf, projDef)
		{
			statusStore := ws.NewProjectStatusStore(projectRoot)
			status, _ := statusStore.Load()
			status.State = ws.WorkspaceStateStopped
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

	fmt.Fprintf(os.Stdout, "Workspace %q created (type: %s)\n", name, wsType)
	return nil
}

// collectOverrides returns CLI flag values that differ from the project definition.
func collectOverrides(cmd *cobra.Command, cf *newFlags, projDef *ws.WorkspaceDefinition) map[string]string {
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
	if cmd.Flags().Changed("type") && string(ws.WorkspaceType(cf.wsType)) != string(projDef.Type) {
		overrides["type"] = cf.wsType
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
	var reset bool

	cmd := &cobra.Command{
		Use:   "attach [name]",
		Short: "Attach to a workspace",
		Long: `Open an interactive session for the named workspace.
When no name is provided, resolves from .cc-deck/workspace.yaml in the project.
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

	if sshWs, ok := e.(*ws.SSHWorkspace); ok {
		sshWs.KillRemoteSession(cmd_context())
	} else if containerWs, ok := e.(*ws.ContainerWorkspace); ok {
		_ = containerWs.Stop(cmd_context())
		_ = containerWs.Start(cmd_context())
	}

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
When no name is provided, resolves from .cc-deck/workspace.yaml in the project.`,
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
		// No state record found. Try to clean up orphaned podman resources
		// (container/volume) that match the naming convention. This handles
		// the case where state was lost (e.g., XDG path migration) but
		// podman resources still exist.
		cleaned := ws.CleanupOrphanedContainer(cmd_context(), name, keepVolumes)
		if cleaned {
			_ = defs.Remove(name)
			fmt.Fprintf(os.Stdout, "Workspace %q cleaned up (orphaned resources removed)\n", name)
			return nil
		}

		// No container resources either. If a definition exists, remove it
		// (stale "not created" entry visible in ws list).
		if defErr := defs.Remove(name); defErr == nil {
			fmt.Fprintf(os.Stdout, "Workspace %q definition removed\n", name)
			return nil
		}

		// Search project-local definitions from the project registry.
		if projectNames, lookupErr := store.AllProjectWorkspaceNames(""); lookupErr == nil {
			if projPath, found := projectNames[name]; found {
				defPath := filepath.Join(projPath, ".cc-deck", "workspace.yaml")
				if rmErr := os.Remove(defPath); rmErr != nil {
					return fmt.Errorf("removing project definition %s: %w", defPath, rmErr)
				}
				statusPath := filepath.Join(projPath, ".cc-deck", "status.yaml")
				_ = os.Remove(statusPath)
				_ = store.UnregisterProject(projPath)
				fmt.Fprintf(os.Stdout, "Workspace %q project definition removed (from %s)\n", name, projPath)
				return nil
			}
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

// projectListEntry represents a project-local workspace in list output.
type projectListEntry struct {
	Name    string
	Type    ws.WorkspaceType
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
	var verbose bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List workspaces",
		Long: `List all cc-deck workspaces with their current status.
Shows both global and project-local workspaces in a unified view.
Project paths and MISSING status are shown for registered projects.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWsList(gf, filterType, showWorktrees, verbose)
		},
	}

	cmd.Flags().StringVarP(&filterType, "type", "t", "", "Filter by workspace type")
	cmd.Flags().BoolVarP(&showWorktrees, "worktrees", "w", false, "Show git worktrees within each project")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show additional columns (PATH)")

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

	// Auto-prune stale project entries whose paths no longer exist.
	if pruned, _ := store.PruneStaleProjects(); pruned > 0 {
		log.Printf("Auto-pruned %d stale project(s) from registry.", pruned)
	}

	// Collect project-local workspaces from the global registry (FR-006, FR-012).
	var projectWs []projectListEntry
	seenProjectNames := make(map[string]bool)
	projects, _ := store.ListProjects()
	for _, p := range projects {
		def, loadErr := ws.LoadProjectDefinition(p.Path)
		if loadErr != nil {
			continue
		}
		if filterType != "" && string(def.Type) != filterType {
			continue
		}
		// Skip if already shown via global state or another project.
		if instanceNames[def.Name] || seenProjectNames[def.Name] {
			continue
		}
		seenProjectNames[def.Name] = true
		statusStore := ws.NewProjectStatusStore(p.Path)
		status, _ := statusStore.Load()
		state := "not created"
		if status.State != "" {
			state = string(status.State)
		}
		projectWs = append(projectWs, projectListEntry{
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
		return writeWsStructured(gf.Output, instances, allDefs, instanceNames, filterType, defs, projectWs)
	default:
		return writeWsTableWithProjects(instances, allDefs, instanceNames, filterType, projectWs, worktrees, defs, verbose)
	}
}

// wsListEntry is a unified representation for JSON/YAML output.
type wsListEntry struct {
	Name         string `json:"name" yaml:"name"`
	Type         string `json:"type" yaml:"type"`
	State        string `json:"state" yaml:"state"`
	Source       string `json:"source" yaml:"source"`
	Storage      string `json:"storage,omitempty" yaml:"storage,omitempty"`
	Image        string `json:"image,omitempty" yaml:"image,omitempty"`
	Path         string `json:"path,omitempty" yaml:"path,omitempty"`
	LastAttached string `json:"last_attached,omitempty" yaml:"last_attached,omitempty"`
	Age          string `json:"age,omitempty" yaml:"age,omitempty"`
}

// buildSourceMap pre-computes the source origin for each workspace name.
// Project-local entries take precedence over global definitions.
func buildSourceMap(defs *ws.DefinitionStore, projectWs []projectListEntry) map[string]string {
	sourceMap := make(map[string]string)
	// Load global definitions once.
	if allGlobal, err := defs.List(nil); err == nil {
		for _, d := range allGlobal {
			sourceMap[d.Name] = "global"
		}
	}
	// Project-local entries override global.
	for _, pe := range projectWs {
		sourceMap[pe.Name] = "project"
	}
	return sourceMap
}

func writeWsStructured(format string, instances []*ws.WorkspaceInstance, allDefs []*ws.WorkspaceDefinition, instanceNames map[string]bool, filterType string, defs *ws.DefinitionStore, projectWs []projectListEntry) error {
	sourceMap := buildSourceMap(defs, projectWs)
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
		entries = append(entries, wsListEntry{
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
		entries = append(entries, wsListEntry{
			Name:    def.Name,
			Type:    string(def.Type),
			State:   "not created",
			Source:  "global",
			Storage: "-",
		})
	}

	// Add project-local entries that are not yet in entries.
	for _, pe := range projectWs {
		entries = append(entries, wsListEntry{
			Name:   pe.Name,
			Type:   string(pe.Type),
			State:  pe.Status,
			Source: "project",
			Path:   pe.Path,
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

// writeWsTableWithProjects writes the ws list table with project-local entries.
func writeWsTableWithProjects(instances []*ws.WorkspaceInstance, allDefs []*ws.WorkspaceDefinition, instanceNames map[string]bool, filterType string, projectWs []projectListEntry, worktrees []worktreeListEntry, defs *ws.DefinitionStore, verbose bool) error {
	sourceMap := buildSourceMap(defs, projectWs)

	// Build path map from project-local entries for all workspaces.
	pathMap := make(map[string]string)
	for _, pe := range projectWs {
		pathMap[pe.Name] = pe.Path
	}
	store := ws.NewStateStore("")
	if projNames, err := store.AllProjectWorkspaceNames(""); err == nil {
		for name, path := range projNames {
			if _, exists := pathMap[name]; !exists {
				pathMap[name] = path
			}
		}
	}
	globalConfigDir := filepath.Dir(ws.DefaultDefinitionPath())

	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

	header := "NAME\tTYPE\tSTATUS\tSOURCE\tSTORAGE\tLAST ATTACHED\tAGE"
	if verbose {
		header += "\tPATH"
	}
	fmt.Fprintln(tw, header)

	homeDir, _ := os.UserHomeDir()
	shortenPath := func(p string) string {
		if homeDir != "" && strings.HasPrefix(p, homeDir) {
			return "~" + p[len(homeDir):]
		}
		return p
	}

	hasEntries := false
	printRow := func(name string, wsType, state, source, storage, lastAttached, age, path string) {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s", name, wsType, state, source, storage, lastAttached, age)
		if verbose {
			fmt.Fprintf(tw, "\t%s", shortenPath(path))
		}
		fmt.Fprint(tw, "\n")
		hasEntries = true
	}

	// Global workspace instances.
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
		path := globalConfigDir
		if p, ok := pathMap[inst.Name]; ok {
			path = p
		}
		printRow(inst.Name, string(instType), string(inst.State), sourceMap[inst.Name], storage,
			formatRelativeTime(inst.LastAttached), formatDuration(time.Since(inst.CreatedAt)), path)
	}

	// Global definitions without instances as "not created".
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
		printRow(d.Name, string(d.Type), "not created", "global", storage, "never", "-", globalConfigDir)
	}

	// Project-local workspaces (FR-012, FR-008).
	for _, pe := range projectWs {
		printRow(pe.Name, string(pe.Type), pe.Status, "project", "-", "-", "-", pe.Path)
	}

	// Worktree sub-entries (FR-020).
	for _, wt := range worktrees {
		printRow("  "+wt.Branch, "", "", "", "", "", "", "")
	}

	if !hasEntries {
		_ = tw.Flush()
		fmt.Println("No workspaces found. Use 'cc-deck ws new' to get started.")
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
		Short: "Show workspace status",
		Long: `Display detailed status information for the named workspace.
When no name is provided, resolves from .cc-deck/workspace.yaml in the project.`,
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
	Name         string               `json:"name" yaml:"name"`
	Type         ws.WorkspaceType  `json:"type" yaml:"type"`
	State        ws.WorkspaceState `json:"state" yaml:"state"`
	Storage      string               `json:"storage" yaml:"storage"`
	Uptime       string               `json:"uptime" yaml:"uptime"`
	LastAttached string               `json:"last_attached" yaml:"last_attached"`
	Sessions     []ws.SessionInfo    `json:"sessions,omitempty" yaml:"sessions,omitempty"`
	Image        string               `json:"image,omitempty" yaml:"image,omitempty"`
	ProjectPath  string               `json:"project_path,omitempty" yaml:"project_path,omitempty"`
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

	uptime := formatDuration(time.Since(status.Since))

	// Look up project path from project registry (FR-009).
	var projectPath string
	projects, _ := store.ListProjects()
	for _, p := range projects {
		def, loadErr := ws.LoadProjectDefinition(p.Path)
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
		out := wsStatusOutput{
			Name:         name,
			Type:         wsType,
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
		out := wsStatusOutput{
			Name:         name,
			Type:         wsType,
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
		return writeWsStatusText(name, wsType, status, storage, uptime, lastAttached, image, projectPath)
	}
}

func writeWsStatusText(name string, wsType ws.WorkspaceType, status *ws.WorkspaceStatus, storage, uptime, lastAttached, image, projectPath string) error {
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintf(tw, "Workspace:\t%s\n", name)
	fmt.Fprintf(tw, "Type:\t%s\n", wsType)
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

	if status.State == ws.WorkspaceStateRunning && len(status.Sessions) > 0 {
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
		Short: "Start a stopped workspace",
		Long: `Bring a stopped workspace back to a running state.
When no name is provided, resolves from .cc-deck/workspace.yaml in the project.`,
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

	if err := e.Start(cmd_context()); err != nil {
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
When no name is provided, resolves from .cc-deck/workspace.yaml in the project.`,
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

	if err := e.Stop(cmd_context()); err != nil {
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
		Short: "Remove stale project registry entries",
		Long: `Remove entries from the global project registry whose directories
no longer exist. This cleans up entries for projects that have been
moved or deleted.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWsPrune()
		},
	}
}

func runWsPrune() error {
	store := ws.NewStateStore("")
	paths, count, err := store.PruneStaleProjectsVerbose()
	if err != nil {
		return err
	}
	if count == 0 {
		fmt.Fprintln(os.Stdout, "No stale projects found.")
	} else {
		for _, p := range paths {
			fmt.Fprintf(os.Stdout, "Pruned: %s\n", p)
		}
		fmt.Fprintf(os.Stdout, "Removed %d stale project(s).\n", count)
	}
	return nil
}

// --- harvest ---

func newWsHarvestCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "harvest <name>",
		Short: "Extract work products from a workspace",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWsHarvest(args[0])
		},
	}
}

func runWsHarvest(name string) error {
	store := ws.NewStateStore("")
	defs := ws.NewDefinitionStore("")

	e, err := resolveWorkspace(name, store, defs)
	if err != nil {
		return err
	}

	return e.Harvest(cmd_context(), ws.HarvestOpts{})
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

// resolveWorkspaceName resolves a workspace name from arguments or
// project-local config. When name is empty, walks to find .cc-deck/workspace.yaml
// at the git root. Auto-registers discovered projects (FR-007). Displays
// resolution message (FR-018). Ensures .cc-deck/.gitignore (FR-030).
func resolveWorkspaceName(args []string, store *ws.FileStateStore) (name string, projectRoot string, err error) {
	if len(args) > 0 && args[0] != "" {
		return args[0], "", nil
	}

	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return "", "", fmt.Errorf("getting current directory: %w", cwdErr)
	}

	root, findErr := project.FindProjectConfig(cwd)
	if findErr != nil {
		return "", "", fmt.Errorf("no workspace name specified and no .cc-deck/workspace.yaml found in project hierarchy")
	}

	def, loadErr := ws.LoadProjectDefinition(root)
	if loadErr != nil {
		return "", "", fmt.Errorf("loading project definition from %s: %w", root, loadErr)
	}

	fmt.Fprintf(os.Stderr, "Using workspace %q from %s/.cc-deck/\n", def.Name, root)

	// Auto-register project on walk-based discovery (FR-007).
	if regErr := store.RegisterProject(root); regErr != nil {
		fmt.Fprintf(os.Stderr, "WARNING: could not register project: %v\n", regErr)
	}

	// Self-heal .gitignore (FR-030).
	_ = ws.EnsureCCDeckGitignore(root)

	return def.Name, root, nil
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

	// Search project definitions from the global registry so that
	// workspaces visible in "ws list" are also resolvable here.
	if projects, err := store.ListProjects(); err == nil {
		for _, p := range projects {
			def, loadErr := ws.LoadProjectDefinition(p.Path)
			if loadErr != nil || def.Name != name {
				continue
			}
			return ws.NewWorkspace(def.Type, name, store, defs)
		}
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

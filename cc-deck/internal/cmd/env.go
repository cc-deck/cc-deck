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
)

// NewEnvCmd creates the env parent command with all subcommands.
func NewEnvCmd(gf *GlobalFlags) *cobra.Command {
	envCmd := &cobra.Command{
		Use:   "env",
		Short: "Manage environments",
		Long:  "Create, manage, and interact with cc-deck environments (local, container, Kubernetes).",
	}

	envCmd.AddCommand(
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

// --- create ---

type createFlags struct {
	envType    string
	image      string
	ports      []string
	allPorts   bool
	storage    string
	path       string
	credential []string
	mount      []string
	auth       string
}

func newEnvCreateCmd(gf *GlobalFlags) *cobra.Command {
	var cf createFlags

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new environment",
		Long: `Create a new cc-deck environment. The name must contain only lowercase
letters, digits, and hyphens (max 40 characters).

Environment types:
  local       Zellij session on the host machine (default)
  container   Container environment managed by podman
  k8s-deploy  Kubernetes StatefulSet (not yet implemented)
  k8s-sandbox Ephemeral Kubernetes Pod (not yet implemented)

Container-specific flags:
  --image       Container image to use
  --port        Port mapping (host:container), repeatable
  --all-ports   Expose all container ports
  --storage     Storage type: named-volume (default), host-path, empty-dir
  --path        Host path for host-path storage
  --credential  Credential as KEY=VALUE, repeatable
  --mount       Bind mount as src:dst[:ro], repeatable
  --auth        Auth mode: auto (default), none, api, vertex, bedrock`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvCreate(gf, args[0], &cf)
		},
	}

	cmd.Flags().StringVarP(&cf.envType, "type", "t", "local", "Environment type (local, container, k8s-deploy, k8s-sandbox)")
	cmd.Flags().StringVar(&cf.image, "image", "", "Container image to use")
	cmd.Flags().StringSliceVar(&cf.ports, "port", nil, "Port mapping (host:container), repeatable")
	cmd.Flags().BoolVar(&cf.allPorts, "all-ports", false, "Expose all container ports")
	cmd.Flags().StringVar(&cf.storage, "storage", "", "Storage type: named-volume, host-path, empty-dir")
	cmd.Flags().StringVar(&cf.path, "path", "", "Host path for host-path storage")
	cmd.Flags().StringSliceVar(&cf.credential, "credential", nil, "Credential as KEY=VALUE, repeatable")
	cmd.Flags().StringSliceVar(&cf.mount, "mount", nil, "Bind mount as src:dst[:ro], repeatable")
	cmd.Flags().StringVar(&cf.auth, "auth", "auto", "Auth mode: auto, none, api, vertex, bedrock")

	return cmd
}

func runEnvCreate(_ *GlobalFlags, name string, cf *createFlags) error {
	if err := env.ValidateEnvName(name); err != nil {
		return err
	}

	envType := env.EnvironmentType(cf.envType)
	store := env.NewStateStore("")
	defs := env.NewDefinitionStore("")

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

	// Resolve image: CLI flag → config default (container.go handles definition → hardcoded fallback).
	image := cf.image
	if image == "" {
		if cfg, loadErr := config.Load(""); loadErr == nil && cfg.Defaults.Container.Image != "" {
			image = cfg.Defaults.Container.Image
		}
	}

	opts := env.CreateOpts{
		Image: image,
	}
	if cf.storage != "" {
		opts.Storage.Type = env.StorageType(cf.storage)
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

	fmt.Fprintf(os.Stdout, "Environment %q created (type: %s)\n", name, envType)
	return nil
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
		if inst.Container != nil {
			image = inst.Container.Image
		}
		entries = append(entries, envListEntry{
			Name:         inst.Name,
			Type:         "container",
			State:        string(inst.State),
			Storage:      "named-volume",
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
		if filterType != "" && filterType != string(env.EnvironmentTypeContainer) {
			continue
		}
		storage := "named-volume"
		lastAttached := formatRelativeTime(inst.LastAttached)
		age := formatDuration(time.Since(inst.CreatedAt))
		image := ""
		if inst.Container != nil {
			image = inst.Container.Image
			_ = image // used below in status text
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			inst.Name,
			env.EnvironmentTypeContainer,
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
	} else if envType == env.EnvironmentTypeContainer {
		storage = "named-volume"
		inst, findErr := store.FindInstanceByName(name)
		if findErr == nil {
			lastAttached = formatRelativeTime(inst.LastAttached)
			if inst.Container != nil {
				image = inst.Container.Image
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

	// Try v2 instance (container environments).
	if _, err := store.FindInstanceByName(name); err == nil {
		return env.NewEnvironment(env.EnvironmentTypeContainer, name, store, defs)
	}

	return nil, fmt.Errorf("environment %q not found", name)
}

// cmd_context returns a background context for CLI operations.
func cmd_context() context.Context {
	return context.Background()
}

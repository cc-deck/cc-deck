package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

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

func newEnvCreateCmd(gf *GlobalFlags) *cobra.Command {
	var envType string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new environment",
		Long: `Create a new cc-deck environment. The name must contain only lowercase
letters, digits, and hyphens (max 40 characters).

Environment types:
  local       Zellij session on the host machine (default)
  podman      Container via Podman (not yet implemented)
  k8s-deploy  Kubernetes StatefulSet (not yet implemented)
  k8s-sandbox Ephemeral Kubernetes Pod (not yet implemented)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvCreate(gf, args[0], envType)
		},
	}

	cmd.Flags().StringVarP(&envType, "type", "t", "local", "Environment type (local, podman, k8s-deploy, k8s-sandbox)")

	return cmd
}

func runEnvCreate(_ *GlobalFlags, name, envTypeStr string) error {
	if err := env.ValidateEnvName(name); err != nil {
		return err
	}

	envType := env.EnvironmentType(envTypeStr)
	store := env.NewStateStore("")

	e, err := env.NewEnvironment(envType, name, store)
	if err != nil {
		return err
	}

	if err := e.Create(cmd_context(), env.CreateOpts{}); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Environment %q created (type: %s)\n", name, envType)
	return nil
}

// --- attach ---

func newEnvAttachCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "attach <name>",
		Short: "Attach to an environment",
		Long:  "Open an interactive Zellij session for the named environment.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvAttach(args[0])
		},
	}
}

func runEnvAttach(name string) error {
	store := env.NewStateStore("")

	record, err := store.FindByName(name)
	if err != nil {
		return err
	}

	e, err := env.NewEnvironment(record.Type, name, store)
	if err != nil {
		return err
	}

	return e.Attach(cmd_context())
}

// --- delete ---

func newEnvDeleteCmd(_ *GlobalFlags) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an environment",
		Long: `Delete the named environment and remove it from the state store.
If the environment is running, use --force to stop and delete it.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvDelete(args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force delete a running environment")

	return cmd
}

func runEnvDelete(name string, force bool) error {
	store := env.NewStateStore("")

	record, err := store.FindByName(name)
	if err != nil {
		return err
	}

	e, err := env.NewEnvironment(record.Type, name, store)
	if err != nil {
		return err
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

	// Reconcile local environments with actual Zellij sessions.
	// Errors are non-fatal (Zellij may not be installed).
	_ = env.ReconcileLocalEnvs(store)

	var filter *env.ListFilter
	if filterType != "" {
		t := env.EnvironmentType(filterType)
		filter = &env.ListFilter{Type: &t}
	}

	records, err := store.List(filter)
	if err != nil {
		return err
	}

	switch gf.Output {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(records)
	case "yaml":
		return yaml.NewEncoder(os.Stdout).Encode(records)
	default:
		return writeEnvTable(records)
	}
}

func writeEnvTable(records []*env.EnvironmentRecord) error {
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tSTATUS\tSTORAGE\tLAST ATTACHED\tAGE")

	if len(records) == 0 {
		_ = tw.Flush()
		fmt.Println("No environments found. Use 'cc-deck env create' to get started.")
		return nil
	}

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
}

func runEnvStatus(gf *GlobalFlags, name string) error {
	store := env.NewStateStore("")

	record, err := store.FindByName(name)
	if err != nil {
		return err
	}

	e, err := env.NewEnvironment(record.Type, name, store)
	if err != nil {
		return err
	}

	status, err := e.Status(cmd_context())
	if err != nil {
		return err
	}

	storage := storageDisplay(record)
	uptime := formatDuration(time.Since(status.Since))
	lastAttached := formatRelativeTime(record.LastAttached)

	switch gf.Output {
	case "json":
		out := envStatusOutput{
			Name:         name,
			Type:         record.Type,
			State:        status.State,
			Storage:      storage,
			Uptime:       uptime,
			LastAttached: lastAttached,
			Sessions:     status.Sessions,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	case "yaml":
		out := envStatusOutput{
			Name:         name,
			Type:         record.Type,
			State:        status.State,
			Storage:      storage,
			Uptime:       uptime,
			LastAttached: lastAttached,
			Sessions:     status.Sessions,
		}
		return yaml.NewEncoder(os.Stdout).Encode(out)
	default:
		return writeEnvStatusText(name, record, status, storage, uptime, lastAttached)
	}
}

func writeEnvStatusText(name string, record *env.EnvironmentRecord, status *env.EnvironmentStatus, storage, uptime, lastAttached string) error {
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintf(tw, "Environment:\t%s\n", name)
	fmt.Fprintf(tw, "Type:\t%s\n", record.Type)
	fmt.Fprintf(tw, "Status:\t%s\n", status.State)
	fmt.Fprintf(tw, "Storage:\t%s\n", storage)
	fmt.Fprintf(tw, "Uptime:\t%s\n", uptime)
	fmt.Fprintf(tw, "Attached:\t%s\n", lastAttached)
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

	record, err := store.FindByName(name)
	if err != nil {
		return err
	}

	if record.State != env.EnvironmentStateStopped {
		return fmt.Errorf("environment %q is not stopped (current state: %s)", name, record.State)
	}

	e, err := env.NewEnvironment(record.Type, name, store)
	if err != nil {
		return err
	}

	if err := e.Start(cmd_context()); err != nil {
		if errors.Is(err, env.ErrNotSupported) {
			fmt.Fprintf(os.Stderr, "Note: %v\n", err)
			return nil
		}
		return err
	}

	// Update state in store.
	record.State = env.EnvironmentStateRunning
	if updateErr := store.Update(record); updateErr != nil {
		return updateErr
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

	record, err := store.FindByName(name)
	if err != nil {
		return err
	}

	if record.State != env.EnvironmentStateRunning {
		return fmt.Errorf("environment %q is not running (current state: %s)", name, record.State)
	}

	e, err := env.NewEnvironment(record.Type, name, store)
	if err != nil {
		return err
	}

	if err := e.Stop(cmd_context()); err != nil {
		if errors.Is(err, env.ErrNotSupported) {
			fmt.Fprintf(os.Stderr, "Note: %v\n", err)
			return nil
		}
		return err
	}

	// Update state in store.
	record.State = env.EnvironmentStateStopped
	if updateErr := store.Update(record); updateErr != nil {
		return updateErr
	}

	fmt.Fprintf(os.Stdout, "Environment %q stopped\n", name)
	return nil
}

// --- stub subcommands ---

func newEnvExecCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "exec <name> -- <cmd...>",
		Short: "Run a command inside an environment",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("exec: not yet implemented")
		},
	}
}

func newEnvPushCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "push <name> [local-path]",
		Short: "Push local files into an environment",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("push: not yet implemented")
		},
	}
}

func newEnvPullCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "pull <name> [remote-path]",
		Short: "Pull files from an environment",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("pull: not yet implemented")
		},
	}
}

func newEnvHarvestCmd(_ *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "harvest <name>",
		Short: "Extract work products from an environment",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("harvest: not yet implemented")
		},
	}
}

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

// cmd_context returns a background context for CLI operations.
func cmd_context() context.Context {
	return context.Background()
}

package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/cc-deck/cc-deck/internal/session"
)

// NewSnapshotCmd creates the snapshot cobra command group.
func NewSnapshotCmd(_ *GlobalFlags) *cobra.Command {
	snapshotCmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Manage session snapshots",
		Long:  "Save, restore, list, and remove session snapshots for workspace recovery.",
	}

	snapshotCmd.AddCommand(
		newSnapshotSaveCmd(),
		newSnapshotRestoreCmd(),
		newSnapshotListCmd(),
		newSnapshotRemoveCmd(),
	)

	return snapshotCmd
}

func newSnapshotSaveCmd() *cobra.Command {
	var auto bool
	cmd := &cobra.Command{
		Use:   "save [name]",
		Short: "Save current session state",
		Long:  "Capture the current workspace state (tabs, working dirs, Claude sessions) to a snapshot file.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if auto {
				return session.RunAutoSave()
			}
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			return runSnapshotSave(name)
		},
	}
	cmd.Flags().BoolVar(&auto, "auto", false, "Perform rolling auto-save with cooldown")
	_ = cmd.Flags().MarkHidden("auto")
	return cmd
}

func newSnapshotRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore [name]",
		Short: "Restore a saved snapshot",
		Long:  "Recreate tabs and start Claude sessions from a saved snapshot. Without a name, uses the most recent snapshot.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			return session.Restore(name, os.Stdout)
		},
	}
}

func newSnapshotListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short: "List saved snapshots",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSnapshotList()
		},
	}
}

func newSnapshotRemoveCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a saved snapshot",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				return runSnapshotRemoveAll()
			}
			if len(args) == 0 {
				return fmt.Errorf("specify a snapshot name or use --all")
			}
			return runSnapshotRemove(args[0])
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Remove all snapshots")
	return cmd
}

func runSnapshotSave(name string) error {
	snap, err := session.QueryPluginState(name)
	if err != nil {
		return err
	}
	if err := session.SaveSnapshot(snap); err != nil {
		return err
	}
	fmt.Printf("Saved snapshot: %s (%d sessions)\n", snap.Name, len(snap.Sessions))
	return nil
}

func runSnapshotList() error {
	infos, err := session.ListSnapshots()
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		fmt.Println("No snapshots found.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSAVED AT\tSESSIONS")
	for _, info := range infos {
		fmt.Fprintf(w, "%s\t%s\t%d\n",
			info.Name,
			info.SavedAt.Local().Format("2006-01-02 15:04:05"),
			info.SessionCount,
		)
	}
	return w.Flush()
}

func runSnapshotRemove(name string) error {
	if err := session.RemoveSnapshot(name); err != nil {
		infos, listErr := session.ListSnapshots()
		if listErr == nil && len(infos) > 0 {
			fmt.Fprintf(os.Stderr, "Available snapshots:\n")
			for _, info := range infos {
				fmt.Fprintf(os.Stderr, "  %s\n", info.Name)
			}
		}
		return err
	}
	fmt.Printf("Removed snapshot: %s\n", name)
	return nil
}

func runSnapshotRemoveAll() error {
	count, err := session.RemoveAllSnapshots()
	if err != nil {
		return err
	}
	fmt.Printf("Removed %d snapshot(s)\n", count)
	return nil
}

package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/rhuss/cc-mux/cc-deck/internal/session"
)

// NewSessionCmd creates the session cobra command group.
func NewSessionCmd(_ *GlobalFlags) *cobra.Command {
	sessionCmd := &cobra.Command{
		Use:   "session",
		Short: "Manage session snapshots",
		Long:  "Save, restore, list, and remove session snapshots for workspace recovery.",
	}

	sessionCmd.AddCommand(
		newSessionSaveCmd(),
		newSessionRestoreCmd(),
		newSessionListCmd(),
		newSessionRemoveCmd(),
	)

	return sessionCmd
}

func newSessionSaveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "save [name]",
		Short: "Save current session state",
		Long:  "Capture the current workspace state (tabs, working dirs, Claude sessions) to a snapshot file.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			return runSessionSave(name)
		},
	}
}

func newSessionRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore [name]",
		Short: "Restore a saved session snapshot",
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

func newSessionListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List saved session snapshots",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSessionList()
		},
	}
}

func newSessionRemoveCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a saved session snapshot",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				return runSessionRemoveAll()
			}
			if len(args) == 0 {
				return fmt.Errorf("specify a snapshot name or use --all")
			}
			return runSessionRemove(args[0])
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Remove all snapshots")
	return cmd
}

func runSessionSave(name string) error {
	snap, err := session.QueryPluginState(name, false)
	if err != nil {
		return err
	}
	if err := session.SaveSnapshot(snap); err != nil {
		return err
	}
	fmt.Printf("Saved session snapshot: %s (%d sessions)\n", snap.Name, len(snap.Sessions))
	return nil
}

func runSessionList() error {
	infos, err := session.ListSnapshots()
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		fmt.Println("No snapshots found.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSAVED AT\tSESSIONS\tTYPE")
	for _, info := range infos {
		snapType := "named"
		if info.AutoSave {
			snapType = "auto"
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
			info.Name,
			info.SavedAt.Local().Format("2006-01-02 15:04:05"),
			info.SessionCount,
			snapType,
		)
	}
	return w.Flush()
}

func runSessionRemove(name string) error {
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

func runSessionRemoveAll() error {
	count, err := session.RemoveAllSnapshots()
	if err != nil {
		return err
	}
	fmt.Printf("Removed %d snapshot(s)\n", count)
	return nil
}

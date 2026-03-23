package cmd

import "github.com/spf13/cobra"

// NewAttachCmd creates a top-level attach command that shares behavior
// with the env attach subcommand.
func NewAttachCmd(gf *GlobalFlags) *cobra.Command {
	return newAttachCmdCore(gf)
}

// NewListCmd creates a top-level list command that shares behavior
// with the env list subcommand.
func NewListCmd(gf *GlobalFlags) *cobra.Command {
	return newListCmdCore(gf)
}

// NewStatusCmd creates a top-level status command that shares behavior
// with the env status subcommand.
func NewStatusCmd(gf *GlobalFlags) *cobra.Command {
	return newStatusCmdCore(gf)
}

// NewStartCmd creates a top-level start command that shares behavior
// with the env start subcommand.
func NewStartCmd(gf *GlobalFlags) *cobra.Command {
	return newStartCmdCore(gf)
}

// NewStopCmd creates a top-level stop command that shares behavior
// with the env stop subcommand.
func NewStopCmd(gf *GlobalFlags) *cobra.Command {
	return newStopCmdCore(gf)
}

// NewLogsCmd creates a top-level logs command that shares behavior
// with the env logs subcommand.
func NewLogsCmd(gf *GlobalFlags) *cobra.Command {
	return newLogsCmdCore(gf)
}

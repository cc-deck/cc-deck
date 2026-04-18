package cmd

import "github.com/spf13/cobra"

func NewAttachCmd(gf *GlobalFlags) *cobra.Command {
	return newAttachCmdCore(gf)
}

func NewListCmd(gf *GlobalFlags) *cobra.Command {
	return newListCmdCore(gf)
}

func NewExecCmd(gf *GlobalFlags) *cobra.Command {
	return newExecCmdCore(gf)
}

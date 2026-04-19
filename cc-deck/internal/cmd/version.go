package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Build-time variables set via ldflags.
var (
	Version       = "dev"
	Commit        = "unknown"
	Date          = "unknown"
	ImageRegistry = "quay.io/cc-deck"
)

type versionInfo struct {
	Version string `json:"version" yaml:"version"`
	Commit  string `json:"commit" yaml:"commit"`
	Date    string `json:"date" yaml:"date"`
}

// NewVersionCmd creates the version cobra command.
func NewVersionCmd(globalFlags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printVersion(os.Stdout, globalFlags.Output)
		},
	}
}

func printVersion(w io.Writer, format string) error {
	commit := Commit
	if len(commit) > 12 {
		commit = commit[:12]
	}
	info := versionInfo{
		Version: Version,
		Commit:  commit,
		Date:    Date,
	}

	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	case "yaml":
		return yaml.NewEncoder(w).Encode(info)
	default:
		fmt.Fprintf(w, "cc-deck version %s (commit: %s, built: %s)\n", info.Version, info.Commit, info.Date)
		return nil
	}
}

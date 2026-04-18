package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func NewConfigCmd(gf *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "System configuration",
		Long:  "Commands for managing plugins, profiles, domain groups, and shell completions.",
	}

	cmd.AddCommand(NewPluginCmd(gf))
	cmd.AddCommand(NewProfileCmd(gf))
	cmd.AddCommand(NewDomainsCmd(gf))
	cmd.AddCommand(NewCompletionCmd())

	return cmd
}

func NewCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for cc-deck.

To load completions:

Bash:
  $ source <(cc-deck completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ cc-deck completion bash > /etc/bash_completion.d/cc-deck
  # macOS:
  $ cc-deck completion bash > $(brew --prefix)/etc/bash_completion.d/cc-deck

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. Execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ cc-deck completion zsh > "${fpath[1]}/_cc-deck"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ cc-deck completion fish | source

  # To load completions for each session, execute once:
  $ cc-deck completion fish > ~/.config/fish/completions/cc-deck.fish
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}
}

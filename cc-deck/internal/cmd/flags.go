package cmd

// GlobalFlags holds global CLI flags shared across all subcommands.
type GlobalFlags struct {
	Kubeconfig string
	Namespace  string
	Profile    string
	ConfigFile string
	Verbose    bool
	Output     string
}

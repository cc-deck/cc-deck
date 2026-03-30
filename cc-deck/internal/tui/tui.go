package tui

import "time"

// Options configures the TUI behavior.
type Options struct {
	PollLocal     time.Duration
	PollContainer time.Duration
	NoColor       bool
}

// Run launches the interactive TUI dashboard.
func Run(opts Options) error {
	if opts.PollLocal == 0 {
		opts.PollLocal = 2 * time.Second
	}
	if opts.PollContainer == 0 {
		opts.PollContainer = 5 * time.Second
	}

	m := newModel(opts)
	p := newProgram(m)
	_, err := p.Run()
	return err
}

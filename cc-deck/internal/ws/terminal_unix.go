//go:build !windows

package ws

import (
	"os"
	"os/signal"
	"syscall"

	v1 "github.com/rhuss/openshell-sdk-go/openshell/v1"
	"golang.org/x/term"
)

func watchTerminalResize(session v1.InteractiveSession) func() {
	resize := func() {
		fd := int(os.Stdin.Fd())
		if w, h, err := term.GetSize(fd); err == nil {
			_ = session.Resize(uint32(w), uint32(h))
		}
	}

	resize()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-sigCh:
				resize()
			case <-done:
				return
			}
		}
	}()

	return func() {
		signal.Stop(sigCh)
		close(done)
	}
}

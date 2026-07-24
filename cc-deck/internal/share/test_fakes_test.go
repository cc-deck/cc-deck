package share

import (
	"context"
	"fmt"
	"os"
	"sync"
)

type fakeProcess struct {
	pid                         int
	waitErr, signalErr, killErr error
	waitCh                      chan error
	onSignal                    func()
	onKill                      func()
}

func (p *fakeProcess) PID() int { return p.pid }
func (p *fakeProcess) Wait() error {
	if p.waitCh != nil {
		return <-p.waitCh
	}
	return p.waitErr
}
func (p *fakeProcess) Signal(os.Signal) error {
	if p.onSignal != nil {
		p.onSignal()
	}
	return p.signalErr
}
func (p *fakeProcess) Kill() error {
	if p.onKill != nil {
		p.onKill()
	}
	return p.killErr
}

type runnerCall struct {
	name string
	args []string
}
type fakeRunner struct {
	mu      sync.Mutex
	calls   []runnerCall
	outputs map[string][]byte
	errors  map[string]error
	process Process
	startFn func(string, []string) (Process, error)
	runFn   func(string, []string) ([]byte, error)
}

func key(n string, a []string) string { return fmt.Sprintf("%s %v", n, a) }
func (r *fakeRunner) Run(_ context.Context, n string, a ...string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, runnerCall{n, append([]string(nil), a...)})
	if r.runFn != nil {
		return r.runFn(n, a)
	}
	if n == "ps" && r.process != nil {
		for i := len(r.calls) - 1; i >= 0; i-- {
			for j, arg := range r.calls[i].args {
				if arg == "--logfile" && j+1 < len(r.calls[i].args) {
					return []byte("cloudflared --logfile " + r.calls[i].args[j+1]), nil
				}
			}
		}
	}
	return r.outputs[key(n, a)], r.errors[key(n, a)]
}
func (r *fakeRunner) Start(_ context.Context, n string, a ...string) (Process, error) {
	r.mu.Lock()
	r.calls = append(r.calls, runnerCall{n, append([]string(nil), a...)})
	r.mu.Unlock()
	if r.startFn != nil {
		return r.startFn(n, a)
	}
	if r.process == nil {
		return nil, fmt.Errorf("no process")
	}
	return r.process, nil
}

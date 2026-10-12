package share

import (
	"context"
	"fmt"
	"sync"
)

type runnerCall struct {
	name string
	args []string
}

// fakeRunner records bounded command invocations. There is no Start: nothing in
// sharing launches a long-lived process any more, so there is no process to
// fake.
type fakeRunner struct {
	mu      sync.Mutex
	calls   []runnerCall
	outputs map[string][]byte
	errors  map[string]error
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
	return r.outputs[key(n, a)], r.errors[key(n, a)]
}

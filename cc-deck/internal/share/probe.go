package share

import (
	"context"
	"fmt"
	"time"
)

// Probe verifies that the endpoint actually reaches a live terminal, in five
// ordered layers, stopping at the first failure.
//
// The implementation lands with the stage matrix that drives it. Until then
// this reports an unverifiable endpoint rather than a passing one, so nothing
// can accidentally treat an unimplemented probe as a successful check.
func (e *StaticEndpoint) Probe(_ context.Context, _ EndpointRef, _ string) (ProbeResult, error) {
	return ProbeResult{OK: false, CheckedAt: time.Now().UTC()},
		fmt.Errorf("endpoint verification is not implemented yet")
}

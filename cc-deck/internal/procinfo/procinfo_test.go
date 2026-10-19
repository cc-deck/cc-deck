package procinfo

import (
	"runtime"
	"testing"
	"time"
)

func TestDefaultSelfHasProcessIdentity(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("process inspection is unsupported on this platform")
	}
	p, err := Default().Get(Default().Self())
	if err != nil {
		t.Fatalf("Get(Self): %v", err)
	}
	if p.Comm == "" {
		t.Error("Comm is empty")
	}
	if p.StartUnix < time.Now().Add(-120*time.Second).Unix() || p.StartUnix > time.Now().Unix()+1 {
		t.Errorf("StartUnix = %d is not near now", p.StartUnix)
	}
}

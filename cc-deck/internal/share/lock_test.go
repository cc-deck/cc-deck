package share

import (
	"context"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStoreLockSerializesAndCancels(t *testing.T) {
	s := NewFileStore(filepath.Join(t.TempDir(), "share.yaml"))
	entered := make(chan struct{})
	release := make(chan struct{})
	go func() { _ = s.WithLock(context.Background(), func() error { close(entered); <-release; return nil }) }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	e := s.WithLock(ctx, func() error { return nil })
	require.ErrorIs(t, e, context.DeadlineExceeded)
	close(release)
	require.Eventually(t, func() bool { return s.WithLock(context.Background(), func() error { return nil }) == nil }, time.Second, 10*time.Millisecond)
}

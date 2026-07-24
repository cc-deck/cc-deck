package share

import (
	"context"
	"errors"
	"os"
	"time"
)

func (s *FileStore) WithLock(ctx context.Context, fn func() error) error {
	if err := os.MkdirAll(filepathDir(s.lockPath), 0700); err != nil {
		return err
	}
	for {
		f, err := os.OpenFile(s.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			_, _ = f.WriteString("locked\n")
			_ = f.Close()
			defer os.Remove(s.lockPath)
			return fn()
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func filepathDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if os.IsPathSeparator(path[i]) {
			if i == 0 {
				return string(path[:1])
			}
			return path[:i]
		}
	}
	return "."
}

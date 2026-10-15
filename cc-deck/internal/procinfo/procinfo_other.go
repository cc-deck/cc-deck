//go:build !darwin && !linux

package procinfo

import "os"

type unsupportedSource struct{}

func defaultSource() Source { return unsupportedSource{} }

func (unsupportedSource) Self() int { return os.Getpid() }

func (unsupportedSource) Get(int) (Process, error) { return Process{}, ErrUnsupported }

func (unsupportedSource) Ancestor(int) (Process, error) { return Process{}, ErrUnsupported }

func (unsupportedSource) ListUser() ([]Process, error) { return nil, ErrUnsupported }

package share

import (
	"fmt"
	"github.com/cc-deck/cc-deck/internal/xdg"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

type FileStore struct{ path, lockPath string }

func DefaultStatePath() string {
	if path := os.Getenv("CC_DECK_SHARE_STATE_FILE"); path != "" {
		return path
	}
	return filepath.Join(xdg.StateHome, "cc-deck", "share.yaml")
}
func NewFileStore(path string) *FileStore {
	if path == "" {
		path = DefaultStatePath()
	}
	return &FileStore{path: path, lockPath: path + ".lock"}
}
func (s *FileStore) Path() string { return s.path }
func (s *FileStore) Load() (*SharingOperation, error) {
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read sharing state: %w", err)
	}
	var op SharingOperation
	if err = yaml.Unmarshal(b, &op); err != nil {
		return nil, fmt.Errorf("parse sharing state: %w", err)
	}
	return &op, nil
}
func (s *FileStore) Save(op *SharingOperation) error {
	if op == nil {
		return fmt.Errorf("sharing operation is nil")
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create sharing state directory: %w", err)
	}
	_ = os.Chmod(dir, 0700)
	b, err := yaml.Marshal(op)
	if err != nil {
		return fmt.Errorf("marshal sharing state: %w", err)
	}
	f, err := os.CreateTemp(dir, filepath.Base(s.path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary sharing state: %w", err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace sharing state: %w", err)
	}
	return os.Chmod(s.path, 0600)
}
func (s *FileStore) Remove() error {
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove sharing state: %w", err)
	}
	return nil
}

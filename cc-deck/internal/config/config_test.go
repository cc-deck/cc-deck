package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfigPath(t *testing.T) {
	p := DefaultConfigPath()
	if filepath.Base(p) != configFile {
		t.Errorf("DefaultConfigPath() base = %q, want %q", filepath.Base(p), configFile)
	}
	if filepath.Base(filepath.Dir(p)) != configDirName {
		t.Errorf("DefaultConfigPath() dir = %q, want %q", filepath.Base(filepath.Dir(p)), configDirName)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil for missing file", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	if len(cfg.Sessions) != 0 || cfg.DefaultProfile != "" {
		t.Errorf("Load() on missing file should return zero-value Config, got %+v", cfg)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("not: [valid: yaml"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid YAML")
	}
}

func TestLoad_UnreadableFile(t *testing.T) {
	dir := t.TempDir()
	// A directory path cannot be read as a file, triggering the non-NotExist
	// branch of the ReadFile error handling.
	path := filepath.Join(dir, "subdir")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want error when path is a directory")
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.yaml")

	cfg := &Config{
		DefaultProfile: "prod",
		Sessions: []Session{
			{Name: "s1", Namespace: "ns1"},
		},
	}

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to exist after Save(): %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.DefaultProfile != cfg.DefaultProfile {
		t.Errorf("loaded.DefaultProfile = %q, want %q", loaded.DefaultProfile, cfg.DefaultProfile)
	}
	if len(loaded.Sessions) != 1 || loaded.Sessions[0].Name != "s1" {
		t.Errorf("loaded.Sessions = %+v, want one session named s1", loaded.Sessions)
	}
}

func TestSave_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "config.yaml")

	cfg := &Config{}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected nested directories to be created: %v", err)
	}
}

func TestSave_InvalidPath(t *testing.T) {
	dir := t.TempDir()
	// Create a file where Save() will try to create a directory, forcing
	// MkdirAll to fail.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to write blocker file: %v", err)
	}
	path := filepath.Join(blocker, "config.yaml")

	cfg := &Config{}
	if err := cfg.Save(path); err == nil {
		t.Fatal("Save() error = nil, want error when parent dir cannot be created")
	}
}

func TestAddSession_SetsCreatedAt(t *testing.T) {
	cfg := &Config{}
	cfg.AddSession(Session{Name: "s1"})

	if len(cfg.Sessions) != 1 {
		t.Fatalf("len(cfg.Sessions) = %d, want 1", len(cfg.Sessions))
	}
	if cfg.Sessions[0].CreatedAt == "" {
		t.Error("AddSession() should set CreatedAt when empty")
	}
	if _, err := time.Parse(time.RFC3339, cfg.Sessions[0].CreatedAt); err != nil {
		t.Errorf("CreatedAt is not a valid RFC3339 timestamp: %v", err)
	}
}

func TestAddSession_PreservesExplicitCreatedAt(t *testing.T) {
	cfg := &Config{}
	want := "2020-01-01T00:00:00Z"
	cfg.AddSession(Session{Name: "s1", CreatedAt: want})

	if cfg.Sessions[0].CreatedAt != want {
		t.Errorf("CreatedAt = %q, want %q", cfg.Sessions[0].CreatedAt, want)
	}
}

func TestAddSession_Multiple(t *testing.T) {
	cfg := &Config{}
	cfg.AddSession(Session{Name: "s1"})
	cfg.AddSession(Session{Name: "s2"})

	if len(cfg.Sessions) != 2 {
		t.Fatalf("len(cfg.Sessions) = %d, want 2", len(cfg.Sessions))
	}
}

func TestRemoveSession_Found(t *testing.T) {
	cfg := &Config{Sessions: []Session{{Name: "s1"}, {Name: "s2"}, {Name: "s3"}}}

	ok := cfg.RemoveSession("s2")
	if !ok {
		t.Fatal("RemoveSession() = false, want true")
	}
	if len(cfg.Sessions) != 2 {
		t.Fatalf("len(cfg.Sessions) = %d, want 2", len(cfg.Sessions))
	}
	for _, s := range cfg.Sessions {
		if s.Name == "s2" {
			t.Error("s2 should have been removed")
		}
	}
}

func TestRemoveSession_NotFound(t *testing.T) {
	cfg := &Config{Sessions: []Session{{Name: "s1"}}}

	ok := cfg.RemoveSession("missing")
	if ok {
		t.Fatal("RemoveSession() = true, want false for missing session")
	}
	if len(cfg.Sessions) != 1 {
		t.Errorf("len(cfg.Sessions) = %d, want 1 (unchanged)", len(cfg.Sessions))
	}
}

func TestFindSession(t *testing.T) {
	cfg := &Config{Sessions: []Session{{Name: "s1", Namespace: "ns1"}, {Name: "s2"}}}

	found := cfg.FindSession("s1")
	if found == nil {
		t.Fatal("FindSession() = nil, want non-nil")
	}
	if found.Namespace != "ns1" {
		t.Errorf("found.Namespace = %q, want ns1", found.Namespace)
	}

	// Mutating through the returned pointer should affect the config's slice.
	found.Namespace = "changed"
	if cfg.Sessions[0].Namespace != "changed" {
		t.Error("FindSession() should return a pointer into the underlying slice")
	}
}

func TestFindSession_NotFound(t *testing.T) {
	cfg := &Config{Sessions: []Session{{Name: "s1"}}}

	found := cfg.FindSession("missing")
	if found != nil {
		t.Errorf("FindSession() = %+v, want nil", found)
	}
}

func TestResolveProfile(t *testing.T) {
	tests := []struct {
		name           string
		defaultProfile string
		flagProfile    string
		want           string
	}{
		{"flag overrides default", "default-profile", "flag-profile", "flag-profile"},
		{"falls back to default", "default-profile", "", "default-profile"},
		{"both empty", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{DefaultProfile: tt.defaultProfile}
			got := cfg.ResolveProfile(tt.flagProfile)
			if got != tt.want {
				t.Errorf("ResolveProfile(%q) = %q, want %q", tt.flagProfile, got, tt.want)
			}
		})
	}
}

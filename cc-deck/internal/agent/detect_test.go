package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// withFakeExecutable creates an executable file named `name` in a temp
// directory and prepends that directory to PATH for the duration of the
// test, so that exec.LookPath(name) succeeds.
func withFakeExecutable(t *testing.T, name string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("writing fake executable: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// withEmptyPath sets PATH to a directory that contains none of the agent
// binaries, so that exec.LookPath fails for all of them.
func withEmptyPath(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

func TestClaudeAgentIsInstalled(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		withFakeExecutable(t, "claude")
		a := &ClaudeAgent{}
		if !a.IsInstalled() {
			t.Error("IsInstalled() = false, want true when claude is on PATH")
		}
	})
	t.Run("not found", func(t *testing.T) {
		withEmptyPath(t)
		a := &ClaudeAgent{}
		if a.IsInstalled() {
			t.Error("IsInstalled() = true, want false when claude is not on PATH")
		}
	})
}

func TestClaudeAgentDetectConfig(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		claudeDir := filepath.Join(home, ".claude")
		if err := os.MkdirAll(claudeDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		a := &ClaudeAgent{}
		if got := a.DetectConfig(); got != claudeDir {
			t.Errorf("DetectConfig() = %q, want %q", got, claudeDir)
		}
	})
	t.Run("missing", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		a := &ClaudeAgent{}
		if got := a.DetectConfig(); got != "" {
			t.Errorf("DetectConfig() = %q, want empty string", got)
		}
	})
}

func TestDefaultClaudeSettingsPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := filepath.Join(home, ".claude", "settings.json")
	if got := defaultClaudeSettingsPath(); got != want {
		t.Errorf("defaultClaudeSettingsPath() = %q, want %q", got, want)
	}
}

func TestCodexAgentIsInstalled(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		withFakeExecutable(t, "codex")
		a := &CodexAgent{}
		if !a.IsInstalled() {
			t.Error("IsInstalled() = false, want true when codex is on PATH")
		}
	})
	t.Run("not found", func(t *testing.T) {
		withEmptyPath(t)
		a := &CodexAgent{}
		if a.IsInstalled() {
			t.Error("IsInstalled() = true, want false when codex is not on PATH")
		}
	})
}

func TestCodexAgentDetectConfig(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		codexDir := filepath.Join(home, ".codex")
		if err := os.MkdirAll(codexDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		a := &CodexAgent{}
		if got := a.DetectConfig(); got != codexDir {
			t.Errorf("DetectConfig() = %q, want %q", got, codexDir)
		}
	})
	t.Run("missing", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		a := &CodexAgent{}
		if got := a.DetectConfig(); got != "" {
			t.Errorf("DetectConfig() = %q, want empty string", got)
		}
	})
}

func TestDefaultCodexHooksPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := filepath.Join(home, ".codex", "hooks.json")
	if got := defaultCodexHooksPath(); got != want {
		t.Errorf("defaultCodexHooksPath() = %q, want %q", got, want)
	}
}

func TestOpenCodeAgentIsInstalled(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		withFakeExecutable(t, "opencode")
		a := &OpenCodeAgent{}
		if !a.IsInstalled() {
			t.Error("IsInstalled() = false, want true when opencode is on PATH")
		}
	})
	t.Run("not found", func(t *testing.T) {
		withEmptyPath(t)
		a := &OpenCodeAgent{}
		if a.IsInstalled() {
			t.Error("IsInstalled() = true, want false when opencode is not on PATH")
		}
	})
}

func TestOpenCodeAgentDetectConfig(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		configDir := filepath.Join(home, ".config", "opencode")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		a := &OpenCodeAgent{}
		if got := a.DetectConfig(); got != configDir {
			t.Errorf("DetectConfig() = %q, want %q", got, configDir)
		}
	})
	t.Run("missing", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		a := &OpenCodeAgent{}
		if got := a.DetectConfig(); got != "" {
			t.Errorf("DetectConfig() = %q, want empty string", got)
		}
	})
}

func TestDefaultOpencodeConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := filepath.Join(home, ".config", "opencode")
	if got := defaultOpencodeConfigDir(); got != want {
		t.Errorf("defaultOpencodeConfigDir() = %q, want %q", got, want)
	}
}

func TestDefaultOpencodePluginPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := filepath.Join(home, ".config", "opencode", "plugins", "cc-deck.ts")
	if got := defaultOpencodePluginPath(); got != want {
		t.Errorf("defaultOpencodePluginPath() = %q, want %q", got, want)
	}
}

func TestDefaultOpencodeConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := filepath.Join(home, ".config", "opencode", "opencode.json")
	if got := defaultOpencodeConfigPath(); got != want {
		t.Errorf("defaultOpencodeConfigPath() = %q, want %q", got, want)
	}
}

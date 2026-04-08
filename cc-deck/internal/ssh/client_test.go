package ssh

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("user@host", 2222, "/path/key", "bastion", "/path/config")

	if c.Host != "user@host" {
		t.Errorf("Host = %q, want %q", c.Host, "user@host")
	}
	if c.Port != 2222 {
		t.Errorf("Port = %d, want %d", c.Port, 2222)
	}
	if c.IdentityFile != "/path/key" {
		t.Errorf("IdentityFile = %q, want %q", c.IdentityFile, "/path/key")
	}
	if c.JumpHost != "bastion" {
		t.Errorf("JumpHost = %q, want %q", c.JumpHost, "bastion")
	}
	if c.SSHConfig != "/path/config" {
		t.Errorf("SSHConfig = %q, want %q", c.SSHConfig, "/path/config")
	}
}

func TestBuildArgs_AllOptions(t *testing.T) {
	c := NewClient("user@host", 2222, "/path/key", "bastion", "/path/config")
	args := c.buildArgs("user@host", "--", "echo hello")

	expected := []string{
		"-F", "/path/config",
		"-p", "2222",
		"-i", "/path/key",
		"-J", "bastion",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "BatchMode=yes",
		"user@host", "--", "echo hello",
	}

	if len(args) != len(expected) {
		t.Fatalf("buildArgs returned %d args, want %d: %v", len(args), len(expected), args)
	}
	for i, arg := range args {
		if arg != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, arg, expected[i])
		}
	}
}

func TestBuildArgs_MinimalOptions(t *testing.T) {
	c := NewClient("user@host", 0, "", "", "")
	args := c.buildArgs("user@host", "--", "ls")

	expected := []string{
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "BatchMode=yes",
		"user@host", "--", "ls",
	}

	if len(args) != len(expected) {
		t.Fatalf("buildArgs returned %d args, want %d: %v", len(args), len(expected), args)
	}
	for i, arg := range args {
		if arg != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, arg, expected[i])
		}
	}
}

func TestNormalizeArch(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"x86_64", "amd64"},
		{"aarch64", "arm64"},
		{"armv7l", "armv7l"},
		{"i686", "i686"},
	}

	for _, tt := range tests {
		got := normalizeArch(tt.input)
		if got != tt.want {
			t.Errorf("normalizeArch(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

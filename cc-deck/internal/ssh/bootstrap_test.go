package ssh

import (
	"testing"
)

func TestConnectivityCheck_Name(t *testing.T) {
	c := &ConnectivityCheck{client: NewClient("user@host", 0, "", "", "")}
	if c.Name() != "SSH connectivity" {
		t.Errorf("Name() = %q, want %q", c.Name(), "SSH connectivity")
	}
	if c.HasRemedy() {
		t.Error("ConnectivityCheck should not have a remedy")
	}
}

func TestOSDetectionCheck_Name(t *testing.T) {
	c := &OSDetectionCheck{client: NewClient("user@host", 0, "", "", "")}
	if c.Name() != "OS/architecture detection" {
		t.Errorf("Name() = %q, want %q", c.Name(), "OS/architecture detection")
	}
	if c.HasRemedy() {
		t.Error("OSDetectionCheck should not have a remedy")
	}
}

func TestZellijCheck_HasRemedy(t *testing.T) {
	tests := []struct {
		os   string
		want bool
	}{
		{"linux", true},
		{"darwin", false},
		{"", false},
	}

	for _, tt := range tests {
		c := &ZellijCheck{client: NewClient("host", 0, "", "", ""), os: tt.os}
		if c.HasRemedy() != tt.want {
			t.Errorf("ZellijCheck(os=%q).HasRemedy() = %v, want %v", tt.os, c.HasRemedy(), tt.want)
		}
	}
}

func TestClaudeCodeCheck_HasRemedy(t *testing.T) {
	c := &ClaudeCodeCheck{client: NewClient("host", 0, "", "", "")}
	if !c.HasRemedy() {
		t.Error("ClaudeCodeCheck should have a remedy")
	}
}

func TestCcDeckCheck_HasRemedy(t *testing.T) {
	c := &CcDeckCheck{client: NewClient("host", 0, "", "", "")}
	if !c.HasRemedy() {
		t.Error("CcDeckCheck should have a remedy")
	}
}

func TestPluginCheck_HasRemedy(t *testing.T) {
	c := &PluginCheck{client: NewClient("host", 0, "", "", "")}
	if !c.HasRemedy() {
		t.Error("PluginCheck should have a remedy")
	}
}

func TestCredentialCheck_ValidModes(t *testing.T) {
	validModes := []string{"", "auto", "api", "vertex", "bedrock", "none"}
	for _, mode := range validModes {
		c := &CredentialCheck{client: NewClient("host", 0, "", "", ""), authMode: mode}
		if err := c.Run(nil); err != nil {
			t.Errorf("CredentialCheck(%q).Run() error = %v, want nil", mode, err)
		}
	}
}

func TestCredentialCheck_InvalidMode(t *testing.T) {
	c := &CredentialCheck{client: NewClient("host", 0, "", "", ""), authMode: "invalid"}
	if err := c.Run(nil); err == nil {
		t.Error("CredentialCheck(invalid).Run() should return error")
	}
}

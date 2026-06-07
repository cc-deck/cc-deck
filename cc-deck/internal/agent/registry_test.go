package agent

import (
	"testing"
)

// stubAgent is a minimal Agent implementation for testing the registry.
type stubAgent struct {
	name        string
	displayName string
	indicator   string
}

func (s *stubAgent) Name() string        { return s.name }
func (s *stubAgent) DisplayName() string { return s.displayName }
func (s *stubAgent) Indicator() string   { return s.indicator }
func (s *stubAgent) IsInstalled() bool   { return false }
func (s *stubAgent) DetectConfig() string { return "" }
func (s *stubAgent) InstallHooks() error { return nil }
func (s *stubAgent) UninstallHooks() error { return nil }
func (s *stubAgent) HooksInstalled() bool { return false }
func (s *stubAgent) TranslateEvent(_ []byte) (*NormalizedPayload, error) { return nil, nil }

func TestRegisterAndGet(t *testing.T) {
	Reset()

	a := &stubAgent{name: "test", displayName: "Test Agent", indicator: "TA"}
	Register(a)

	got := Get("test")
	if got == nil {
		t.Fatal("expected agent, got nil")
	}
	if got.Name() != "test" {
		t.Errorf("Name() = %q, want %q", got.Name(), "test")
	}
	if got.DisplayName() != "Test Agent" {
		t.Errorf("DisplayName() = %q, want %q", got.DisplayName(), "Test Agent")
	}
	if got.Indicator() != "TA" {
		t.Errorf("Indicator() = %q, want %q", got.Indicator(), "TA")
	}
}

func TestGetNotFound(t *testing.T) {
	Reset()

	got := Get("nonexistent")
	if got != nil {
		t.Errorf("expected nil for unknown agent, got %v", got)
	}
}

func TestRegisterDuplicateNamePanics(t *testing.T) {
	Reset()

	Register(&stubAgent{name: "dup", displayName: "First", indicator: "D1"})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate name, got none")
		}
	}()

	Register(&stubAgent{name: "dup", displayName: "Second", indicator: "D2"})
}

func TestRegisterDuplicateIndicatorPanics(t *testing.T) {
	Reset()

	Register(&stubAgent{name: "first", displayName: "First", indicator: "XX"})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate indicator, got none")
		}
	}()

	Register(&stubAgent{name: "second", displayName: "Second", indicator: "XX"})
}

func TestAllStableOrdering(t *testing.T) {
	Reset()

	Register(&stubAgent{name: "zeta", displayName: "Zeta", indicator: "ZZ"})
	Register(&stubAgent{name: "alpha", displayName: "Alpha", indicator: "AA"})
	Register(&stubAgent{name: "mid", displayName: "Mid", indicator: "MM"})

	all := All()
	if len(all) != 3 {
		t.Fatalf("All() returned %d agents, want 3", len(all))
	}
	if all[0].Name() != "alpha" {
		t.Errorf("All()[0].Name() = %q, want %q", all[0].Name(), "alpha")
	}
	if all[1].Name() != "mid" {
		t.Errorf("All()[1].Name() = %q, want %q", all[1].Name(), "mid")
	}
	if all[2].Name() != "zeta" {
		t.Errorf("All()[2].Name() = %q, want %q", all[2].Name(), "zeta")
	}
}

func TestAllEmpty(t *testing.T) {
	Reset()

	all := All()
	if len(all) != 0 {
		t.Errorf("All() returned %d agents on empty registry, want 0", len(all))
	}
}

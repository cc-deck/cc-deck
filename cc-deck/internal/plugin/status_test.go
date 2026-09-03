package plugin

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestYesNo(t *testing.T) {
	if got := yesNo(true); got != "yes" {
		t.Errorf("yesNo(true) = %q, want %q", got, "yes")
	}
	if got := yesNo(false); got != "no" {
		t.Errorf("yesNo(false) = %q, want %q", got, "no")
	}
}

func TestHumanSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero bytes", 0, "0 B"},
		{"small byte count", 512, "512 B"},
		{"just under a KB", 1023, "1023 B"},
		{"exactly one KB", 1024, "1.0 KB"},
		{"fractional KB", 1536, "1.5 KB"},
		{"just under a MB", 1024*1024 - 1, "1024.0 KB"},
		{"exactly one MB", 1024 * 1024, "1.0 MB"},
		{"fractional MB", 5 * 1024 * 1024, "5.0 MB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := humanSize(tt.bytes); got != tt.want {
				t.Errorf("humanSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestTildeHome(t *testing.T) {
	origHomeDirFunc := homeDirFunc
	defer func() { homeDirFunc = origHomeDirFunc }()

	homeDirFunc = func() (string, error) { return "/home/user", nil }

	if got := tildeHome("/home/user/.config/zellij"); got != "~/.config/zellij" {
		t.Errorf("tildeHome() = %q, want %q", got, "~/.config/zellij")
	}
	if got := tildeHome("/other/path"); got != "/other/path" {
		t.Errorf("tildeHome() with non-matching prefix = %q, want unchanged path", got)
	}
}

func TestTildeHome_HomeDirError(t *testing.T) {
	origHomeDirFunc := homeDirFunc
	defer func() { homeDirFunc = origHomeDirFunc }()

	homeDirFunc = func() (string, error) { return "", errors.New("no home dir") }

	if got := tildeHome("/home/user/file"); got != "/home/user/file" {
		t.Errorf("tildeHome() with homeDir error = %q, want unchanged path", got)
	}
}

func TestExpectedPluginPath(t *testing.T) {
	installed := ZellijInfo{Installed: true, PluginsDir: "/home/user/.config/zellij/plugins"}
	want := "/home/user/.config/zellij/plugins/cc_deck.wasm"
	if got := expectedPluginPath(installed); got != want {
		t.Errorf("expectedPluginPath(installed) = %q, want %q", got, want)
	}

	notInstalled := ZellijInfo{Installed: false}
	if got := expectedPluginPath(notInstalled); got != "" {
		t.Errorf("expectedPluginPath(not installed) = %q, want empty string", got)
	}
}

func TestStatusReport_FormatText(t *testing.T) {
	report := StatusReport{
		Plugin: PluginStatus{Installed: true, Path: "/home/user/.config/zellij/plugins/cc_deck.wasm", Size: 2048, Version: "0.8.0"},
		Zellij: ZellijStatus{Installed: true, Version: "0.43.1", Compatibility: "compatible", ConfigDir: "/home/user/.config/zellij"},
		Layouts: LayoutStatus{
			CcDeckLayout:    "minimal",
			DefaultInjected: true,
		},
		Hooks: HookStatus{Registered: true, EventCount: 3},
	}

	var buf bytes.Buffer
	if err := report.FormatText(&buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"Installed:      yes", "0.8.0", "0.43.1", "compatible", "minimal", "injected", "yes (3 event types)"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatText() output missing %q; got:\n%s", want, out)
		}
	}
}

func TestStatusReport_FormatText_NotInstalled(t *testing.T) {
	report := StatusReport{
		Plugin: PluginStatus{Installed: false, Path: "/expected/path/cc_deck.wasm"},
		Zellij: ZellijStatus{Installed: false},
		Layouts: LayoutStatus{
			DefaultInjected: false,
		},
		Hooks: HookStatus{Registered: false},
	}

	var buf bytes.Buffer
	if err := report.FormatText(&buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"Installed:      no", "Expected path:", "not installed", "not injected", "Registered:     no"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatText() output missing %q; got:\n%s", want, out)
		}
	}
}

func TestStatusReport_FormatJSON(t *testing.T) {
	report := StatusReport{Plugin: PluginStatus{Installed: true, Version: "0.8.0"}}
	var buf bytes.Buffer
	if err := report.FormatJSON(&buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"installed": true`) {
		t.Errorf("FormatJSON() output missing expected field; got:\n%s", out)
	}
	if !strings.Contains(out, `"0.8.0"`) {
		t.Errorf("FormatJSON() output missing version; got:\n%s", out)
	}
}

func TestStatusReport_FormatYAML(t *testing.T) {
	report := StatusReport{Plugin: PluginStatus{Installed: true, Version: "0.8.0"}}
	var buf bytes.Buffer
	if err := report.FormatYAML(&buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "installed: true") {
		t.Errorf("FormatYAML() output missing expected field; got:\n%s", out)
	}
	if !strings.Contains(out, "0.8.0") {
		t.Errorf("FormatYAML() output missing version; got:\n%s", out)
	}
}

func TestRunStatus_FormatSelection(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := RunStatus(&out, &errOut, "json"); err != nil {
		t.Fatalf("unexpected error for json format: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(out.String()), "{") {
		t.Errorf("RunStatus with json format should produce JSON, got:\n%s", out.String())
	}

	out.Reset()
	errOut.Reset()
	if err := RunStatus(&out, &errOut, "yaml"); err != nil {
		t.Fatalf("unexpected error for yaml format: %v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(out.String()), "{") {
		t.Errorf("RunStatus with yaml format should not produce JSON, got:\n%s", out.String())
	}

	out.Reset()
	errOut.Reset()
	if err := RunStatus(&out, &errOut, "text"); err != nil {
		t.Fatalf("unexpected error for text format: %v", err)
	}
	if !strings.Contains(out.String(), "Plugin Status") {
		t.Errorf("RunStatus with text format should produce text output, got:\n%s", out.String())
	}
}

func TestHomeDir(t *testing.T) {
	// Sanity check that homeDir delegates to homeDirFunc.
	origHomeDirFunc := homeDirFunc
	defer func() { homeDirFunc = origHomeDirFunc }()

	homeDirFunc = func() (string, error) { return filepath.Join("test", "home"), nil }
	got, err := homeDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != filepath.Join("test", "home") {
		t.Errorf("homeDir() = %q, want %q", got, filepath.Join("test", "home"))
	}
}

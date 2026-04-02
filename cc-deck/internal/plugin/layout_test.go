package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestControllerConfigBlock_ContainsMarkers(t *testing.T) {
	block := controllerConfigBlock("/home/user/.config/zellij/plugins")
	if !strings.Contains(block, ConfigInjectionStart) {
		t.Error("block missing start marker")
	}
	if !strings.Contains(block, ConfigInjectionEnd) {
		t.Error("block missing end marker")
	}
	if !strings.Contains(block, "cc_deck.wasm") {
		t.Error("block missing controller wasm reference")
	}
}

func TestHasControllerConfig(t *testing.T) {
	block := controllerConfigBlock("/plugins")
	if !HasControllerConfig(block) {
		t.Error("should detect markers in block")
	}
	if HasControllerConfig("some random config content") {
		t.Error("should not detect markers in unrelated content")
	}
	if HasControllerConfig("") {
		t.Error("should not detect markers in empty string")
	}
	// Only start marker, no end
	if HasControllerConfig(ConfigInjectionStart) {
		t.Error("should require both markers")
	}
}

func TestInjectControllerConfig_EmptyContent(t *testing.T) {
	result := InjectControllerConfig("", "/plugins")
	if !HasControllerConfig(result) {
		t.Error("should contain markers after injection into empty content")
	}
	if !strings.Contains(result, "cc_deck.wasm") {
		t.Error("should contain controller reference")
	}
}

func TestInjectControllerConfig_ExistingContent(t *testing.T) {
	existing := "theme \"catppuccin\"\ndefault_layout \"compact\"\n"
	result := InjectControllerConfig(existing, "/plugins")
	if !strings.Contains(result, "theme \"catppuccin\"") {
		t.Error("should preserve existing content")
	}
	if !HasControllerConfig(result) {
		t.Error("should contain controller block")
	}
}

func TestInjectControllerConfig_Idempotent(t *testing.T) {
	content := "some config\n"
	first := InjectControllerConfig(content, "/plugins")
	second := InjectControllerConfig(first, "/plugins")
	if first != second {
		t.Errorf("not idempotent:\nfirst:  %q\nsecond: %q", first, second)
	}
}

func TestInjectControllerConfig_UpdatesPath(t *testing.T) {
	content := InjectControllerConfig("", "/old/path")
	if !strings.Contains(content, "/old/path") {
		t.Fatal("should contain old path")
	}
	updated := InjectControllerConfig(content, "/new/path")
	if strings.Contains(updated, "/old/path") {
		t.Error("should have replaced old path")
	}
	if !strings.Contains(updated, "/new/path") {
		t.Error("should contain new path")
	}
	// Should still have exactly one block
	if strings.Count(updated, ConfigInjectionStart) != 1 {
		t.Error("should have exactly one start marker")
	}
}

func TestRemoveControllerConfig_RemovesBlock(t *testing.T) {
	content := "before\n" + controllerConfigBlock("/plugins") + "after\n"
	result := RemoveControllerConfig(content)
	if HasControllerConfig(result) {
		t.Error("should not contain markers after removal")
	}
	if !strings.Contains(result, "before") {
		t.Error("should preserve content before block")
	}
	if !strings.Contains(result, "after") {
		t.Error("should preserve content after block")
	}
}

func TestRemoveControllerConfig_NoMarkers(t *testing.T) {
	content := "no markers here\n"
	result := RemoveControllerConfig(content)
	if result != content {
		t.Error("should return content unchanged when no markers present")
	}
}

func TestRemoveControllerConfig_EmptyContent(t *testing.T) {
	result := RemoveControllerConfig("")
	if result != "" {
		t.Error("should return empty string for empty input")
	}
}

func TestEnsureControllerInConfig_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.kdl")

	err := ensureControllerInConfig(configPath, "/plugins")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if !HasControllerConfig(string(content)) {
		t.Error("created file should contain controller block")
	}
}

func TestEnsureControllerInConfig_Idempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.kdl")
	os.WriteFile(configPath, []byte("theme \"default\"\n"), 0644)

	ensureControllerInConfig(configPath, "/plugins")
	first, _ := os.ReadFile(configPath)

	ensureControllerInConfig(configPath, "/plugins")
	second, _ := os.ReadFile(configPath)

	if string(first) != string(second) {
		t.Error("should be idempotent across calls")
	}
}

func TestRemoveControllerFromConfig_CleansUp(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.kdl")

	original := "theme \"default\"\n"
	os.WriteFile(configPath, []byte(original), 0644)

	ensureControllerInConfig(configPath, "/plugins")
	removeControllerFromConfig(configPath)

	content, _ := os.ReadFile(configPath)
	if HasControllerConfig(string(content)) {
		t.Error("should not contain controller block after removal")
	}
	if !strings.Contains(string(content), "theme \"default\"") {
		t.Error("should preserve original content")
	}
}

func TestInjectControllerConfig_MergesIntoExistingLoadPlugins(t *testing.T) {
	existing := "theme \"default\"\nload_plugins {\n}\n"
	result := InjectControllerConfig(existing, "/plugins")

	// Should NOT have two load_plugins blocks
	count := strings.Count(result, "load_plugins")
	if count != 1 {
		t.Errorf("expected 1 load_plugins block, got %d:\n%s", count, result)
	}
	if !strings.Contains(result, "cc_deck.wasm") {
		t.Error("should contain controller entry")
	}
	if !HasControllerConfig(result) {
		t.Error("should have injection markers")
	}
}

func TestInjectControllerConfig_MergesIdempotent(t *testing.T) {
	existing := "load_plugins {\n    \"file:other-plugin.wasm\"\n}\n"
	first := InjectControllerConfig(existing, "/plugins")
	second := InjectControllerConfig(first, "/plugins")
	if first != second {
		t.Errorf("not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestRemoveControllerConfig_FromMergedBlock(t *testing.T) {
	existing := "load_plugins {\n}\n"
	injected := InjectControllerConfig(existing, "/plugins")
	removed := RemoveControllerConfig(injected)
	if strings.Contains(removed, "cc_deck.wasm") {
		t.Error("controller entry should be removed")
	}
	if !strings.Contains(removed, "load_plugins") {
		t.Error("load_plugins block should remain (without controller)")
	}
}

func TestRemoveControllerFromConfig_MissingFile(t *testing.T) {
	err := removeControllerFromConfig("/nonexistent/config.kdl")
	if err != nil {
		t.Errorf("should return nil for missing file, got: %v", err)
	}
}

package sync

import (
	"testing"
)

func TestMergeExcludes_DefaultsOnly(t *testing.T) {
	result := mergeExcludes(nil)

	if len(result) != len(DefaultExcludes) {
		t.Fatalf("expected %d excludes, got %d", len(DefaultExcludes), len(result))
	}
	for i, exc := range DefaultExcludes {
		if result[i] != exc {
			t.Errorf("expected exclude[%d]=%q, got %q", i, exc, result[i])
		}
	}
}

func TestMergeExcludes_WithUserExcludes(t *testing.T) {
	user := []string{"vendor", "dist"}
	result := mergeExcludes(user)

	expected := len(DefaultExcludes) + len(user)
	if len(result) != expected {
		t.Fatalf("expected %d excludes, got %d: %v", expected, len(result), result)
	}

	// Verify defaults come first
	for i, exc := range DefaultExcludes {
		if result[i] != exc {
			t.Errorf("expected default exclude[%d]=%q, got %q", i, exc, result[i])
		}
	}

	// Verify user excludes come after
	for i, exc := range user {
		idx := len(DefaultExcludes) + i
		if result[idx] != exc {
			t.Errorf("expected user exclude[%d]=%q, got %q", idx, exc, result[idx])
		}
	}
}

func TestMergeExcludes_NoDuplicates(t *testing.T) {
	// Include a default in user excludes - should not be duplicated
	user := []string{".git", "vendor"}
	result := mergeExcludes(user)

	expected := len(DefaultExcludes) + 1 // only "vendor" is new
	if len(result) != expected {
		t.Fatalf("expected %d excludes (no duplicates), got %d: %v", expected, len(result), result)
	}
}

func TestBuildTarCreateArgs(t *testing.T) {
	excludes := []string{".git", "node_modules"}
	localDir := "/home/user/project"

	args := buildTarCreateArgs(excludes, localDir)

	expected := []string{"-cf", "-", "--exclude", ".git", "--exclude", "node_modules", "-C", localDir, "."}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, arg := range expected {
		if args[i] != arg {
			t.Errorf("args[%d]: expected %q, got %q", i, arg, args[i])
		}
	}
}

func TestBuildTarCreateArgs_NoExcludes(t *testing.T) {
	args := buildTarCreateArgs(nil, "/tmp/src")

	expected := []string{"-cf", "-", "-C", "/tmp/src", "."}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, arg := range expected {
		if args[i] != arg {
			t.Errorf("args[%d]: expected %q, got %q", i, arg, args[i])
		}
	}
}

func TestDefaultExcludes(t *testing.T) {
	expectedDefaults := map[string]bool{
		".git":        true,
		"node_modules": true,
		"target":       true,
		"__pycache__":  true,
	}

	for _, exc := range DefaultExcludes {
		if !expectedDefaults[exc] {
			t.Errorf("unexpected default exclude: %q", exc)
		}
	}

	if len(DefaultExcludes) != len(expectedDefaults) {
		t.Errorf("expected %d default excludes, got %d", len(expectedDefaults), len(DefaultExcludes))
	}
}

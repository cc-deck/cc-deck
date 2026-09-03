package voice

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModelDir(t *testing.T) {
	dir := ModelDir()
	if dir == "" {
		t.Fatal("ModelDir returned empty string")
	}
	if filepath.Base(dir) != "models" {
		t.Errorf("ModelDir() = %q, want basename %q", dir, "models")
	}
	if filepath.Base(filepath.Dir(dir)) != "cc-deck" {
		t.Errorf("ModelDir() parent = %q, want %q", filepath.Dir(dir), "cc-deck")
	}
}

func TestModelPath_KnownModel(t *testing.T) {
	got := ModelPath("tiny.en")
	want := filepath.Join(ModelDir(), "ggml-tiny.en.bin")
	if got != want {
		t.Errorf("ModelPath(tiny.en) = %q, want %q", got, want)
	}
}

func TestModelPath_UnknownModel(t *testing.T) {
	got := ModelPath("weird-name")
	want := filepath.Join(ModelDir(), "ggml-weird-name.bin")
	if got != want {
		t.Errorf("ModelPath(unknown) = %q, want %q", got, want)
	}
}

func TestModelPath_UnknownModelSanitizesPath(t *testing.T) {
	got := ModelPath("../../etc/passwd")
	want := filepath.Join(ModelDir(), "ggml-passwd.bin")
	if got != want {
		t.Errorf("ModelPath(traversal) = %q, want %q", got, want)
	}
}

func TestValidateModel_UnknownModel(t *testing.T) {
	err := ValidateModel("does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
}

func TestValidateModel_MissingFile(t *testing.T) {
	// ModelDir resolves from a package-level xdg.CacheHome set at init
	// time, so it can't be overridden per-test via os.Setenv. Skip if a
	// previous setup run happens to have already cached this model.
	if _, err := os.Stat(ModelPath("tiny.en")); err == nil {
		t.Skip("tiny.en model already present in cache; skipping missing-file case")
	}
	err := ValidateModel("tiny.en")
	if err == nil {
		t.Fatal("expected error for missing model file")
	}
}

func TestCheckDependency_DoesNotPanic(t *testing.T) {
	// checkDependency only prints to stdout; verify it handles both a
	// present and an absent binary without panicking.
	checkDependency("go")
	checkDependency("definitely-not-a-real-binary-xyz")
}

func TestReadWriteSHAFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "model.bin.sha256")

	if err := writeSHAFile(path, "deadbeef"); err != nil {
		t.Fatalf("writeSHAFile: %v", err)
	}

	got, err := readSHAFile(path)
	if err != nil {
		t.Fatalf("readSHAFile: %v", err)
	}
	if got != "deadbeef" {
		t.Errorf("readSHAFile() = %q, want %q", got, "deadbeef")
	}
}

func TestReadSHAFile_MissingFile(t *testing.T) {
	_, err := readSHAFile(filepath.Join(t.TempDir(), "missing.sha256"))
	if err == nil {
		t.Fatal("expected error for missing SHA file")
	}
}

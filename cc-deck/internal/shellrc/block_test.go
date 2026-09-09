package shellrc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsure_CreateNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rc")

	changed, err := Ensure(path, "echo hello")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true for new file")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "# >>> cc-deck >>>\necho hello\n# <<< cc-deck <<<\n"
	if string(got) != want {
		t.Errorf("content mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestEnsure_UpdateExistingBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rc")

	initial := "before\n# >>> cc-deck >>>\nold content\n# <<< cc-deck <<<\nafter\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := Ensure(path, "new content")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true for updated block")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "before\n# >>> cc-deck >>>\nnew content\n# <<< cc-deck <<<\nafter\n"
	if string(got) != want {
		t.Errorf("content mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestEnsure_NoOpWhenContentMatches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rc")

	content := "# >>> cc-deck >>>\nsame\n# <<< cc-deck <<<\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := Ensure(path, "same")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if changed {
		t.Fatal("expected changed=false when content matches")
	}
}

func TestEnsure_PreservesSurroundingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rc")

	// Include varied surrounding content with special characters.
	before := "# custom stuff\nexport FOO=bar\n\n"
	after := "\n# more stuff\nalias ll='ls -la'\n"
	initial := before + "# >>> cc-deck >>>\nold\n# <<< cc-deck <<<\n" + after
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := Ensure(path, "new")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := before + "# >>> cc-deck >>>\nnew\n# <<< cc-deck <<<\n" + after
	if string(got) != want {
		t.Errorf("surrounding content not preserved\ngot:  %q\nwant: %q", got, want)
	}
}

func TestEnsure_AppendWhenNoMarkers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rc")

	existing := "# existing config\nexport BAR=baz\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := Ensure(path, "echo added")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true for append")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := existing + "# >>> cc-deck >>>\necho added\n# <<< cc-deck <<<\n"
	if string(got) != want {
		t.Errorf("content mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestEnsure_AppendAddsNewlineIfMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rc")

	// File does NOT end with a newline.
	existing := "no trailing newline"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := Ensure(path, "echo fix")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := existing + "\n# >>> cc-deck >>>\necho fix\n# <<< cc-deck <<<\n"
	if string(got) != want {
		t.Errorf("content mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestEnsureAll(t *testing.T) {
	home := t.TempDir()

	changed, err := EnsureAll(home)
	if err != nil {
		t.Fatalf("EnsureAll: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}

	wantBlock := "# >>> cc-deck >>>\n" + pathContent + "\n# <<< cc-deck <<<\n"
	for _, name := range []string{".bashrc", ".zshrc"} {
		got, err := os.ReadFile(filepath.Join(home, name))
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", name, err)
		}
		if string(got) != wantBlock {
			t.Errorf("%s content mismatch\ngot:  %q\nwant: %q", name, got, wantBlock)
		}
	}

	// Second call should be a no-op.
	changed, err = EnsureAll(home)
	if err != nil {
		t.Fatalf("EnsureAll (2nd): %v", err)
	}
	if changed {
		t.Fatal("expected changed=false on second call")
	}
}

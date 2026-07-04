package voice

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile_Terms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "glossary.txt")
	content := "Kubernetes\nHelm\nIngress\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	terms, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile error: %v", err)
	}
	want := []string{"Kubernetes", "Helm", "Ingress"}
	if len(terms) != len(want) {
		t.Fatalf("got %d terms, want %d", len(terms), len(want))
	}
	for i, term := range terms {
		if term != want[i] {
			t.Errorf("term[%d] = %q, want %q", i, term, want[i])
		}
	}
}

func TestLoadFile_CommentsAndBlanks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "glossary.txt")
	content := "# This is a comment\nKubernetes\n\n# Another comment\nHelm\n  \nIngress\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	terms, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile error: %v", err)
	}
	want := []string{"Kubernetes", "Helm", "Ingress"}
	if len(terms) != len(want) {
		t.Fatalf("got %d terms, want %d: %v", len(terms), len(want), terms)
	}
	for i, term := range terms {
		if term != want[i] {
			t.Errorf("term[%d] = %q, want %q", i, term, want[i])
		}
	}
}

func TestLoadFile_MissingFile(t *testing.T) {
	terms, err := LoadFile("/nonexistent/path/glossary.txt")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if terms != nil {
		t.Errorf("expected nil terms for missing file, got: %v", terms)
	}
}

func TestLoadFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "glossary.txt")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	terms, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile error: %v", err)
	}
	if len(terms) != 0 {
		t.Errorf("expected 0 terms for empty file, got %d", len(terms))
	}
}

func TestLoadFile_WhitespaceOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "glossary.txt")
	content := "  \n\n  \n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	terms, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile error: %v", err)
	}
	if len(terms) != 0 {
		t.Errorf("expected 0 terms for whitespace-only file, got %d", len(terms))
	}
}

func TestLoadFile_TrimWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "glossary.txt")
	content := "  Kubernetes  \n  Helm  \n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	terms, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile error: %v", err)
	}
	want := []string{"Kubernetes", "Helm"}
	if len(terms) != len(want) {
		t.Fatalf("got %d terms, want %d", len(terms), len(want))
	}
	for i, term := range terms {
		if term != want[i] {
			t.Errorf("term[%d] = %q, want %q", i, term, want[i])
		}
	}
}

func TestDedup_NoDuplicates(t *testing.T) {
	result := dedup([]string{"Kubernetes", "gRPC"}, []string{"Helm", "Ingress"})
	want := []string{"Kubernetes", "gRPC", "Helm", "Ingress"}
	if len(result) != len(want) {
		t.Fatalf("got %d terms, want %d: %v", len(result), len(want), result)
	}
	for i, term := range result {
		if term != want[i] {
			t.Errorf("term[%d] = %q, want %q", i, term, want[i])
		}
	}
}

func TestDedup_CaseInsensitive_ProjectWins(t *testing.T) {
	// Global has "kubernetes", project has "Kubernetes" - project casing wins
	result := dedup([]string{"kubernetes", "gRPC"}, []string{"Kubernetes", "Helm"})
	want := []string{"Kubernetes", "gRPC", "Helm"}
	if len(result) != len(want) {
		t.Fatalf("got %d terms, want %d: %v", len(result), len(want), result)
	}
	for i, term := range result {
		if term != want[i] {
			t.Errorf("term[%d] = %q, want %q", i, term, want[i])
		}
	}
}

func TestDedup_EmptySlices(t *testing.T) {
	tests := []struct {
		name   string
		first  []string
		second []string
		want   int
	}{
		{"both empty", nil, nil, 0},
		{"first empty", nil, []string{"Helm"}, 1},
		{"second empty", []string{"Kubernetes"}, nil, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dedup(tt.first, tt.second)
			if len(result) != tt.want {
				t.Errorf("got %d terms, want %d: %v", len(result), tt.want, result)
			}
		})
	}
}

func TestResolvePrompt_GlobalOnly(t *testing.T) {
	g := NewGlossary([]string{"Kubernetes", "gRPC"})
	prompt := g.ResolvePrompt("")
	want := "Kubernetes, gRPC"
	if prompt != want {
		t.Errorf("prompt = %q, want %q", prompt, want)
	}
}

func TestResolvePrompt_NoTerms(t *testing.T) {
	g := NewGlossary(nil)
	prompt := g.ResolvePrompt("")
	if prompt != "" {
		t.Errorf("prompt = %q, want empty", prompt)
	}
}

func TestResolvePrompt_ProjectTerms(t *testing.T) {
	dir := t.TempDir()
	ccDir := filepath.Join(dir, ".cc-deck")
	if err := os.MkdirAll(ccDir, 0o755); err != nil {
		t.Fatal(err)
	}
	glossaryPath := filepath.Join(ccDir, "voice-glossary.txt")
	if err := os.WriteFile(glossaryPath, []byte("Helm\nIngress\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := NewGlossary([]string{"Kubernetes", "gRPC"})
	prompt := g.ResolvePrompt(dir)
	want := "Kubernetes, gRPC, Helm, Ingress"
	if prompt != want {
		t.Errorf("prompt = %q, want %q", prompt, want)
	}
}

func TestResolvePrompt_DedupWithProjectCasingWins(t *testing.T) {
	dir := t.TempDir()
	ccDir := filepath.Join(dir, ".cc-deck")
	if err := os.MkdirAll(ccDir, 0o755); err != nil {
		t.Fatal(err)
	}
	glossaryPath := filepath.Join(ccDir, "voice-glossary.txt")
	// Project has "KUBERNETES" which should replace global "Kubernetes"
	if err := os.WriteFile(glossaryPath, []byte("KUBERNETES\nHelm\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := NewGlossary([]string{"Kubernetes", "gRPC"})
	prompt := g.ResolvePrompt(dir)
	want := "KUBERNETES, gRPC, Helm"
	if prompt != want {
		t.Errorf("prompt = %q, want %q", prompt, want)
	}
}

func TestResolvePrompt_MissingProjectGlossary(t *testing.T) {
	dir := t.TempDir()
	// No .cc-deck/voice-glossary.txt exists

	g := NewGlossary([]string{"Kubernetes"})
	prompt := g.ResolvePrompt(dir)
	want := "Kubernetes"
	if prompt != want {
		t.Errorf("prompt = %q, want %q", prompt, want)
	}
}

func TestResolvePrompt_CachesProjectTerms(t *testing.T) {
	dir := t.TempDir()
	ccDir := filepath.Join(dir, ".cc-deck")
	if err := os.MkdirAll(ccDir, 0o755); err != nil {
		t.Fatal(err)
	}
	glossaryPath := filepath.Join(ccDir, "voice-glossary.txt")
	if err := os.WriteFile(glossaryPath, []byte("Helm\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := NewGlossary([]string{"Kubernetes"})

	// First call loads from file
	prompt1 := g.ResolvePrompt(dir)
	if prompt1 != "Kubernetes, Helm" {
		t.Fatalf("first call: prompt = %q", prompt1)
	}

	// Modify file (cache should still return old value)
	if err := os.WriteFile(glossaryPath, []byte("Ingress\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	prompt2 := g.ResolvePrompt(dir)
	if prompt2 != "Kubernetes, Helm" {
		t.Errorf("second call (cached): prompt = %q, want %q", prompt2, "Kubernetes, Helm")
	}
}

func TestResolvePrompt_TokenWarning(t *testing.T) {
	// Create a glossary with enough unique terms to exceed 800 characters
	var terms []string
	for i := 0; i < 100; i++ {
		terms = append(terms, fmt.Sprintf("VeryLongTechnicalTerm%03d", i))
	}
	g := NewGlossary(terms)
	prompt := g.ResolvePrompt("")
	if len(prompt) <= tokenWarningThreshold {
		t.Errorf("prompt length = %d, expected > %d for warning test", len(prompt), tokenWarningThreshold)
	}
	// The warning is logged via log.Printf; we verify the prompt is still returned
	if prompt == "" {
		t.Error("expected non-empty prompt even when exceeding threshold")
	}
}

func TestResolvePrompt_EmptyWorkingDirUsesGlobalOnly(t *testing.T) {
	g := NewGlossary([]string{"Kubernetes", "gRPC"})
	prompt := g.ResolvePrompt("")
	want := "Kubernetes, gRPC"
	if prompt != want {
		t.Errorf("prompt = %q, want %q", prompt, want)
	}
}

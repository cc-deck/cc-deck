package build

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// toolPathRegistry maps tool name keywords (matched case-insensitively using
// word-boundary matching against manifest tool names) to their non-standard
// install paths. The {home} placeholder is replaced with the actual home
// directory at resolution time.
var toolPathRegistry = map[string]string{
	"go":    "/usr/local/go/bin",
	"cargo": "{home}/.cargo/bin",
	"rust":  "{home}/.cargo/bin",
}

// ResolveToolPaths returns the list of non-standard install paths needed by
// the manifest's tools. Each manifest tool name is checked against the
// registry keys using case-insensitive word-boundary matching: the key must
// appear as a standalone word in the tool name (bounded by start/end of
// string, spaces, or non-alphanumeric characters). The {home} placeholder
// in matched paths is replaced with homeDir. Results are deduplicated while
// preserving insertion order.
func ResolveToolPaths(m *Manifest, homeDir string) []string {
	if m == nil || len(m.Tools) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var paths []string

	for _, tool := range m.Tools {
		nameLower := strings.ToLower(tool.Name)
		for key, pathTmpl := range toolPathRegistry {
			if containsWord(nameLower, key) {
				resolved := strings.ReplaceAll(pathTmpl, "{home}", homeDir)
				if !seen[resolved] {
					seen[resolved] = true
					paths = append(paths, resolved)
				}
			}
		}
	}

	// Sort for deterministic output.
	sort.Strings(paths)
	return paths
}

// containsWord checks whether the word appears in s as a standalone token,
// bounded by non-alphanumeric characters or string boundaries.
func containsWord(s, word string) bool {
	idx := 0
	for {
		pos := strings.Index(s[idx:], word)
		if pos < 0 {
			return false
		}
		start := idx + pos
		end := start + len(word)

		leftOK := start == 0 || !isAlphaNum(s[start-1])
		rightOK := end == len(s) || !isAlphaNum(s[end])

		if leftOK && rightOK {
			return true
		}
		idx = start + 1
	}
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// ContainerfileData holds the variables for rendering Containerfile templates.
type ContainerfileData struct {
	Target     string   // "container" or "openshell"
	User       string   // "dev" or "sandbox"
	HomeDir    string   // "/home/dev" or "/sandbox"
	ContextDir string   // "container" or "openshell"
	BaseImage  string   // from manifest targets
	Shell      string   // from settings or "zsh"
	ToolPaths  []string // resolved non-standard tool install paths for PATH restoration
}

// snippetOrder defines the rendering order for Containerfile template snippets.
var snippetOrder = []string{
	"01-header",
	"02-user-setup",
	"03-mandatory-stack",
	"04-openshell-extras",
	"05-shell-finalize",
	"055-openshell-policy",
	"06-footer",
}

// ContainerDataForTarget returns the ContainerfileData for a given target.
func ContainerDataForTarget(m *Manifest, target string) *ContainerfileData {
	shell := "zsh"
	if m.Settings != nil && m.Settings.Shell != "" {
		shell = m.Settings.Shell
	}

	switch target {
	case "container":
		homeDir := "/home/dev"
		return &ContainerfileData{
			Target:     "container",
			User:       "dev",
			HomeDir:    homeDir,
			ContextDir: "container",
			BaseImage:  m.BaseImage(),
			Shell:      shell,
			ToolPaths:  ResolveToolPaths(m, homeDir),
		}
	case "openshell":
		homeDir := "/sandbox"
		return &ContainerfileData{
			Target:     "openshell",
			User:       "sandbox",
			HomeDir:    homeDir,
			ContextDir: "openshell",
			BaseImage:  m.OpenShellBaseImage(),
			Shell:      shell,
			ToolPaths:  ResolveToolPaths(m, homeDir),
		}
	default:
		return nil
	}
}

// RenderContainerfileSnippets renders all Containerfile templates with the
// given data and returns a map of snippet name to rendered content.
func RenderContainerfileSnippets(data *ContainerfileData) (map[string]string, error) {
	snippets := make(map[string]string, len(snippetOrder))

	for _, name := range snippetOrder {
		tmplPath := fmt.Sprintf("templates/containerfile/%s.tmpl", name)
		raw, err := embeddedContainerfileTemplates.ReadFile(tmplPath)
		if err != nil {
			return nil, fmt.Errorf("reading template %s: %w", name, err)
		}

		tmpl, err := template.New(name).Parse(string(raw))
		if err != nil {
			return nil, fmt.Errorf("parsing template %s: %w", name, err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("rendering template %s: %w", name, err)
		}

		rendered := strings.TrimRight(buf.String(), "\n") + "\n"
		snippets[name] = rendered
	}

	return snippets, nil
}

// ExtractContainerfileSnippets renders templates and writes them as numbered
// snippet files to the target directory. Each file is a plain-text
// Containerfile fragment with all template variables resolved.
func ExtractContainerfileSnippets(targetDir string, data *ContainerfileData) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("creating snippets directory: %w", err)
	}

	snippets, err := RenderContainerfileSnippets(data)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(snippets))
	for name := range snippets {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		path := filepath.Join(targetDir, name+".txt")
		if err := os.WriteFile(path, []byte(snippets[name]), 0o644); err != nil {
			return fmt.Errorf("writing snippet %s: %w", name, err)
		}
	}

	return nil
}

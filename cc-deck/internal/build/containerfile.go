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

// ContainerfileData holds the variables for rendering Containerfile templates.
type ContainerfileData struct {
	Target     string // "container" or "openshell"
	User       string // "dev" or "sandbox"
	HomeDir    string // "/home/dev" or "/sandbox"
	ContextDir string // "container" or "openshell"
	BaseImage  string // from manifest targets
	Shell      string // from settings or "zsh"
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
		return &ContainerfileData{
			Target:     "container",
			User:       "dev",
			HomeDir:    "/home/dev",
			ContextDir: "container",
			BaseImage:  m.BaseImage(),
			Shell:      shell,
		}
	case "openshell":
		return &ContainerfileData{
			Target:     "openshell",
			User:       "sandbox",
			HomeDir:    "/sandbox",
			ContextDir: "openshell",
			BaseImage:  m.OpenShellBaseImage(),
			Shell:      shell,
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

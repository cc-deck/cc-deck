package ws

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const templateFileName = "workspace-template.yaml"

var placeholderRegex = regexp.MustCompile(`\{\{(\w+)(?::([^}]*))?\}\}`)

// Placeholder represents a template placeholder with an optional default value.
type Placeholder struct {
	Name    string
	Default string
}

// WorkspaceTemplate is a git-committable template for workspace creation.
// Variant bodies use WorkspaceSpec fields (the same fields as WorkspaceDefinition
// minus name and type, which are derived from the template structure).
type WorkspaceTemplate struct {
	Name     string                     `yaml:"name"`
	Variants map[string]WorkspaceSpec   `yaml:"variants"`
}

// LoadTemplate reads .cc-deck/workspace-template.yaml from the given project root.
// Returns nil, nil if the template file does not exist.
func LoadTemplate(projectRoot string) (*WorkspaceTemplate, error) {
	path := filepath.Join(projectRoot, ".cc-deck", templateFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading template file: %w", err)
	}

	var tmpl WorkspaceTemplate
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("parsing template file: %w", err)
	}

	return &tmpl, nil
}

// ValidateTemplate checks that the template has a name and valid variant keys.
func ValidateTemplate(tmpl *WorkspaceTemplate) error {
	if tmpl.Name == "" {
		return fmt.Errorf("template missing required \"name\" field")
	}
	if len(tmpl.Variants) == 0 {
		return fmt.Errorf("template has no variants defined")
	}

	validTypes := map[string]bool{
		string(WorkspaceTypeSSH):        true,
		string(WorkspaceTypeContainer):  true,
		string(WorkspaceTypeCompose):    true,
		string(WorkspaceTypeK8sDeploy):  true,
	}

	for key := range tmpl.Variants {
		if !validTypes[key] {
			keys := make([]string, 0, len(validTypes))
			for k := range validTypes {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return fmt.Errorf("unknown workspace type %q; valid types: %s", key, strings.Join(keys, ", "))
		}
	}

	return nil
}

// ExtractPlaceholders scans YAML bytes for {{name}} and {{name:default}} patterns.
// Returns deduplicated placeholders in order of first occurrence.
func ExtractPlaceholders(data []byte) []Placeholder {
	matches := placeholderRegex.FindAllSubmatch(data, -1)
	seen := make(map[string]bool)
	var result []Placeholder
	for _, m := range matches {
		name := string(m[1])
		if seen[name] {
			continue
		}
		seen[name] = true
		p := Placeholder{Name: name}
		if len(m) > 2 {
			p.Default = string(m[2])
		}
		result = append(result, p)
	}
	return result
}

// ResolvePlaceholders replaces {{name}} and {{name:default}} patterns with
// the corresponding values from the answers map.
func ResolvePlaceholders(data []byte, answers map[string]string) []byte {
	return placeholderRegex.ReplaceAllFunc(data, func(match []byte) []byte {
		sub := placeholderRegex.FindSubmatch(match)
		name := string(sub[1])
		if val, ok := answers[name]; ok {
			return []byte(sanitizePlaceholderValue(val))
		}
		if len(sub) > 2 && len(sub[2]) > 0 {
			return sub[2]
		}
		return match
	})
}

func sanitizePlaceholderValue(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// PromptForPlaceholders interactively prompts the user for placeholder values.
// If a placeholder has a default, pressing Enter accepts it.
func PromptForPlaceholders(placeholders []Placeholder, reader *bufio.Reader) (map[string]string, error) {
	answers := make(map[string]string)
	for _, p := range placeholders {
		prompt := p.Name
		if p.Default != "" {
			prompt += " [" + p.Default + "]"
		}
		fmt.Fprintf(os.Stderr, "  %s: ", prompt)

		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("reading input for %q: %w", p.Name, err)
		}
		line = strings.TrimSpace(line)

		if line == "" && p.Default != "" {
			line = p.Default
		}
		answers[p.Name] = line
	}
	return answers, nil
}

// VariantToDefinition converts a WorkspaceSpec (template variant) into a
// WorkspaceDefinition with the given name and type.
func VariantToDefinition(name string, wsType WorkspaceType, v *WorkspaceSpec) *WorkspaceDefinition {
	return &WorkspaceDefinition{
		Name:          name,
		Type:          wsType,
		WorkspaceSpec: *v,
	}
}

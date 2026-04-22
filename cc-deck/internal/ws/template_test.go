package ws

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadTemplate_Success(t *testing.T) {
	dir := t.TempDir()
	ccDir := filepath.Join(dir, ".cc-deck")
	require.NoError(t, os.MkdirAll(ccDir, 0o755))

	content := `name: my-project
variants:
  ssh:
    host: "{{ssh_user:roland}}@marovo"
    repos:
      - url: https://github.com/org/repo.git
  container:
    image: quay.io/cc-deck/cc-deck-demo
    storage:
      type: named-volume
`
	require.NoError(t, os.WriteFile(filepath.Join(ccDir, templateFileName), []byte(content), 0o644))

	tmpl, err := LoadTemplate(dir)
	require.NoError(t, err)
	require.NotNil(t, tmpl)
	assert.Equal(t, "my-project", tmpl.Name)
	assert.Len(t, tmpl.Variants, 2)
	assert.Contains(t, tmpl.Variants, "ssh")
	assert.Contains(t, tmpl.Variants, "container")
	assert.Equal(t, "quay.io/cc-deck/cc-deck-demo", tmpl.Variants["container"].Image)
}

func TestLoadTemplate_NotFound(t *testing.T) {
	dir := t.TempDir()

	tmpl, err := LoadTemplate(dir)
	require.NoError(t, err)
	assert.Nil(t, tmpl)
}

func TestLoadTemplate_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	ccDir := filepath.Join(dir, ".cc-deck")
	require.NoError(t, os.MkdirAll(ccDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ccDir, templateFileName), []byte(":\n  invalid: ["), 0o644))

	_, err := LoadTemplate(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing template file")
}

func TestValidateTemplate_MissingName(t *testing.T) {
	tmpl := &WorkspaceTemplate{
		Variants: map[string]TemplateVariant{"ssh": {}},
	}
	err := ValidateTemplate(tmpl)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required \"name\"")
}

func TestValidateTemplate_NoVariants(t *testing.T) {
	tmpl := &WorkspaceTemplate{
		Name:     "test",
		Variants: map[string]TemplateVariant{},
	}
	err := ValidateTemplate(tmpl)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no variants defined")
}

func TestValidateTemplate_UnknownType(t *testing.T) {
	tmpl := &WorkspaceTemplate{
		Name: "test",
		Variants: map[string]TemplateVariant{
			"bogus": {},
		},
	}
	err := ValidateTemplate(tmpl)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown workspace type \"bogus\"")
}

func TestValidateTemplate_ValidTypes(t *testing.T) {
	tmpl := &WorkspaceTemplate{
		Name: "test",
		Variants: map[string]TemplateVariant{
			"ssh":        {},
			"container":  {},
			"compose":    {},
			"k8s-deploy": {},
		},
	}
	require.NoError(t, ValidateTemplate(tmpl))
}

func TestExtractPlaceholders_Basic(t *testing.T) {
	data := []byte(`host: "{{ssh_user}}@marovo"`)
	phs := ExtractPlaceholders(data)
	require.Len(t, phs, 1)
	assert.Equal(t, "ssh_user", phs[0].Name)
	assert.Equal(t, "", phs[0].Default)
}

func TestExtractPlaceholders_WithDefault(t *testing.T) {
	data := []byte(`host: "{{ssh_user:roland}}@marovo"`)
	phs := ExtractPlaceholders(data)
	require.Len(t, phs, 1)
	assert.Equal(t, "ssh_user", phs[0].Name)
	assert.Equal(t, "roland", phs[0].Default)
}

func TestExtractPlaceholders_Multiple(t *testing.T) {
	data := []byte(`host: "{{user:roland}}@{{hostname}}"`)
	phs := ExtractPlaceholders(data)
	require.Len(t, phs, 2)
	assert.Equal(t, "user", phs[0].Name)
	assert.Equal(t, "roland", phs[0].Default)
	assert.Equal(t, "hostname", phs[1].Name)
}

func TestExtractPlaceholders_Deduplicated(t *testing.T) {
	data := []byte(`a: "{{name}}" b: "{{name}}"`)
	phs := ExtractPlaceholders(data)
	require.Len(t, phs, 1)
}

func TestResolvePlaceholders_Basic(t *testing.T) {
	data := []byte(`host: "{{ssh_user}}@marovo"`)
	answers := map[string]string{"ssh_user": "roland"}
	result := ResolvePlaceholders(data, answers)
	assert.Equal(t, `host: "roland@marovo"`, string(result))
}

func TestResolvePlaceholders_WithDefault(t *testing.T) {
	data := []byte(`host: "{{ssh_user:roland}}@marovo"`)
	// No answer provided: should use default.
	result := ResolvePlaceholders(data, map[string]string{})
	assert.Equal(t, `host: "roland@marovo"`, string(result))
}

func TestResolvePlaceholders_AnswerOverridesDefault(t *testing.T) {
	data := []byte(`host: "{{ssh_user:roland}}@marovo"`)
	answers := map[string]string{"ssh_user": "admin"}
	result := ResolvePlaceholders(data, answers)
	assert.Equal(t, `host: "admin@marovo"`, string(result))
}

func TestResolvePlaceholders_UnknownPlaceholder(t *testing.T) {
	data := []byte(`host: "{{unknown}}@marovo"`)
	result := ResolvePlaceholders(data, map[string]string{})
	assert.Equal(t, `host: "{{unknown}}@marovo"`, string(result))
}

func TestPromptForPlaceholders_WithDefaults(t *testing.T) {
	phs := []Placeholder{
		{Name: "user", Default: "roland"},
		{Name: "host"},
	}
	input := "\nmarovo\n"
	reader := bufio.NewReader(strings.NewReader(input))
	answers, err := PromptForPlaceholders(phs, reader)
	require.NoError(t, err)
	assert.Equal(t, "roland", answers["user"])
	assert.Equal(t, "marovo", answers["host"])
}

func TestVariantToDefinition(t *testing.T) {
	v := &TemplateVariant{
		Image: "test:latest",
		Host:  "user@host",
		Repos: []RepoEntry{{URL: "https://github.com/org/repo.git"}},
	}
	def := VariantToDefinition("my-ws", WorkspaceTypeSSH, v)
	assert.Equal(t, "my-ws", def.Name)
	assert.Equal(t, WorkspaceTypeSSH, def.Type)
	assert.Equal(t, "test:latest", def.Image)
	assert.Equal(t, "user@host", def.Host)
	require.Len(t, def.Repos, 1)
}

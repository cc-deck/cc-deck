package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandGroup_Builtin(t *testing.T) {
	r := NewResolver(nil)

	domains, err := r.ExpandGroup("python")
	require.NoError(t, err)
	assert.Contains(t, domains, "pypi.org")
	assert.Contains(t, domains, "files.pythonhosted.org")
}

func TestExpandGroup_UnknownGroup(t *testing.T) {
	r := NewResolver(nil)

	_, err := r.ExpandGroup("nonexistent")
	require.Error(t, err)

	var uge *UnknownGroupError
	require.ErrorAs(t, err, &uge)
	assert.Equal(t, "nonexistent", uge.Name)
	assert.Contains(t, uge.Available, "python")
	assert.Contains(t, uge.Available, "golang")
}

func TestExpandGroup_UserOverride(t *testing.T) {
	userGroups := map[string]userConfigGroup{
		"nodejs": {
			Domains: []string{"registry.internal.corp", "npm.internal.corp"},
		},
	}
	r := NewResolver(userGroups)

	domains, err := r.ExpandGroup("nodejs")
	require.NoError(t, err)
	// User override replaces built-in entirely
	assert.Equal(t, []string{"registry.internal.corp", "npm.internal.corp"}, domains)
	// Built-in domains should NOT be present
	assert.NotContains(t, domains, "registry.npmjs.org")
}

func TestExpandGroup_UserExtends(t *testing.T) {
	userGroups := map[string]userConfigGroup{
		"python": {
			Extends: "builtin",
			Domains: []string{"pypi.internal.corp"},
		},
	}
	r := NewResolver(userGroups)

	domains, err := r.ExpandGroup("python")
	require.NoError(t, err)
	// Should have both built-in and user domains
	assert.Contains(t, domains, "pypi.org")
	assert.Contains(t, domains, "files.pythonhosted.org")
	assert.Contains(t, domains, "pypi.internal.corp")
}

func TestExpandGroup_Includes(t *testing.T) {
	userGroups := map[string]userConfigGroup{
		"company": {
			Includes: []string{"python", "golang"},
			Domains:  []string{"artifacts.internal.corp"},
		},
	}
	r := NewResolver(userGroups)

	domains, err := r.ExpandGroup("company")
	require.NoError(t, err)
	// Should have company's own domains
	assert.Contains(t, domains, "artifacts.internal.corp")
	// Plus included python domains
	assert.Contains(t, domains, "pypi.org")
	// Plus included golang domains
	assert.Contains(t, domains, "proxy.golang.org")
}

func TestExpandGroup_CycleDetection(t *testing.T) {
	userGroups := map[string]userConfigGroup{
		"group-a": {
			Includes: []string{"group-b"},
			Domains:  []string{"a.example.com"},
		},
		"group-b": {
			Includes: []string{"group-a"},
			Domains:  []string{"b.example.com"},
		},
	}
	r := NewResolver(userGroups)

	_, err := r.ExpandGroup("group-a")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular include")
}

func TestExpandAll_MixedGroupsAndDomains(t *testing.T) {
	r := NewResolver(nil)

	domains, err := r.ExpandAll([]string{"python", "custom.example.com"})
	require.NoError(t, err)
	// Should have python group domains
	assert.Contains(t, domains, "pypi.org")
	// Should have the literal domain
	assert.Contains(t, domains, "custom.example.com")
}

func TestExpandAll_UnknownGroupErrors(t *testing.T) {
	r := NewResolver(nil)

	_, err := r.ExpandAll([]string{"python", "nonexistent"})
	require.Error(t, err)

	var uge *UnknownGroupError
	require.ErrorAs(t, err, &uge)
}

func TestWildcardDedup(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no wildcards",
			input:    []string{"pypi.org", "github.com"},
			expected: []string{"pypi.org", "github.com"},
		},
		{
			name:     "wildcard covers subdomain",
			input:    []string{".github.com", "api.github.com", "raw.github.com"},
			expected: []string{".github.com"},
		},
		{
			name:     "wildcard covers base domain",
			input:    []string{".github.com", "github.com"},
			expected: []string{".github.com"},
		},
		{
			name:     "unrelated domains preserved",
			input:    []string{".github.com", "pypi.org", "api.github.com"},
			expected: []string{".github.com", "pypi.org"},
		},
		{
			name:     "dedup exact duplicates",
			input:    []string{"pypi.org", "pypi.org", "github.com"},
			expected: []string{"pypi.org", "github.com"},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WildcardDedup(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsGroupName(t *testing.T) {
	assert.True(t, isGroupName("python"))
	assert.True(t, isGroupName("nodejs"))
	assert.True(t, isGroupName("company"))
	assert.False(t, isGroupName("pypi.org"))
	assert.False(t, isGroupName("api.github.com"))
	assert.False(t, isGroupName(".github.com"))
}

func TestGroupSource(t *testing.T) {
	userGroups := map[string]userConfigGroup{
		"python": {
			Extends: "builtin",
			Domains: []string{"pypi.internal.corp"},
		},
		"company": {
			Domains: []string{"corp.example.com"},
		},
	}
	r := NewResolver(userGroups)

	assert.Equal(t, SourceExtended, r.GroupSource("python"))
	assert.Equal(t, SourceUser, r.GroupSource("company"))
	assert.Equal(t, SourceBuiltin, r.GroupSource("golang"))
	assert.Equal(t, Source(""), r.GroupSource("nonexistent"))
}

func TestAllGroupNames(t *testing.T) {
	userGroups := map[string]userConfigGroup{
		"company": {
			Domains: []string{"corp.example.com"},
		},
	}
	r := NewResolver(userGroups)

	names := r.AllGroupNames()
	// Should include both built-in and user groups
	assert.Contains(t, names, "python")
	assert.Contains(t, names, "company")
	// Should be sorted
	for i := 1; i < len(names); i++ {
		assert.True(t, names[i-1] < names[i], "names should be sorted: %s >= %s", names[i-1], names[i])
	}
}

func TestExpandGroup_RecursiveIncludes(t *testing.T) {
	userGroups := map[string]userConfigGroup{
		"dev-stack": {
			Includes: []string{"frontend"},
		},
		"frontend": {
			Includes: []string{"nodejs"},
			Domains:  []string{"cdn.example.com"},
		},
	}
	r := NewResolver(userGroups)

	domains, err := r.ExpandGroup("dev-stack")
	require.NoError(t, err)
	// Should have nodejs built-in domains (via frontend include)
	assert.Contains(t, domains, "registry.npmjs.org")
	// Should have frontend's own domains
	assert.Contains(t, domains, "cdn.example.com")
}

func TestLoadUserConfigFrom_MissingFile(t *testing.T) {
	groups, err := LoadUserConfigFrom("/nonexistent/path/domains.yaml")
	require.NoError(t, err)
	assert.Nil(t, groups)
}

func TestLoadUserConfigFrom_ValidFile(t *testing.T) {
	groups, err := LoadUserConfigFrom("../../testdata/domains/user-extend.yaml")
	require.NoError(t, err)
	require.NotNil(t, groups)

	python, ok := groups["python"]
	require.True(t, ok)
	assert.Equal(t, "builtin", python.Extends)
	assert.Contains(t, python.Domains, "pypi.internal.corp")

	company, ok := groups["company"]
	require.True(t, ok)
	assert.Contains(t, company.Includes, "python")
	assert.Contains(t, company.Includes, "golang")
}

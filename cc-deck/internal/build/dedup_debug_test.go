package build

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDedupRustDomainGroup(t *testing.T) {
	manifest := &Manifest{
		Tools: []ToolEntry{
			{Name: "Rust stable (edition 2021, MSRV >= 1.80.0)"},
			{Name: "Go >= 1.25.0"},
			{Name: "Node.js >= 22.12.0"},
		},
		Network: &NetworkConfig{
			AllowedDomains: []string{"rust", "golang", "nodejs", "github"},
		},
		Credentials: []CredentialEntry{
			{Type: "claude-vertex"},
		},
	}

	result, err := AssemblePolicyWithOptions(manifest, nil, "", nil, "", AssemblyOptions{})
	if err != nil {
		t.Fatal(err)
	}

	policy := result.Policy
	fmt.Println("Network policy keys:")
	for key, np := range policy.NetworkPolicies {
		fmt.Printf("  %s: name=%q endpoints=%d binaries=%d\n", key, np.Name, len(np.Endpoints), len(np.Binaries))
	}

	_, hasPkgRust := policy.NetworkPolicies["pkg_rust"]
	_, hasRust := policy.NetworkPolicies["rust"]
	assert.True(t, hasPkgRust, "pkg_rust should exist (from component)")
	assert.False(t, hasRust, "rust should NOT exist (should be deduped by coveredHosts)")

	_, hasPkgGo := policy.NetworkPolicies["pkg_go"]
	_, hasGolang := policy.NetworkPolicies["golang"]
	assert.True(t, hasPkgGo, "pkg_go should exist")
	assert.False(t, hasGolang, "golang should NOT exist (should be deduped)")

	_, hasPkgNode := policy.NetworkPolicies["pkg_node"]
	_, hasNodejs := policy.NetworkPolicies["nodejs"]
	assert.True(t, hasPkgNode, "pkg_node should exist")
	assert.True(t, hasNodejs, "nodejs kept: domain group has npmjs.com which is not in pkg_node (npmjs.org)")
}

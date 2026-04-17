package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateRoute(t *testing.T) {
	labels := k8sStandardLabels("test-env")
	route := GenerateRoute("test-env", "cc-deck", "cc-deck-test-env", labels)
	require.NotNil(t, route)

	assert.Equal(t, "route.openshift.io/v1", route.GetAPIVersion())
	assert.Equal(t, "Route", route.GetKind())
	assert.Equal(t, "cc-deck-test-env", route.GetName())
	assert.Equal(t, "cc-deck", route.GetNamespace())

	spec, ok := route.Object["spec"].(map[string]interface{})
	require.True(t, ok)

	to, ok := spec["to"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Service", to["kind"])
	assert.Equal(t, "cc-deck-test-env", to["name"])
}

func TestGenerateEgressFirewall(t *testing.T) {
	labels := k8sStandardLabels("test-env")
	domains := []string{"api.anthropic.com"}

	fw := GenerateEgressFirewall("test-env", "cc-deck", labels, domains)
	require.NotNil(t, fw)

	assert.Equal(t, "k8s.ovn.org/v1", fw.GetAPIVersion())
	assert.Equal(t, "EgressFirewall", fw.GetKind())
	assert.Equal(t, "cc-deck-test-env", fw.GetName())

	spec, ok := fw.Object["spec"].(map[string]interface{})
	require.True(t, ok)

	egress, ok := spec["egress"].([]interface{})
	require.True(t, ok)
	// At minimum: DNS allow + domain allow + default deny.
	assert.GreaterOrEqual(t, len(egress), 2)

	// Last rule should be deny.
	lastRule, ok := egress[len(egress)-1].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Deny", lastRule["type"])
}

func TestGenerateEgressFirewall_NoDomains(t *testing.T) {
	labels := k8sStandardLabels("test-env")

	fw := GenerateEgressFirewall("test-env", "cc-deck", labels, nil)
	require.NotNil(t, fw)

	spec, ok := fw.Object["spec"].(map[string]interface{})
	require.True(t, ok)

	egress, ok := spec["egress"].([]interface{})
	require.True(t, ok)
	// DNS allow + default deny.
	assert.Len(t, egress, 2)
}

package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCredentialSecret(t *testing.T) {
	labels := k8sStandardLabels("test-env")
	creds := map[string]string{
		"ANTHROPIC_API_KEY": "sk-ant-test",
		"GH_TOKEN":          "ghp_test",
	}

	secret := generateCredentialSecret("test-env", "default", labels, creds)
	require.NotNil(t, secret)

	assert.Equal(t, "cc-deck-test-env-creds", secret.Name)
	assert.Equal(t, "default", secret.Namespace)
	assert.Equal(t, []byte("sk-ant-test"), secret.Data["ANTHROPIC_API_KEY"])
	assert.Equal(t, []byte("ghp_test"), secret.Data["GH_TOKEN"])
}

func TestGenerateExternalSecret(t *testing.T) {
	labels := k8sStandardLabels("test-env")

	eso := generateExternalSecret("test-env", "default", labels, "my-vault", "secret/data/cc-deck")
	require.NotNil(t, eso)

	assert.Equal(t, "external-secrets.io/v1", eso.GetAPIVersion())
	assert.Equal(t, "ExternalSecret", eso.GetKind())
	assert.Equal(t, "cc-deck-test-env-eso", eso.GetName())
	assert.Equal(t, "default", eso.GetNamespace())

	spec, ok := eso.Object["spec"].(map[string]interface{})
	require.True(t, ok)

	storeRef, ok := spec["secretStoreRef"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "my-vault", storeRef["name"])
	assert.Equal(t, "SecretStore", storeRef["kind"])

	target, ok := spec["target"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "cc-deck-test-env-eso-secret", target["name"])
}

func TestStringMapToInterface(t *testing.T) {
	m := map[string]string{"key": "value", "key2": "value2"}
	result := stringMapToInterface(m)
	assert.Equal(t, "value", result["key"])
	assert.Equal(t, "value2", result["key2"])
}

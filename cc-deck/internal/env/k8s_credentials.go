package env

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CredentialResult holds the generated credential resources.
type CredentialResult struct {
	Secret         *corev1.Secret
	ExternalSecret *unstructured.Unstructured
}

// HandleCredentials creates the appropriate credential resources based on
// the provided options. Returns nil if no credentials are specified.
func HandleCredentials(ctx context.Context, client *K8sClient, envName, ns string, labels, creds map[string]string, existingSecret, secretStore, secretStoreRef, secretPath string) (*CredentialResult, error) {
	result := &CredentialResult{}

	// Inline credentials: create a K8s Secret.
	if len(creds) > 0 {
		secret := generateCredentialSecret(envName, ns, labels, creds)
		result.Secret = secret
	}

	// ESO integration: generate ExternalSecret CR.
	if secretStore != "" {
		if secretStoreRef == "" {
			return nil, fmt.Errorf("--secret-store-ref is required when --secret-store is specified")
		}
		if secretPath == "" {
			return nil, fmt.Errorf("--secret-path is required when --secret-store is specified")
		}

		// Check for ESO CRDs.
		if !client.HasAPIGroup("external-secrets.io/v1") {
			return nil, fmt.Errorf("External Secrets Operator is not installed on this cluster (external-secrets.io API group not found)")
		}

		eso := generateExternalSecret(envName, ns, labels, secretStoreRef, secretPath)
		result.ExternalSecret = eso
	}

	if result.Secret == nil && result.ExternalSecret == nil && existingSecret == "" {
		return nil, nil
	}

	return result, nil
}

func generateCredentialSecret(envName, ns string, labels map[string]string, creds map[string]string) *corev1.Secret {
	secretData := make(map[string][]byte)
	for key, val := range creds {
		secretData[key] = []byte(val)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sResourceName(envName) + "-creds",
			Namespace: ns,
			Labels:    labels,
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}
}

func generateExternalSecret(envName, ns string, labels map[string]string, secretStoreRef, secretPath string) *unstructured.Unstructured {
	resName := k8sResourceName(envName) + "-eso"
	targetSecretName := resName + "-secret"

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "external-secrets.io/v1",
			"kind":       "ExternalSecret",
			"metadata": map[string]interface{}{
				"name":      resName,
				"namespace": ns,
				"labels":    stringMapToInterface(labels),
			},
			"spec": map[string]interface{}{
				"refreshInterval": "1h",
				"secretStoreRef": map[string]interface{}{
					"name": secretStoreRef,
					"kind": "SecretStore",
				},
				"target": map[string]interface{}{
					"name":           targetSecretName,
					"creationPolicy": "Owner",
				},
				"dataFrom": []interface{}{
					map[string]interface{}{
						"extract": map[string]interface{}{
							"key": secretPath,
						},
					},
				},
			},
		},
	}
}

func stringMapToInterface(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

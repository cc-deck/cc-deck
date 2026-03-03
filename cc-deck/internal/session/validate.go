package session

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/rhuss/cc-mux/cc-deck/internal/config"
)

// ValidateProfileSecrets checks that all K8s Secrets referenced by the profile
// exist in the given namespace. Returns nil if all Secrets exist, or an error
// with instructions for creating missing Secrets.
func ValidateProfileSecrets(ctx context.Context, clientset kubernetes.Interface, namespace string, profile config.Profile) error {
	var missing []string

	switch profile.Backend {
	case config.BackendAnthropic:
		if profile.APIKeySecret != "" {
			if err := checkSecretExists(ctx, clientset, namespace, profile.APIKeySecret); err != nil {
				missing = append(missing, fmt.Sprintf(
					"  Secret %q not found.\n  Create it with: kubectl create secret generic %s --from-literal=api-key=<YOUR_API_KEY> -n %s",
					profile.APIKeySecret, profile.APIKeySecret, namespace,
				))
			}
		}

	case config.BackendVertex:
		if profile.CredentialsSecret != "" {
			if err := checkSecretExists(ctx, clientset, namespace, profile.CredentialsSecret); err != nil {
				missing = append(missing, fmt.Sprintf(
					"  Secret %q not found.\n  Create it with: kubectl create secret generic %s --from-file=credentials.json=<PATH_TO_SA_KEY> -n %s",
					profile.CredentialsSecret, profile.CredentialsSecret, namespace,
				))
			}
		}
	}

	if profile.GitCredentialSecret != "" {
		if err := checkSecretExists(ctx, clientset, namespace, profile.GitCredentialSecret); err != nil {
			missing = append(missing, fmt.Sprintf(
				"  Secret %q not found.\n  Create the git credential Secret in namespace %q.",
				profile.GitCredentialSecret, namespace,
			))
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing Kubernetes Secrets:\n%s", strings.Join(missing, "\n"))
	}

	return nil
}

func checkSecretExists(ctx context.Context, clientset kubernetes.Interface, namespace, name string) error {
	_, err := clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	return err
}

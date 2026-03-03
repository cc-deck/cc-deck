package k8s

import "fmt"

// ClusterUnreachableError indicates the Kubernetes cluster could not be reached.
type ClusterUnreachableError struct {
	Cause error
}

func (e *ClusterUnreachableError) Error() string {
	return fmt.Sprintf("cluster unreachable: %v", e.Cause)
}

func (e *ClusterUnreachableError) Unwrap() error {
	return e.Cause
}

func (e *ClusterUnreachableError) Troubleshoot() string {
	return "Check your kubeconfig with 'kubectl cluster-info'. " +
		"Verify the cluster is running and your network allows connections to the API server."
}

// SecretNotFoundError indicates a required Kubernetes Secret was not found.
type SecretNotFoundError struct {
	SecretName string
	Namespace  string
}

func (e *SecretNotFoundError) Error() string {
	return fmt.Sprintf("Secret %q not found in namespace %q", e.SecretName, e.Namespace)
}

func (e *SecretNotFoundError) Troubleshoot() string {
	return fmt.Sprintf(
		"Create the Secret with: kubectl create secret generic %s --from-literal=<key>=<value> -n %s",
		e.SecretName, e.Namespace,
	)
}

// PVCQuotaError indicates PVC creation failed, likely due to storage quota limits.
type PVCQuotaError struct {
	Namespace string
	Cause     error
}

func (e *PVCQuotaError) Error() string {
	return fmt.Sprintf("PVC creation failed in namespace %q: %v", e.Namespace, e.Cause)
}

func (e *PVCQuotaError) Unwrap() error {
	return e.Cause
}

func (e *PVCQuotaError) Troubleshoot() string {
	return fmt.Sprintf(
		"Check storage quotas with: kubectl describe quota -n %s\n"+
			"Verify a StorageClass is available: kubectl get storageclass",
		e.Namespace,
	)
}

// ImagePullError indicates a container image could not be pulled.
type ImagePullError struct {
	Image string
}

func (e *ImagePullError) Error() string {
	return fmt.Sprintf("failed to pull image %q", e.Image)
}

func (e *ImagePullError) Troubleshoot() string {
	return "Verify the image exists and is accessible from the cluster. " +
		"If the image is in a private registry, ensure an imagePullSecret is configured."
}

// Troubleshooter is implemented by errors that provide troubleshooting guidance.
type Troubleshooter interface {
	error
	Troubleshoot() string
}

// FormatError returns the error message with troubleshooting guidance appended,
// if the error implements the Troubleshooter interface. Otherwise it returns
// the standard error string.
func FormatError(err error) string {
	if ts, ok := err.(Troubleshooter); ok {
		return fmt.Sprintf("%s\n  Hint: %s", err.Error(), ts.Troubleshoot())
	}
	return err.Error()
}

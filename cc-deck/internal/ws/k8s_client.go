package ws

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sClient wraps Kubernetes client-go for workspace lifecycle operations.
type K8sClient struct {
	clientset     kubernetes.Interface
	dynamicClient dynamic.Interface
	discovery     kubernetes.Interface
}

// NewK8sClient creates a K8s client from kubeconfig and context.
// Empty strings use the default kubeconfig and current context.
func NewK8sClient(kubeconfig, context string) (*K8sClient, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		rules.ExplicitPath = kubeconfig
	}

	overrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		overrides.CurrentContext = context
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating clientset: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return &K8sClient{
		clientset:     clientset,
		dynamicClient: dynClient,
		discovery:     clientset,
	}, nil
}

// ValidateNamespace checks that the namespace exists on the cluster.
func (c *K8sClient) ValidateNamespace(ctx context.Context, namespace string) error {
	_, err := c.clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("namespace %q does not exist", namespace)
		}
		return fmt.Errorf("checking namespace: %w", err)
	}
	return nil
}

// CreateStatefulSet creates a StatefulSet in the given namespace.
func (c *K8sClient) CreateStatefulSet(ctx context.Context, ns string, sts *appsv1.StatefulSet) error {
	_, err := c.clientset.AppsV1().StatefulSets(ns).Create(ctx, sts, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating StatefulSet %s/%s: %w", ns, sts.Name, err)
	}
	return nil
}

// DeleteStatefulSet deletes a StatefulSet by name.
func (c *K8sClient) DeleteStatefulSet(ctx context.Context, ns, name string) error {
	err := c.clientset.AppsV1().StatefulSets(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// ScaleStatefulSet updates the replica count of a StatefulSet.
func (c *K8sClient) ScaleStatefulSet(ctx context.Context, ns, name string, replicas int32) error {
	scale, err := c.clientset.AppsV1().StatefulSets(ns).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting scale for StatefulSet %s/%s: %w", ns, name, err)
	}
	scale.Spec.Replicas = replicas
	_, err = c.clientset.AppsV1().StatefulSets(ns).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("scaling StatefulSet %s/%s to %d: %w", ns, name, replicas, err)
	}
	return nil
}

// CreateService creates a Service in the given namespace.
func (c *K8sClient) CreateService(ctx context.Context, ns string, svc *corev1.Service) error {
	_, err := c.clientset.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating Service %s/%s: %w", ns, svc.Name, err)
	}
	return nil
}

// DeleteService deletes a Service by name.
func (c *K8sClient) DeleteService(ctx context.Context, ns, name string) error {
	err := c.clientset.CoreV1().Services(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// CreateConfigMap creates a ConfigMap in the given namespace.
func (c *K8sClient) CreateConfigMap(ctx context.Context, ns string, cm *corev1.ConfigMap) error {
	_, err := c.clientset.CoreV1().ConfigMaps(ns).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating ConfigMap %s/%s: %w", ns, cm.Name, err)
	}
	return nil
}

// DeleteConfigMap deletes a ConfigMap by name.
func (c *K8sClient) DeleteConfigMap(ctx context.Context, ns, name string) error {
	err := c.clientset.CoreV1().ConfigMaps(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// CreateSecret creates a Secret in the given namespace.
func (c *K8sClient) CreateSecret(ctx context.Context, ns string, secret *corev1.Secret) error {
	_, err := c.clientset.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating Secret %s/%s: %w", ns, secret.Name, err)
	}
	return nil
}

// DeleteSecret deletes a Secret by name.
func (c *K8sClient) DeleteSecret(ctx context.Context, ns, name string) error {
	err := c.clientset.CoreV1().Secrets(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// CreateNetworkPolicy creates a NetworkPolicy in the given namespace.
func (c *K8sClient) CreateNetworkPolicy(ctx context.Context, ns string, np *networkingv1.NetworkPolicy) error {
	_, err := c.clientset.NetworkingV1().NetworkPolicies(ns).Create(ctx, np, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating NetworkPolicy %s/%s: %w", ns, np.Name, err)
	}
	return nil
}

// DeleteNetworkPolicy deletes a NetworkPolicy by name.
func (c *K8sClient) DeleteNetworkPolicy(ctx context.Context, ns, name string) error {
	err := c.clientset.NetworkingV1().NetworkPolicies(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// DeletePVC deletes a PersistentVolumeClaim by name.
func (c *K8sClient) DeletePVC(ctx context.Context, ns, name string) error {
	err := c.clientset.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// WaitForPodReady waits until the named Pod reaches the Running phase with
// all containers ready, or until the timeout is reached.
func (c *K8sClient) WaitForPodReady(ctx context.Context, ns, podName string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pod, err := c.clientset.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil // Pod not created yet
			}
			return false, err
		}
		if pod.Status.Phase != corev1.PodRunning {
			return false, nil
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				return false, nil
			}
		}
		return true, nil
	})
}

// ReconcileState checks the actual K8s state and returns the corresponding
// WorkspaceState for the given StatefulSet.
func (c *K8sClient) ReconcileState(ctx context.Context, ns, stsName string) (WorkspaceState, error) {
	sts, err := c.clientset.AppsV1().StatefulSets(ns).Get(ctx, stsName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return WorkspaceStateError, nil
		}
		return WorkspaceStateUnknown, err
	}

	if sts.Spec.Replicas != nil && *sts.Spec.Replicas == 0 {
		return WorkspaceStateStopped, nil
	}

	if sts.Status.ReadyReplicas > 0 {
		return WorkspaceStateRunning, nil
	}

	return WorkspaceStateCreating, nil
}

// HasAPIGroup checks if the cluster supports the given API group version.
func (c *K8sClient) HasAPIGroup(groupVersion string) bool {
	_, err := c.clientset.Discovery().ServerResourcesForGroupVersion(groupVersion)
	return err == nil
}

// CreateExternalSecret creates an ExternalSecret custom resource via the dynamic client.
func (c *K8sClient) CreateExternalSecret(ctx context.Context, ns string, obj *unstructured.Unstructured) error {
	gvr := schema.GroupVersionResource{
		Group:    "external-secrets.io",
		Version:  "v1",
		Resource: "externalsecrets",
	}
	_, err := c.dynamicClient.Resource(gvr).Namespace(ns).Create(ctx, obj, metav1.CreateOptions{})
	return err
}

// DeleteExternalSecret deletes an ExternalSecret custom resource by name.
func (c *K8sClient) DeleteExternalSecret(ctx context.Context, ns, name string) error {
	gvr := schema.GroupVersionResource{
		Group:    "external-secrets.io",
		Version:  "v1",
		Resource: "externalsecrets",
	}
	err := c.dynamicClient.Resource(gvr).Namespace(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// CreateRoute creates an OpenShift Route via the dynamic client.
func (c *K8sClient) CreateRoute(ctx context.Context, ns string, obj *unstructured.Unstructured) error {
	gvr := schema.GroupVersionResource{
		Group:    "route.openshift.io",
		Version:  "v1",
		Resource: "routes",
	}
	_, err := c.dynamicClient.Resource(gvr).Namespace(ns).Create(ctx, obj, metav1.CreateOptions{})
	return err
}

// DeleteRoute deletes an OpenShift Route by name.
func (c *K8sClient) DeleteRoute(ctx context.Context, ns, name string) error {
	gvr := schema.GroupVersionResource{
		Group:    "route.openshift.io",
		Version:  "v1",
		Resource: "routes",
	}
	err := c.dynamicClient.Resource(gvr).Namespace(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// CreateEgressFirewall creates an OVN EgressFirewall via the dynamic client.
func (c *K8sClient) CreateEgressFirewall(ctx context.Context, ns string, obj *unstructured.Unstructured) error {
	gvr := schema.GroupVersionResource{
		Group:    "k8s.ovn.org",
		Version:  "v1",
		Resource: "egressfirewalls",
	}
	_, err := c.dynamicClient.Resource(gvr).Namespace(ns).Create(ctx, obj, metav1.CreateOptions{})
	return err
}

// DeleteEgressFirewall deletes an OVN EgressFirewall by name.
func (c *K8sClient) DeleteEgressFirewall(ctx context.Context, ns, name string) error {
	gvr := schema.GroupVersionResource{
		Group:    "k8s.ovn.org",
		Version:  "v1",
		Resource: "egressfirewalls",
	}
	err := c.dynamicClient.Resource(gvr).Namespace(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

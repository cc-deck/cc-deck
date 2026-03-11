//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/rhuss/cc-mux/cc-deck/internal/k8s"
	"github.com/rhuss/cc-mux/cc-deck/internal/session"
)

const (
	testNamespace = "cc-deck-test"
	testImage     = "localhost/cc-deck-stub"
	testImageTag  = "latest"
	testSecretKey = "test-api-key"
)

func TestMain(m *testing.M) {
	// Load kubeconfig (kind cluster)
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := kubeconfig.ClientConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Skipping integration tests: no kubeconfig available: %v\n", err)
		os.Exit(0)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Skipping integration tests: cannot create K8s client: %v\n", err)
		os.Exit(0)
	}

	// Verify cluster is reachable
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = clientset.CoreV1().Namespaces().Get(ctx, "default", metav1.GetOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Skipping integration tests: cluster unreachable: %v\n", err)
		os.Exit(0)
	}

	// Ensure test namespace exists
	ctx = context.Background()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}
	_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !isAlreadyExists(err) {
		fmt.Fprintf(os.Stderr, "Failed to create test namespace: %v\n", err)
		os.Exit(1)
	}

	// Ensure test Secret exists
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testSecretKey, Namespace: testNamespace},
		StringData: map[string]string{"api-key": "test-key-not-real"},
	}
	_, err = clientset.CoreV1().Secrets(testNamespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && !isAlreadyExists(err) {
		fmt.Fprintf(os.Stderr, "Failed to create test Secret: %v\n", err)
		os.Exit(1)
	}

	env = &testEnv{
		clientset:  clientset,
		restConfig: restConfig,
		namespace:  testNamespace,
		image:      testImage,
		imageTag:   testImageTag,
	}

	os.Exit(m.Run())
}

func isAlreadyExists(err error) bool {
	return k8serrors.IsAlreadyExists(err)
}

// --- Phase 2: Core Lifecycle Tests (US1) ---

func TestDeployCreatesResources(t *testing.T) {
	t.Parallel()
	name := uniqueName(t)
	opts := newDeployOpts(t, name)
	deployAndCleanup(t, opts)

	prefix := k8s.ResourcePrefix(name)
	assertResourceExists(t, "StatefulSet", prefix)
	assertResourceExists(t, "Service", prefix)
	assertResourceExists(t, "ConfigMap", prefix+"-zellij")
	assertResourceExists(t, "NetworkPolicy", prefix+"-egress")
}

func TestDeployPodReachesRunning(t *testing.T) {
	t.Parallel()
	name := uniqueName(t)
	opts := newDeployOpts(t, name)
	deployAndCleanup(t, opts)

	podName := k8s.ResourcePrefix(name) + "-0"
	phase := getPodPhase(t, podName)
	assert.Equal(t, corev1.PodRunning, phase, "Pod should be Running after deploy")
}

func TestDeployDuplicateNameFails(t *testing.T) {
	t.Parallel()
	name := uniqueName(t)
	opts := newDeployOpts(t, name)
	deployAndCleanup(t, opts)

	// Second deploy with the same name should fail
	opts2 := newDeployOpts(t, name)
	_, err := session.Deploy(context.Background(), opts2)
	require.Error(t, err, "duplicate deploy should fail")

	var conflictErr *session.ResourceConflictError
	assert.ErrorAs(t, err, &conflictErr, "should be a ResourceConflictError")
}

func TestListShowsDeployedSession(t *testing.T) {
	t.Parallel()
	name := uniqueName(t)
	opts := newDeployOpts(t, name)
	deployAndCleanup(t, opts)

	// Verify StatefulSet exists (list reads from local config, so we check
	// cluster directly for the integration test)
	ctx := context.Background()
	prefix := k8s.ResourcePrefix(name)
	sts, err := env.clientset.AppsV1().StatefulSets(env.namespace).Get(ctx, prefix, metav1.GetOptions{})
	require.NoError(t, err, "StatefulSet should exist")
	assert.Equal(t, prefix, sts.Name)
	assert.Equal(t, env.namespace, sts.Namespace)

	// Also verify the list command works with a temp config
	tmpConfig := t.TempDir() + "/config.yaml"
	// Write a minimal config with the session
	configContent := fmt.Sprintf(`sessions:
- name: %s
  namespace: %s
  profile: test
  status: running
  pod_name: %s-0
`, name, env.namespace, prefix)
	require.NoError(t, os.WriteFile(tmpConfig, []byte(configContent), 0o644))

	var buf bytes.Buffer
	err = session.List(ctx, env.clientset, &buf, session.ListOptions{
		ConfigPath: tmpConfig,
		Output:     "text",
	})
	require.NoError(t, err, "list should succeed")
	assert.Contains(t, buf.String(), name, "list output should contain session name")
}

func TestDeleteRemovesAllResources(t *testing.T) {
	t.Parallel()
	name := uniqueName(t)
	opts := newDeployOpts(t, name)

	ctx := context.Background()
	_, err := session.Deploy(ctx, opts)
	require.NoError(t, err, "deploy should succeed")

	prefix := k8s.ResourcePrefix(name)

	// Verify resources exist before delete
	assertResourceExists(t, "StatefulSet", prefix)

	// Delete
	caps := &k8s.ClusterCapabilities{}
	tmpConfig := t.TempDir() + "/config.yaml"
	err = session.Delete(ctx, env.clientset, env.restConfig, caps, &bytes.Buffer{}, name, env.namespace, session.DeleteOptions{ConfigPath: tmpConfig})
	require.NoError(t, err, "delete should succeed")

	// Verify resources are gone
	assertResourceNotExists(t, "StatefulSet", prefix)
	assertResourceNotExists(t, "Service", prefix)
	assertResourceNotExists(t, "ConfigMap", prefix+"-zellij")
	assertResourceNotExists(t, "NetworkPolicy", prefix+"-egress")
}

// --- Phase 3: Resource Validation Tests (US3) ---

func TestDeployWithNoNetworkPolicy(t *testing.T) {
	t.Parallel()
	name := uniqueName(t)
	opts := newDeployOpts(t, name)
	opts.NoNetworkPolicy = true
	deployAndCleanup(t, opts)

	prefix := k8s.ResourcePrefix(name)
	assertResourceExists(t, "StatefulSet", prefix)
	assertResourceNotExists(t, "NetworkPolicy", prefix+"-egress")
}

func TestDeployCustomStorageSize(t *testing.T) {
	t.Parallel()
	name := uniqueName(t)
	opts := newDeployOpts(t, name)
	opts.StorageSize = "5Gi"
	deployAndCleanup(t, opts)

	ctx := context.Background()
	prefix := k8s.ResourcePrefix(name)
	pvcName := "data-" + prefix + "-0"

	pvc, err := env.clientset.CoreV1().PersistentVolumeClaims(env.namespace).Get(ctx, pvcName, metav1.GetOptions{})
	require.NoError(t, err, "PVC should exist")

	expected := resource.MustParse("5Gi")
	actual := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	assert.True(t, expected.Equal(actual), "PVC storage should be 5Gi, got %s", actual.String())
}

func TestNetworkPolicyEgressRules(t *testing.T) {
	t.Parallel()
	name := uniqueName(t)
	opts := newDeployOpts(t, name)
	deployAndCleanup(t, opts)

	ctx := context.Background()
	prefix := k8s.ResourcePrefix(name)
	npName := prefix + "-egress"

	np, err := env.clientset.NetworkingV1().NetworkPolicies(env.namespace).Get(ctx, npName, metav1.GetOptions{})
	require.NoError(t, err, "NetworkPolicy should exist")

	// Verify egress rules exist
	require.NotEmpty(t, np.Spec.Egress, "should have egress rules")

	// Check for DNS rule (port 53)
	foundDNS := false
	for _, rule := range np.Spec.Egress {
		for _, port := range rule.Ports {
			if port.Port != nil && port.Port.IntValue() == 53 {
				foundDNS = true
			}
		}
	}
	assert.True(t, foundDNS, "NetworkPolicy should allow DNS (port 53)")
}

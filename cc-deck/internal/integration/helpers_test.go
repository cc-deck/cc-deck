//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/rhuss/cc-mux/cc-deck/internal/config"
	"github.com/rhuss/cc-mux/cc-deck/internal/k8s"
	"github.com/rhuss/cc-mux/cc-deck/internal/session"
)

// testEnv holds shared state for the integration test suite.
type testEnv struct {
	clientset  kubernetes.Interface
	restConfig *rest.Config
	namespace  string
	image      string
	imageTag   string
}

// env is the shared test environment, initialized in TestMain.
var env *testEnv

// testProfile returns a dummy Anthropic profile for testing.
func testProfile() config.Profile {
	return config.Profile{
		Backend:      config.BackendAnthropic,
		APIKeySecret: "test-api-key",
	}
}

// uniqueName generates a unique session name for a test to enable parallel execution.
func uniqueName(t *testing.T) string {
	t.Helper()
	// Use test name (sanitized) + random suffix
	name := strings.ToLower(t.Name())
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "_", "-")
	// Truncate to keep K8s name limits (63 chars total with cc-deck- prefix)
	if len(name) > 30 {
		name = name[:30]
	}
	suffix := fmt.Sprintf("%04x", rand.Intn(0xFFFF))
	return fmt.Sprintf("t-%s-%s", name, suffix)
}

// newDeployOpts creates DeployOptions for a test with a unique session name.
func newDeployOpts(t *testing.T, name string) session.DeployOptions {
	t.Helper()
	caps := &k8s.ClusterCapabilities{} // vanilla K8s, no OpenShift
	return session.DeployOptions{
		Name:        name,
		Namespace:   env.namespace,
		ProfileName: "test",
		Profile:     testProfile(),
		Image:       env.image,
		ImageTag:    env.imageTag,
		Clientset:   env.clientset,
		RestConfig:  env.restConfig,
		Caps:        caps,
	}
}

// deployAndCleanup deploys a session and registers cleanup to delete it.
// Returns the session name.
func deployAndCleanup(t *testing.T, opts session.DeployOptions) {
	t.Helper()
	ctx := context.Background()

	_, err := session.Deploy(ctx, opts)
	require.NoError(t, err, "deploy should succeed")

	t.Cleanup(func() {
		cleanupSession(t, opts.Name)
	})
}

// cleanupSession best-effort deletes all resources for a session.
func cleanupSession(t *testing.T, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	caps := &k8s.ClusterCapabilities{}
	_ = session.Delete(ctx, env.clientset, env.restConfig, caps, io.Discard, name, env.namespace, session.DeleteOptions{})
}

// assertResourceExists verifies a K8s resource exists by kind and name.
func assertResourceExists(t *testing.T, kind, name string) {
	t.Helper()
	ctx := context.Background()
	var err error

	switch kind {
	case "StatefulSet":
		_, err = env.clientset.AppsV1().StatefulSets(env.namespace).Get(ctx, name, metav1.GetOptions{})
	case "Service":
		_, err = env.clientset.CoreV1().Services(env.namespace).Get(ctx, name, metav1.GetOptions{})
	case "ConfigMap":
		_, err = env.clientset.CoreV1().ConfigMaps(env.namespace).Get(ctx, name, metav1.GetOptions{})
	case "NetworkPolicy":
		_, err = env.clientset.NetworkingV1().NetworkPolicies(env.namespace).Get(ctx, name, metav1.GetOptions{})
	case "Pod":
		_, err = env.clientset.CoreV1().Pods(env.namespace).Get(ctx, name, metav1.GetOptions{})
	default:
		t.Fatalf("unsupported resource kind: %s", kind)
	}

	require.NoError(t, err, "%s %q should exist", kind, name)
}

// assertResourceNotExists verifies a K8s resource does not exist.
func assertResourceNotExists(t *testing.T, kind, name string) {
	t.Helper()
	ctx := context.Background()
	var err error

	switch kind {
	case "StatefulSet":
		_, err = env.clientset.AppsV1().StatefulSets(env.namespace).Get(ctx, name, metav1.GetOptions{})
	case "Service":
		_, err = env.clientset.CoreV1().Services(env.namespace).Get(ctx, name, metav1.GetOptions{})
	case "ConfigMap":
		_, err = env.clientset.CoreV1().ConfigMaps(env.namespace).Get(ctx, name, metav1.GetOptions{})
	case "NetworkPolicy":
		_, err = env.clientset.NetworkingV1().NetworkPolicies(env.namespace).Get(ctx, name, metav1.GetOptions{})
	default:
		t.Fatalf("unsupported resource kind: %s", kind)
	}

	require.True(t, errors.IsNotFound(err), "%s %q should not exist, got: %v", kind, name, err)
}

// waitForPodRunning waits for a Pod to reach Running phase.
func waitForPodRunning(t *testing.T, podName string, timeout time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := session.WaitForPodRunning(ctx, env.clientset, env.namespace, podName, timeout, false)
	require.NoError(t, err, "Pod %q should reach Running phase", podName)
}

// getPodPhase returns the current phase of a Pod.
func getPodPhase(t *testing.T, podName string) corev1.PodPhase {
	t.Helper()
	ctx := context.Background()
	pod, err := env.clientset.CoreV1().Pods(env.namespace).Get(ctx, podName, metav1.GetOptions{})
	require.NoError(t, err, "should get Pod %q", podName)
	return pod.Status.Phase
}

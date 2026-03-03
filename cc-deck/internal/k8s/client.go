package k8s

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ClientOptions configures how the K8s client is created.
type ClientOptions struct {
	Kubeconfig string
	Namespace  string
}

// Client wraps a Kubernetes clientset with resolved namespace.
type Client struct {
	Clientset       kubernetes.Interface
	DiscoveryClient discovery.DiscoveryInterface
	RestConfig      *rest.Config
	Namespace       string
}

// NewClient creates a Kubernetes client using the following precedence for kubeconfig:
// 1. Explicit flag value (opts.Kubeconfig)
// 2. KUBECONFIG environment variable
// 3. Default path (~/.kube/config)
//
// Namespace is resolved from (in order):
// 1. Explicit flag value (opts.Namespace)
// 2. Current context namespace from kubeconfig
// 3. "default" as fallback
func NewClient(opts ClientOptions) (*Client, error) {
	kubeconfigPath := resolveKubeconfig(opts.Kubeconfig)

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	configOverrides := &clientcmd.ConfigOverrides{}

	if opts.Namespace != "" {
		configOverrides.Context.Namespace = opts.Namespace
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}

	ns := resolveNamespace(opts.Namespace, kubeConfig)

	return &Client{
		Clientset:       clientset,
		DiscoveryClient: clientset.Discovery(),
		RestConfig:      restConfig,
		Namespace:       ns,
	}, nil
}

// resolveKubeconfig determines the kubeconfig path using flag > env > default.
func resolveKubeconfig(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := os.Getenv("KUBECONFIG"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}

// resolveNamespace determines the namespace using flag > kubeconfig context > "default".
func resolveNamespace(flagValue string, kubeConfig clientcmd.ClientConfig) string {
	if flagValue != "" {
		return flagValue
	}
	ns, _, err := kubeConfig.Namespace()
	if err == nil && ns != "" {
		return ns
	}
	return "default"
}

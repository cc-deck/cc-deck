package session

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/k8s"
)

const (
	// MethodExec connects via kubectl exec into the Pod.
	MethodExec = "exec"
	// MethodWeb connects via port-forward and opens a browser.
	MethodWeb = "web"
	// MethodPortForward connects via port-forward without opening a browser.
	MethodPortForward = "port-forward"

	defaultWebPort   = 8082
	containerWebPort = 8082
)

// ConnectOptions holds parameters for connecting to a session.
type ConnectOptions struct {
	SessionName string
	Method      string
	LocalPort   int
	Clientset   kubernetes.Interface
	RestConfig  *rest.Config
	Namespace   string
	Caps        *k8s.ClusterCapabilities
}

// ConnectResult holds the outcome of a connect operation.
type ConnectResult struct {
	Method  string
	WebURL  string
	WebPort int
	PodName string
}

// Connect establishes a connection to a running session using the specified method.
// If method is empty, auto-detection is used.
func Connect(ctx context.Context, opts ConnectOptions) (*ConnectResult, error) {
	podName := k8s.ResourcePrefix(opts.SessionName) + "-0"

	pod, err := opts.Clientset.CoreV1().Pods(opts.Namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("finding session Pod %q: %w", podName, err)
	}

	if pod.Status.Phase != corev1.PodRunning {
		return nil, fmt.Errorf("session Pod %q is not running (status: %s)", podName, pod.Status.Phase)
	}

	method := opts.Method
	if method == "" {
		method, err = autoDetectMethod(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("auto-detecting connection method: %w", err)
		}
	}

	result := &ConnectResult{
		Method:  method,
		PodName: podName,
	}

	switch method {
	case MethodExec:
		err = execConnect(ctx, opts.RestConfig, opts.Namespace, podName)
	case MethodWeb:
		localPort := opts.LocalPort
		if localPort == 0 {
			localPort = defaultWebPort
		}
		result.WebPort = localPort
		result.WebURL = fmt.Sprintf("http://localhost:%d", localPort)
		err = webConnect(ctx, opts.RestConfig, opts.Namespace, podName, localPort)
	case MethodPortForward:
		localPort := opts.LocalPort
		if localPort == 0 {
			localPort = defaultWebPort
		}
		result.WebPort = localPort
		result.WebURL = fmt.Sprintf("http://localhost:%d", localPort)
		err = portForwardConnect(ctx, opts.RestConfig, opts.Namespace, podName, localPort)
	default:
		return nil, fmt.Errorf("unknown connection method: %q", method)
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

// execConnect opens an interactive terminal to the Pod and attaches to Zellij.
func execConnect(ctx context.Context, restConfig *rest.Config, namespace, podName string) error {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("creating clientset for exec: %w", err)
	}

	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		Param("container", "claude").
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "true").
		Param("command", "zellij").
		Param("command", "attach").
		Param("command", "--create")

	executor, err := remotecommand.NewSPDYExecutor(restConfig, http.MethodPost, req.URL())
	if err != nil {
		return fmt.Errorf("creating SPDY executor: %w", err)
	}

	return executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    true,
	})
}

// webConnect port-forwards to the Pod and opens a browser.
func webConnect(ctx context.Context, restConfig *rest.Config, namespace, podName string, localPort int) error {
	stopCh := make(chan struct{})
	readyCh := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		errCh <- runPortForward(restConfig, namespace, podName, localPort, stopCh, readyCh)
	}()

	select {
	case <-readyCh:
		webURL := fmt.Sprintf("http://localhost:%d", localPort)
		fmt.Fprintf(os.Stderr, "Port-forward ready: %s\n", webURL)
		if err := openBrowser(webURL); err != nil {
			fmt.Fprintf(os.Stderr, "Could not open browser: %v\nOpen manually: %s\n", err, webURL)
		}
	case err := <-errCh:
		return fmt.Errorf("port-forward failed: %w", err)
	case <-ctx.Done():
		close(stopCh)
		return ctx.Err()
	}

	// Wait for interrupt or port-forward error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	select {
	case <-sigCh:
		close(stopCh)
		fmt.Fprintln(os.Stderr, "\nPort-forward stopped.")
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("port-forward error: %w", err)
		}
	case <-ctx.Done():
		close(stopCh)
	}

	return nil
}

// portForwardConnect port-forwards to the Pod without opening a browser.
func portForwardConnect(ctx context.Context, restConfig *rest.Config, namespace, podName string, localPort int) error {
	stopCh := make(chan struct{})
	readyCh := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		errCh <- runPortForward(restConfig, namespace, podName, localPort, stopCh, readyCh)
	}()

	select {
	case <-readyCh:
		webURL := fmt.Sprintf("http://localhost:%d", localPort)
		fmt.Fprintf(os.Stderr, "Port-forward ready: %s\n", webURL)
		fmt.Fprintf(os.Stderr, "Press Ctrl+C to stop.\n")
	case err := <-errCh:
		return fmt.Errorf("port-forward failed: %w", err)
	case <-ctx.Done():
		close(stopCh)
		return ctx.Err()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	select {
	case <-sigCh:
		close(stopCh)
		fmt.Fprintln(os.Stderr, "\nPort-forward stopped.")
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("port-forward error: %w", err)
		}
	case <-ctx.Done():
		close(stopCh)
	}

	return nil
}

// runPortForward sets up a port-forward to the Pod.
func runPortForward(restConfig *rest.Config, namespace, podName string, localPort int, stopCh, readyCh chan struct{}) error {
	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return fmt.Errorf("creating SPDY round tripper: %w", err)
	}

	hostURL := strings.TrimRight(restConfig.Host, "/")
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName)
	serverURL, err := url.Parse(hostURL + path)
	if err != nil {
		return fmt.Errorf("parsing server URL: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, serverURL)

	ports := []string{fmt.Sprintf("%d:%d", localPort, containerWebPort)}
	fw, err := portforward.New(dialer, ports, stopCh, readyCh, os.Stdout, os.Stderr)
	if err != nil {
		return fmt.Errorf("creating port forwarder: %w", err)
	}

	return fw.ForwardPorts()
}

// autoDetectMethod determines the best connection method.
// If a Route or Ingress exists for the session, returns the web URL.
// Otherwise, defaults to exec.
func autoDetectMethod(ctx context.Context, opts ConnectOptions) (string, error) {
	webURL, err := DiscoverWebURL(ctx, opts.RestConfig, opts.Namespace, opts.SessionName, opts.Caps)
	if err == nil && webURL != "" {
		fmt.Fprintf(os.Stderr, "Discovered web URL: %s\n", webURL)
		return MethodWeb, nil
	}
	return MethodExec, nil
}

// DiscoverWebURL looks for an OpenShift Route or Kubernetes Ingress for the session
// and returns the web URL if found.
func DiscoverWebURL(ctx context.Context, restConfig *rest.Config, namespace, sessionName string, caps *k8s.ClusterCapabilities) (string, error) {
	resourceName := k8s.ResourcePrefix(sessionName)

	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return "", fmt.Errorf("creating dynamic client: %w", err)
	}

	// Try OpenShift Route first
	if caps != nil && caps.IsOpenShift {
		webURL, err := discoverRouteURL(ctx, dynClient, namespace, resourceName)
		if err == nil && webURL != "" {
			return webURL, nil
		}
	}

	// Try Kubernetes Ingress
	webURL, err := discoverIngressURL(ctx, dynClient, namespace, resourceName)
	if err == nil && webURL != "" {
		return webURL, nil
	}

	return "", fmt.Errorf("no Route or Ingress found for session %q", sessionName)
}

// discoverRouteURL looks up an OpenShift Route for the session.
func discoverRouteURL(ctx context.Context, dynClient dynamic.Interface, namespace, resourceName string) (string, error) {
	routeGVR := schema.GroupVersionResource{
		Group:    "route.openshift.io",
		Version:  "v1",
		Resource: "routes",
	}

	route, err := dynClient.Resource(routeGVR).Namespace(namespace).Get(ctx, resourceName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return extractRouteURL(route)
}

// extractRouteURL extracts the URL from an OpenShift Route.
func extractRouteURL(route *unstructured.Unstructured) (string, error) {
	spec, ok := route.Object["spec"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("route has no spec")
	}

	host, ok := spec["host"].(string)
	if !ok || host == "" {
		return "", fmt.Errorf("route has no host")
	}

	scheme := "http"
	if tls, ok := spec["tls"].(map[string]interface{}); ok && tls != nil {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s", scheme, host), nil
}

// discoverIngressURL looks up a Kubernetes Ingress for the session.
func discoverIngressURL(ctx context.Context, dynClient dynamic.Interface, namespace, resourceName string) (string, error) {
	ingressGVR := schema.GroupVersionResource{
		Group:    "networking.k8s.io",
		Version:  "v1",
		Resource: "ingresses",
	}

	ingress, err := dynClient.Resource(ingressGVR).Namespace(namespace).Get(ctx, resourceName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return extractIngressURL(ingress)
}

// extractIngressURL extracts the URL from a Kubernetes Ingress.
func extractIngressURL(ingress *unstructured.Unstructured) (string, error) {
	spec, ok := ingress.Object["spec"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("ingress has no spec")
	}

	rules, ok := spec["rules"].([]interface{})
	if !ok || len(rules) == 0 {
		return "", fmt.Errorf("ingress has no rules")
	}

	firstRule, ok := rules[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid ingress rule")
	}

	host, ok := firstRule["host"].(string)
	if !ok || host == "" {
		return "", fmt.Errorf("ingress rule has no host")
	}

	scheme := "http"
	if tls, ok := spec["tls"].([]interface{}); ok && len(tls) > 0 {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s", scheme, host), nil
}

// UpdateSessionConnection updates the session's connection details in the config.
func UpdateSessionConnection(cfg *config.Config, sessionName string, result *ConnectResult) {
	session := cfg.FindSession(sessionName)
	if session == nil {
		return
	}

	session.Connection = config.Connection{
		Method:     result.Method,
		ExecTarget: result.PodName,
		WebURL:     result.WebURL,
		WebPort:    result.WebPort,
	}
}

// openBrowser opens the given URL in the default browser.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	default:
		return fmt.Errorf("unsupported platform %q for browser opening", runtime.GOOS)
	}
}

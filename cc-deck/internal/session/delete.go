package session

import (
	"context"
	"fmt"
	"io"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/rhuss/cc-mux/cc-deck/internal/config"
	"github.com/rhuss/cc-mux/cc-deck/internal/k8s"
)

// DeleteOptions configures the delete operation.
type DeleteOptions struct {
	ConfigPath string
	Verbose    bool
}

// Delete removes all Kubernetes resources for a session and updates the local config.
func Delete(ctx context.Context, clientset kubernetes.Interface, restConfig *rest.Config, caps *k8s.ClusterCapabilities, w io.Writer, sessionName, namespace string, opts DeleteOptions) error {
	prefix := k8s.ResourcePrefix(sessionName)

	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("creating dynamic client: %w", err)
	}

	// Delete StatefulSet
	deleteResource(ctx, w, opts.Verbose, "StatefulSet", prefix, func() error {
		return clientset.AppsV1().StatefulSets(namespace).Delete(ctx, prefix, metav1.DeleteOptions{})
	})

	// Delete Service
	deleteResource(ctx, w, opts.Verbose, "Service", prefix, func() error {
		return clientset.CoreV1().Services(namespace).Delete(ctx, prefix, metav1.DeleteOptions{})
	})

	// Delete PVC (volumeClaimTemplate naming: data-<statefulset-name>-0)
	pvcName := "data-" + prefix + "-0"
	deleteResource(ctx, w, opts.Verbose, "PersistentVolumeClaim", pvcName, func() error {
		return clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
	})

	// Delete NetworkPolicy
	npName := prefix + "-egress"
	deleteResource(ctx, w, opts.Verbose, "NetworkPolicy", npName, func() error {
		return clientset.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, npName, metav1.DeleteOptions{})
	})

	// Delete ConfigMap (Zellij config)
	cmName := prefix + "-zellij"
	deleteResource(ctx, w, opts.Verbose, "ConfigMap", cmName, func() error {
		return clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, cmName, metav1.DeleteOptions{})
	})

	// Delete OpenShift-specific resources if applicable
	if caps != nil && caps.IsOpenShift {
		// Delete EgressFirewall
		egressFWGVR := schema.GroupVersionResource{
			Group:    "k8s.ovn.org",
			Version:  "v1",
			Resource: "egressfirewalls",
		}
		deleteResource(ctx, w, opts.Verbose, "EgressFirewall", prefix, func() error {
			return dynClient.Resource(egressFWGVR).Namespace(namespace).Delete(ctx, prefix, metav1.DeleteOptions{})
		})

		// Delete Route
		routeGVR := schema.GroupVersionResource{
			Group:    "route.openshift.io",
			Version:  "v1",
			Resource: "routes",
		}
		deleteResource(ctx, w, opts.Verbose, "Route", prefix, func() error {
			return dynClient.Resource(routeGVR).Namespace(namespace).Delete(ctx, prefix, metav1.DeleteOptions{})
		})
	} else {
		// Delete Ingress (vanilla K8s)
		deleteResource(ctx, w, opts.Verbose, "Ingress", prefix, func() error {
			return clientset.NetworkingV1().Ingresses(namespace).Delete(ctx, prefix, metav1.DeleteOptions{})
		})
	}

	// Remove session from local config
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if cfg.RemoveSession(sessionName) {
		if err := cfg.Save(opts.ConfigPath); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
	}

	fmt.Fprintf(w, "Session %q deleted successfully.\n", sessionName)
	return nil
}

// deleteResource attempts to delete a resource, ignoring NotFound errors.
func deleteResource(ctx context.Context, w io.Writer, verbose bool, kind, name string, deleteFn func() error) {
	err := deleteFn()
	if err != nil {
		if errors.IsNotFound(err) {
			if verbose {
				fmt.Fprintf(w, "  %s/%s not found (skipped)\n", kind, name)
			}
			return
		}
		fmt.Fprintf(w, "  Warning: failed to delete %s/%s: %v\n", kind, name, err)
		return
	}
	if verbose {
		fmt.Fprintf(w, "  %s/%s deleted\n", kind, name)
	}
}

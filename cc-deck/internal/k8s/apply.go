package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

const fieldManager = "cc-deck"

// Applier performs Server-Side Apply operations on Kubernetes resources.
type Applier struct {
	dynamicClient dynamic.Interface
}

// NewApplier creates a new Applier from a rest.Config.
func NewApplier(cfg *rest.Config) (*Applier, error) {
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}
	return &Applier{dynamicClient: dc}, nil
}

// ApplyStatefulSet applies a StatefulSet using Server-Side Apply.
func (a *Applier) ApplyStatefulSet(ctx context.Context, sts *appsv1.StatefulSet) error {
	gvr := appsv1.SchemeGroupVersion.WithResource("statefulsets")
	return a.applyObject(ctx, sts, gvr.GroupVersion().String(), "StatefulSet", sts.Namespace, gvr)
}

// ApplyService applies a Service using Server-Side Apply.
func (a *Applier) ApplyService(ctx context.Context, svc *corev1.Service) error {
	gvr := corev1.SchemeGroupVersion.WithResource("services")
	return a.applyObject(ctx, svc, "v1", "Service", svc.Namespace, gvr)
}

// applyObject converts a typed K8s object to unstructured and performs
// a Server-Side Apply patch.
func (a *Applier) applyObject(
	ctx context.Context,
	obj runtime.Object,
	apiVersion, kind, namespace string,
	gvr schema.GroupVersionResource,
) error {
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return fmt.Errorf("converting to unstructured: %w", err)
	}

	u := &unstructured.Unstructured{Object: data}
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)

	patchData, err := json.Marshal(u)
	if err != nil {
		return fmt.Errorf("marshaling patch data: %w", err)
	}

	resource := a.dynamicClient.Resource(gvr).Namespace(namespace)
	_, err = resource.Patch(ctx, u.GetName(), types.ApplyPatchType, patchData, metav1.PatchOptions{
		FieldManager: fieldManager,
	})
	if err != nil {
		return fmt.Errorf("applying %s %q: %w", kind, u.GetName(), err)
	}

	return nil
}

// ApplyConfigMap applies a ConfigMap using Server-Side Apply.
func (a *Applier) ApplyConfigMap(ctx context.Context, cm *corev1.ConfigMap) error {
	gvr := corev1.SchemeGroupVersion.WithResource("configmaps")
	return a.applyObject(ctx, cm, "v1", "ConfigMap", cm.Namespace, gvr)
}

// ApplyNetworkPolicy applies a NetworkPolicy using Server-Side Apply.
func (a *Applier) ApplyNetworkPolicy(ctx context.Context, np *networkingv1.NetworkPolicy) error {
	gvr := networkingv1.SchemeGroupVersion.WithResource("networkpolicies")
	return a.applyObject(ctx, np, "networking.k8s.io/v1", "NetworkPolicy", np.Namespace, gvr)
}

// ApplyUnstructured applies an unstructured resource using Server-Side Apply.
// The GVR must be provided since it cannot be inferred from unstructured objects.
func (a *Applier) ApplyUnstructured(ctx context.Context, obj *unstructured.Unstructured, gvr schema.GroupVersionResource) error {
	patchData, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("marshaling patch data: %w", err)
	}

	ns := obj.GetNamespace()
	resource := a.dynamicClient.Resource(gvr).Namespace(ns)
	_, err = resource.Patch(ctx, obj.GetName(), types.ApplyPatchType, patchData, metav1.PatchOptions{
		FieldManager: fieldManager,
	})
	if err != nil {
		return fmt.Errorf("applying %s %q: %w", obj.GetKind(), obj.GetName(), err)
	}

	return nil
}

func init() {
	_ = appsv1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
	_ = networkingv1.AddToScheme(scheme.Scheme)
}

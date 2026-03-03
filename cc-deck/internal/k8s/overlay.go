package k8s

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// ApplyWithOverlay writes generated resources to a temp directory, merges them
// with a user-provided kustomize overlay, builds the result, and applies it.
// Falls back to direct apply with a warning if the kustomize binary is not found.
func ApplyWithOverlay(
	ctx context.Context,
	applier *Applier,
	overlayDir string,
	cm *corev1.ConfigMap,
	sts *appsv1.StatefulSet,
	svc *corev1.Service,
) error {
	// Verify overlay directory exists
	info, err := os.Stat(overlayDir)
	if err != nil {
		return fmt.Errorf("overlay directory %q: %w", overlayDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("overlay path %q is not a directory", overlayDir)
	}

	// Check for kustomize binary
	kustomizePath, err := exec.LookPath("kustomize")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: kustomize not found on PATH, applying resources directly without overlay\n")
		if applyErr := applier.ApplyConfigMap(ctx, cm); applyErr != nil {
			return fmt.Errorf("applying ConfigMap: %w", applyErr)
		}
		if applyErr := applier.ApplyStatefulSet(ctx, sts); applyErr != nil {
			return fmt.Errorf("applying StatefulSet: %w", applyErr)
		}
		if applyErr := applier.ApplyService(ctx, svc); applyErr != nil {
			return fmt.Errorf("applying Service: %w", applyErr)
		}
		return nil
	}

	// Create temp directory for base resources
	tmpDir, err := os.MkdirTemp("", "cc-deck-kustomize-*")
	if err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	baseDir := filepath.Join(tmpDir, "base")
	if err := os.Mkdir(baseDir, 0o755); err != nil {
		return fmt.Errorf("creating base directory: %w", err)
	}

	// Write generated resources as YAML
	resources := []struct {
		obj      runtime.Object
		filename string
	}{
		{cm, "configmap.yaml"},
		{sts, "statefulset.yaml"},
		{svc, "service.yaml"},
	}

	var resourceFiles []string
	for _, r := range resources {
		data, marshalErr := json.Marshal(r.obj)
		if marshalErr != nil {
			return fmt.Errorf("marshaling %s: %w", r.filename, marshalErr)
		}
		path := filepath.Join(baseDir, r.filename)
		if writeErr := os.WriteFile(path, data, 0o644); writeErr != nil {
			return fmt.Errorf("writing %s: %w", r.filename, writeErr)
		}
		resourceFiles = append(resourceFiles, r.filename)
	}

	// Write base kustomization.yaml
	kustomization := "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n"
	for _, f := range resourceFiles {
		kustomization += "  - " + f + "\n"
	}
	kustomizationPath := filepath.Join(baseDir, "kustomization.yaml")
	if err := os.WriteFile(kustomizationPath, []byte(kustomization), 0o644); err != nil {
		return fmt.Errorf("writing base kustomization.yaml: %w", err)
	}

	// Create overlay kustomization that references both the base and user overlay
	overlayBuildDir := filepath.Join(tmpDir, "overlay")
	if err := os.Mkdir(overlayBuildDir, 0o755); err != nil {
		return fmt.Errorf("creating overlay build directory: %w", err)
	}

	absOverlayDir, err := filepath.Abs(overlayDir)
	if err != nil {
		return fmt.Errorf("resolving overlay path: %w", err)
	}

	overlayKustomization := fmt.Sprintf(
		"apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - %s\n  - %s\n",
		baseDir, absOverlayDir,
	)
	overlayKustomizationPath := filepath.Join(overlayBuildDir, "kustomization.yaml")
	if err := os.WriteFile(overlayKustomizationPath, []byte(overlayKustomization), 0o644); err != nil {
		return fmt.Errorf("writing overlay kustomization.yaml: %w", err)
	}

	// Run kustomize build
	cmd := exec.CommandContext(ctx, kustomizePath, "build", overlayBuildDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kustomize build failed: %s\n%w", stderr.String(), err)
	}

	// Parse and apply the kustomize output (multi-document YAML)
	return applyMultiDocYAML(ctx, applier, &stdout)
}

// applyMultiDocYAML parses a multi-document YAML stream and applies each
// resource via Server-Side Apply.
func applyMultiDocYAML(ctx context.Context, applier *Applier, r io.Reader) error {
	reader := utilyaml.NewYAMLReader(bufio.NewReader(r))
	for {
		data, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("reading YAML document: %w", err)
		}

		data = bytes.TrimSpace(data)
		if len(data) == 0 {
			continue
		}

		// Parse into unstructured
		obj := &unstructured.Unstructured{}
		if err := obj.UnmarshalJSON(jsonFromYAML(data)); err != nil {
			// Try direct YAML unmarshal
			parsed, parseErr := parseYAMLToUnstructured(data)
			if parseErr != nil {
				return fmt.Errorf("parsing resource: %w", parseErr)
			}
			obj = parsed
		}

		gvr, err := gvrFromUnstructured(obj)
		if err != nil {
			return fmt.Errorf("determining resource type for %s %q: %w", obj.GetKind(), obj.GetName(), err)
		}

		if err := applier.ApplyUnstructured(ctx, obj, gvr); err != nil {
			return fmt.Errorf("applying %s %q: %w", obj.GetKind(), obj.GetName(), err)
		}
	}
	return nil
}

// parseYAMLToUnstructured converts raw YAML bytes into an Unstructured object.
func parseYAMLToUnstructured(data []byte) (*unstructured.Unstructured, error) {
	jsonData, err := utilyaml.ToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("converting YAML to JSON: %w", err)
	}
	obj := &unstructured.Unstructured{}
	if err := obj.UnmarshalJSON(jsonData); err != nil {
		return nil, fmt.Errorf("unmarshaling JSON: %w", err)
	}
	return obj, nil
}

// jsonFromYAML attempts to convert YAML to JSON. Returns the input unchanged
// if it's already valid JSON.
func jsonFromYAML(data []byte) []byte {
	jsonData, err := utilyaml.ToJSON(data)
	if err != nil {
		return data
	}
	return jsonData
}

// gvrFromUnstructured maps an unstructured object's apiVersion and kind to a
// GroupVersionResource for use with the dynamic client.
func gvrFromUnstructured(obj *unstructured.Unstructured) (schema.GroupVersionResource, error) {
	gvk := obj.GroupVersionKind()
	if gvk.Kind == "" {
		return schema.GroupVersionResource{}, fmt.Errorf("object has no kind")
	}

	// Map common kinds to their resource names
	resourceMap := map[string]string{
		"ConfigMap":      "configmaps",
		"Secret":         "secrets",
		"Service":        "services",
		"StatefulSet":    "statefulsets",
		"Deployment":     "deployments",
		"Pod":            "pods",
		"NetworkPolicy":  "networkpolicies",
		"ServiceAccount": "serviceaccounts",
		"Role":           "roles",
		"RoleBinding":    "rolebindings",
		"PersistentVolumeClaim": "persistentvolumeclaims",
	}

	resource, ok := resourceMap[gvk.Kind]
	if !ok {
		// Fall back to lowercase kind + "s" as a reasonable guess
		resource = strings.ToLower(gvk.Kind) + "s"
	}

	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: resource,
	}, nil
}

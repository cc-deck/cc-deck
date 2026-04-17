package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestGenerateResources_BasicStatefulSet(t *testing.T) {
	opts := K8sResourceOpts{
		Name:        "test-env",
		Namespace:   "default",
		Image:       "quay.io/cc-deck/demo:latest",
		StorageSize: "10Gi",
		Labels:      k8sStandardLabels("test-env"),
	}

	resources, err := GenerateResources(opts)
	require.NoError(t, err)
	require.NotNil(t, resources)

	// StatefulSet
	sts := resources.StatefulSet
	require.NotNil(t, sts)
	assert.Equal(t, "cc-deck-test-env", sts.Name)
	assert.Equal(t, "default", sts.Namespace)
	assert.Equal(t, int32(1), *sts.Spec.Replicas)
	assert.Equal(t, "cc-deck-test-env", sts.Spec.ServiceName)

	// Main container
	require.Len(t, sts.Spec.Template.Spec.Containers, 1)
	container := sts.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "workspace", container.Name)
	assert.Equal(t, "quay.io/cc-deck/demo:latest", container.Image)
	assert.Equal(t, []string{"sleep", "infinity"}, container.Command)

	// Volume mount for workspace
	assert.Contains(t, container.VolumeMounts, corev1.VolumeMount{
		Name:      "data",
		MountPath: "/workspace",
	})

	// VolumeClaimTemplate
	require.Len(t, sts.Spec.VolumeClaimTemplates, 1)
	pvc := sts.Spec.VolumeClaimTemplates[0]
	assert.Equal(t, "data", pvc.Name)
	assert.Contains(t, pvc.Spec.AccessModes, corev1.ReadWriteOnce)
}

func TestGenerateResources_HeadlessService(t *testing.T) {
	opts := K8sResourceOpts{
		Name:        "test-env",
		Namespace:   "cc-deck",
		Image:       "demo:latest",
		StorageSize: "10Gi",
		Labels:      k8sStandardLabels("test-env"),
	}

	resources, err := GenerateResources(opts)
	require.NoError(t, err)

	svc := resources.Service
	require.NotNil(t, svc)
	assert.Equal(t, "cc-deck-test-env", svc.Name)
	assert.Equal(t, "cc-deck", svc.Namespace)
	assert.Equal(t, "None", svc.Spec.ClusterIP)
	assert.Equal(t, "cc-deck", svc.Spec.Selector["app.kubernetes.io/name"])
	assert.Equal(t, "test-env", svc.Spec.Selector["app.kubernetes.io/instance"])
}

func TestGenerateResources_ConfigMap(t *testing.T) {
	opts := K8sResourceOpts{
		Name:        "test-env",
		Namespace:   "default",
		Image:       "demo:latest",
		StorageSize: "10Gi",
		Labels:      k8sStandardLabels("test-env"),
	}

	resources, err := GenerateResources(opts)
	require.NoError(t, err)

	cm := resources.ConfigMap
	require.NotNil(t, cm)
	assert.Equal(t, "cc-deck-test-env", cm.Name)
	assert.Equal(t, "k8s-deploy", cm.Data["env-type"])
	assert.Equal(t, "cc-deck", cm.Data["managed-by"])
}

func TestGenerateResources_Labels(t *testing.T) {
	labels := k8sStandardLabels("my-env")

	assert.Equal(t, "cc-deck", labels["app.kubernetes.io/name"])
	assert.Equal(t, "my-env", labels["app.kubernetes.io/instance"])
	assert.Equal(t, "cc-deck", labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "workspace", labels["app.kubernetes.io/component"])
}

func TestGenerateResources_EmptyImageError(t *testing.T) {
	opts := K8sResourceOpts{
		Name:        "test-env",
		Namespace:   "default",
		Image:       "",
		StorageSize: "10Gi",
		Labels:      k8sStandardLabels("test-env"),
	}

	_, err := GenerateResources(opts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image is required")
}

func TestGenerateResources_InvalidStorageSize(t *testing.T) {
	opts := K8sResourceOpts{
		Name:        "test-env",
		Namespace:   "default",
		Image:       "demo:latest",
		StorageSize: "invalid",
		Labels:      k8sStandardLabels("test-env"),
	}

	_, err := GenerateResources(opts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid storage size")
}

func TestGenerateResources_InlineCredentials(t *testing.T) {
	opts := K8sResourceOpts{
		Name:        "test-env",
		Namespace:   "default",
		Image:       "demo:latest",
		StorageSize: "10Gi",
		Credentials: map[string]string{
			"ANTHROPIC_API_KEY": "sk-ant-test",
		},
		Labels: k8sStandardLabels("test-env"),
	}

	resources, err := GenerateResources(opts)
	require.NoError(t, err)

	// StatefulSet should have credentials volume mount.
	container := resources.StatefulSet.Spec.Template.Spec.Containers[0]
	found := false
	for _, vm := range container.VolumeMounts {
		if vm.MountPath == "/run/secrets/cc-deck" {
			found = true
			assert.True(t, vm.ReadOnly)
			break
		}
	}
	assert.True(t, found, "credential volume mount not found")
}

func TestGenerateResources_ExistingSecret(t *testing.T) {
	opts := K8sResourceOpts{
		Name:           "test-env",
		Namespace:      "default",
		Image:          "demo:latest",
		StorageSize:    "10Gi",
		ExistingSecret: "my-secret",
		Labels:         k8sStandardLabels("test-env"),
	}

	resources, err := GenerateResources(opts)
	require.NoError(t, err)

	// Should reference existing secret in volumes.
	found := false
	for _, vol := range resources.StatefulSet.Spec.Template.Spec.Volumes {
		if vol.Secret != nil && vol.Secret.SecretName == "my-secret" {
			found = true
			break
		}
	}
	assert.True(t, found, "existing secret volume not found")
}

func TestGenerateResources_CustomStorageClass(t *testing.T) {
	opts := K8sResourceOpts{
		Name:         "test-env",
		Namespace:    "default",
		Image:        "demo:latest",
		StorageSize:  "50Gi",
		StorageClass: "fast-ssd",
		Labels:       k8sStandardLabels("test-env"),
	}

	resources, err := GenerateResources(opts)
	require.NoError(t, err)

	pvc := resources.StatefulSet.Spec.VolumeClaimTemplates[0]
	require.NotNil(t, pvc.Spec.StorageClassName)
	assert.Equal(t, "fast-ssd", *pvc.Spec.StorageClassName)
}

func TestGenerateResources_NetworkPolicyDefault(t *testing.T) {
	opts := K8sResourceOpts{
		Name:        "test-env",
		Namespace:   "default",
		Image:       "demo:latest",
		StorageSize: "10Gi",
		Labels:      k8sStandardLabels("test-env"),
	}

	resources, err := GenerateResources(opts)
	require.NoError(t, err)

	// NetworkPolicy should be generated by default (even with no domains).
	np := resources.NetworkPolicy
	require.NotNil(t, np)
	assert.Equal(t, "cc-deck-test-env", np.Name)

	// Should have DNS egress rule.
	require.NotEmpty(t, np.Spec.Egress)
}

func TestGenerateResources_NoNetworkPolicy(t *testing.T) {
	opts := K8sResourceOpts{
		Name:            "test-env",
		Namespace:       "default",
		Image:           "demo:latest",
		StorageSize:     "10Gi",
		NoNetworkPolicy: true,
		Labels:          k8sStandardLabels("test-env"),
	}

	resources, err := GenerateResources(opts)
	require.NoError(t, err)
	assert.Nil(t, resources.NetworkPolicy)
}

func TestGenerateResources_MCPSidecar(t *testing.T) {
	opts := K8sResourceOpts{
		Name:        "test-env",
		Namespace:   "default",
		Image:       "demo:latest",
		StorageSize: "10Gi",
		MCPSidecars: []MCPSidecarOpts{
			{
				Name:    "github",
				Image:   "ghcr.io/github/mcp-server:latest",
				Port:    3000,
				EnvVars: []string{"GH_TOKEN"},
			},
		},
		Labels: k8sStandardLabels("test-env"),
	}

	resources, err := GenerateResources(opts)
	require.NoError(t, err)

	// Should have 2 containers (workspace + mcp-github).
	containers := resources.StatefulSet.Spec.Template.Spec.Containers
	require.Len(t, containers, 2)

	sidecar := containers[1]
	assert.Equal(t, "mcp-github", sidecar.Name)
	assert.Equal(t, "ghcr.io/github/mcp-server:latest", sidecar.Image)
	require.Len(t, sidecar.Ports, 1)
	assert.Equal(t, int32(3000), sidecar.Ports[0].ContainerPort)

	// Env var should reference credential secret.
	require.Len(t, sidecar.Env, 1)
	assert.Equal(t, "GH_TOKEN", sidecar.Env[0].Name)
	require.NotNil(t, sidecar.Env[0].ValueFrom)
	assert.Equal(t, "cc-deck-test-env-creds", sidecar.Env[0].ValueFrom.SecretKeyRef.Name)
}

func TestGenerateResources_BothInlineAndExistingSecret(t *testing.T) {
	opts := K8sResourceOpts{
		Name:           "test-env",
		Namespace:      "default",
		Image:          "demo:latest",
		StorageSize:    "10Gi",
		Credentials:    map[string]string{"KEY": "val"},
		ExistingSecret: "my-secret",
		Labels:         k8sStandardLabels("test-env"),
	}

	resources, err := GenerateResources(opts)
	require.NoError(t, err)

	container := resources.StatefulSet.Spec.Template.Spec.Containers[0]

	// Should have both inline and external credential mounts.
	inlineFound := false
	externalFound := false
	for _, vm := range container.VolumeMounts {
		if vm.MountPath == "/run/secrets/cc-deck/inline" {
			inlineFound = true
			assert.True(t, vm.ReadOnly)
		}
		if vm.MountPath == "/run/secrets/cc-deck/external" {
			externalFound = true
			assert.True(t, vm.ReadOnly)
		}
	}
	assert.True(t, inlineFound, "inline credential mount not found")
	assert.True(t, externalFound, "external credential mount not found")
}

func TestK8sResourceName(t *testing.T) {
	assert.Equal(t, "cc-deck-my-env", k8sResourceName("my-env"))
}

func TestK8sPodName(t *testing.T) {
	assert.Equal(t, "cc-deck-my-env-0", k8sPodName("my-env"))
}

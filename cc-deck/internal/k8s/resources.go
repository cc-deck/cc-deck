package k8s

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/rhuss/cc-mux/cc-deck/internal/config"
)

const (
	appLabel       = "app.kubernetes.io/name"
	instanceLabel  = "app.kubernetes.io/instance"
	managedByLabel = "app.kubernetes.io/managed-by"
	componentLabel = "app.kubernetes.io/component"

	appName        = "cc-deck"
	componentValue = "claude-session"

	containerPort      = 8082
	workspaceMountPath = "/workspace"

	zellijConfigVolume    = "zellij-config"
	zellijConfigMountPath = "/home/claude/.config/zellij"
	zellijConfigKey       = "config.kdl"
)

// SessionParams holds parameters for building K8s resources for a session.
type SessionParams struct {
	Name        string
	Namespace   string
	Profile     config.Profile
	Image       string
	ImageTag    string
	StorageSize string
}

// ResourcePrefix returns the standard resource name prefix for a session.
func ResourcePrefix(sessionName string) string {
	return appName + "-" + sessionName
}

// standardLabels returns the standard set of labels for all cc-deck resources.
func standardLabels(sessionName string) map[string]string {
	return map[string]string{
		appLabel:       appName,
		instanceLabel:  sessionName,
		managedByLabel: appName,
		componentLabel: componentValue,
	}
}

// BuildStatefulSet creates a StatefulSet for a Claude Code session.
func BuildStatefulSet(p SessionParams) *appsv1.StatefulSet {
	name := ResourcePrefix(p.Name)
	labels := standardLabels(p.Name)
	replicas := int32(1)

	container := corev1.Container{
		Name:    "claude",
		Image:   p.Image + ":" + p.ImageTag,
		Command: []string{"zellij"},
		Ports: []corev1.ContainerPort{
			{
				Name:          "web",
				ContainerPort: containerPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "data",
				MountPath: workspaceMountPath,
			},
			{
				Name:      zellijConfigVolume,
				MountPath: zellijConfigMountPath,
				ReadOnly:  true,
			},
		},
	}

	applyCredentialConfig(&container, p.Profile)

	configMapName := ResourcePrefix(p.Name) + "-zellij"
	volumes := []corev1.Volume{
		{
			Name: zellijConfigVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
				},
			},
		},
	}
	if p.Profile.Backend == config.BackendVertex {
		volumes = append(volumes, GCPCredentialVolume(p.Profile.CredentialsSecret))
	}

	ApplyGitCredentialConfig(&container, &volumes, p.Profile)

	storageSize := p.StorageSize
	if storageSize == "" {
		storageSize = "10Gi"
	}

	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: name,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
					Volumes:    volumes,
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(storageSize),
							},
						},
					},
				},
			},
		},
	}
}

// BuildHeadlessService creates a headless Service for the StatefulSet.
func BuildHeadlessService(p SessionParams) *corev1.Service {
	name := ResourcePrefix(p.Name)
	labels := standardLabels(p.Name)

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "web",
					Port:       containerPort,
					TargetPort: intstr.FromInt32(containerPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// applyCredentialConfig configures environment variables and volume mounts
// based on the profile's backend type.
func applyCredentialConfig(c *corev1.Container, p config.Profile) {
	switch p.Backend {
	case config.BackendAnthropic:
		c.Env = append(c.Env, corev1.EnvVar{
			Name: "ANTHROPIC_API_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: p.APIKeySecret,
					},
					Key: "api-key",
				},
			},
		})

	case config.BackendVertex:
		credPath := "/var/run/secrets/gcp/credentials.json"

		c.Env = append(c.Env,
			corev1.EnvVar{
				Name:  "GOOGLE_APPLICATION_CREDENTIALS",
				Value: credPath,
			},
			corev1.EnvVar{
				Name:  "CLOUD_ML_REGION",
				Value: p.Region,
			},
			corev1.EnvVar{
				Name:  "GOOGLE_CLOUD_PROJECT",
				Value: p.Project,
			},
		)

		volumeName := "gcp-credentials"
		c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: "/var/run/secrets/gcp",
			ReadOnly:  true,
		})

		// The volume itself must be added to the PodSpec by the caller
		// or by a wrapper that has access to the PodSpec.
	}
}

// BuildZellijConfigMap creates a ConfigMap containing Zellij configuration
// with web server enabled for remote access.
func BuildZellijConfigMap(p SessionParams) *corev1.ConfigMap {
	name := ResourcePrefix(p.Name)
	labels := standardLabels(p.Name)

	configContent := `// Zellij configuration for cc-deck sessions
web_server true
web_server_ip "0.0.0.0"
web_server_port 8082
`

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-zellij",
			Namespace: p.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			zellijConfigKey: configContent,
		},
	}
}

// GCPCredentialVolume returns the Volume definition for Vertex AI credentials.
// This should be added to the PodSpec when using the Vertex backend.
func GCPCredentialVolume(secretName string) corev1.Volume {
	return corev1.Volume{
		Name: "gcp-credentials",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}
}

// ApplyGitCredentialConfig adds volume mounts and volumes for git credentials
// to the container and PodSpec volumes slice. Two modes are supported:
//   - SSH: Mounts the Secret at /home/claude/.ssh with key "ssh-privatekey"
//   - Token: Mounts the Secret as env vars and configures git credential helper
func ApplyGitCredentialConfig(container *corev1.Container, volumes *[]corev1.Volume, p config.Profile) {
	if p.GitCredentialSecret == "" || p.GitCredentialType == "" {
		return
	}

	switch p.GitCredentialType {
	case config.GitCredentialSSH:
		sshVolumeName := "git-ssh"
		defaultMode := int32(0o400)

		*volumes = append(*volumes, corev1.Volume{
			Name: sshVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  p.GitCredentialSecret,
					DefaultMode: &defaultMode,
				},
			},
		})

		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      sshVolumeName,
			MountPath: "/home/claude/.ssh",
			ReadOnly:  true,
		})

		// Disable strict host key checking for automated use
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "GIT_SSH_COMMAND",
			Value: "ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null",
		})

	case config.GitCredentialToken:
		container.Env = append(container.Env,
			corev1.EnvVar{
				Name: "GIT_TOKEN",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: p.GitCredentialSecret,
						},
						Key: "token",
					},
				},
			},
			corev1.EnvVar{
				Name:  "GIT_ASKPASS",
				Value: "/bin/sh -c 'echo $GIT_TOKEN'",
			},
		)
	}
}

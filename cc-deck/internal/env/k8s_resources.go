package env

import (
	"fmt"
	"net"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// K8sResourceSet holds all generated K8s resources for a single environment.
type K8sResourceSet struct {
	StatefulSet   *appsv1.StatefulSet
	Service       *corev1.Service
	ConfigMap     *corev1.ConfigMap
	Secret        *corev1.Secret
	NetworkPolicy *networkingv1.NetworkPolicy
	Sidecars      []corev1.Container
}

// K8sResourceOpts captures all inputs needed for resource generation.
type K8sResourceOpts struct {
	Name            string
	Namespace       string
	Image           string
	StorageSize     string
	StorageClass    string
	Credentials     map[string]string
	ExistingSecret  string
	Domains         []string
	NoNetworkPolicy bool
	MCPSidecars     []MCPSidecarOpts
	Labels          map[string]string
}

// MCPSidecarOpts describes one MCP sidecar container.
type MCPSidecarOpts struct {
	Name    string
	Image   string
	Port    int
	EnvVars []string
}

// GenerateResources creates all K8s resource objects from environment config.
// Resources are generated in memory but not applied to the cluster.
func GenerateResources(opts K8sResourceOpts) (*K8sResourceSet, error) {
	if opts.Image == "" {
		return nil, fmt.Errorf("image is required")
	}

	if _, err := resource.ParseQuantity(opts.StorageSize); err != nil {
		return nil, fmt.Errorf("invalid storage size %q: %w", opts.StorageSize, err)
	}

	resName := k8sResourceName(opts.Name)
	selectorLabels := map[string]string{
		"app.kubernetes.io/name":     "cc-deck",
		"app.kubernetes.io/instance": opts.Name,
	}

	set := &K8sResourceSet{}

	// Generate headless Service.
	set.Service = generateService(resName, opts.Namespace, opts.Labels, selectorLabels)

	// Generate ConfigMap.
	set.ConfigMap = generateConfigMap(resName, opts.Namespace, opts.Labels)

	// Generate StatefulSet.
	set.StatefulSet = generateStatefulSet(resName, opts, selectorLabels)

	// Generate MCP sidecar containers.
	for _, mcp := range opts.MCPSidecars {
		sidecar := generateMCPSidecar(mcp, resName)
		set.StatefulSet.Spec.Template.Spec.Containers = append(
			set.StatefulSet.Spec.Template.Spec.Containers, sidecar)
	}

	// Generate NetworkPolicy.
	if !opts.NoNetworkPolicy {
		set.NetworkPolicy = generateNetworkPolicy(resName, opts.Namespace, opts.Labels, selectorLabels, opts.Domains)
	}

	return set, nil
}

func generateService(name, ns string, labels, selectorLabels map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  selectorLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "placeholder",
					Port:       80,
					TargetPort: intstr.FromInt32(80),
				},
			},
		},
	}
}

func generateConfigMap(name, ns string, labels map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Data: map[string]string{
			"env-type":    "k8s-deploy",
			"managed-by":  "cc-deck",
		},
	}
}

func generateStatefulSet(name string, opts K8sResourceOpts, selectorLabels map[string]string) *appsv1.StatefulSet {
	replicas := int32(1)

	// Build volume mounts for the main container.
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "data",
			MountPath: k8sWorkspacePath,
		},
	}

	// Determine credential secret name and volume setup.
	var volumes []corev1.Volume
	credSecretName := ""

	if opts.ExistingSecret != "" && len(opts.Credentials) > 0 {
		// Both inline and existing: mount at sub-paths.
		inlineSecretName := name + "-creds"
		credSecretName = inlineSecretName

		volumes = append(volumes,
			corev1.Volume{
				Name: "credentials-inline",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: inlineSecretName,
					},
				},
			},
			corev1.Volume{
				Name: "credentials-external",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: opts.ExistingSecret,
					},
				},
			},
		)
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      "credentials-inline",
				MountPath: k8sCredentialPath + "/inline",
				ReadOnly:  true,
			},
			corev1.VolumeMount{
				Name:      "credentials-external",
				MountPath: k8sCredentialPath + "/external",
				ReadOnly:  true,
			},
		)
	} else if opts.ExistingSecret != "" {
		// Existing secret only.
		volumes = append(volumes, corev1.Volume{
			Name: "credentials",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: opts.ExistingSecret,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "credentials",
			MountPath: k8sCredentialPath,
			ReadOnly:  true,
		})
	} else if len(opts.Credentials) > 0 {
		// Inline credentials only.
		credSecretName = name + "-creds"
		volumes = append(volumes, corev1.Volume{
			Name: "credentials",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: credSecretName,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "credentials",
			MountPath: k8sCredentialPath,
			ReadOnly:  true,
		})
	}

	mainContainer := corev1.Container{
		Name:            "workspace",
		Image:           opts.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"sleep", "infinity"},
		VolumeMounts:    volumeMounts,
	}

	// Parse storage size.
	storageQty := resource.MustParse(opts.StorageSize)

	// Build volumeClaimTemplate.
	pvcSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: storageQty,
			},
		},
	}
	if opts.StorageClass != "" {
		pvcSpec.StorageClassName = &opts.StorageClass
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: opts.Namespace,
			Labels:    opts.Labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: name,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: opts.Labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{mainContainer},
					Volumes:    volumes,
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: pvcSpec,
				},
			},
		},
	}

	return sts
}

func generateNetworkPolicy(name, ns string, labels, selectorLabels map[string]string, domains []string) *networkingv1.NetworkPolicy {
	dnsPort53UDP := intstr.FromInt32(53)
	dnsPort53TCP := intstr.FromInt32(53)
	httpsPort := intstr.FromInt32(443)
	protocolUDP := corev1.ProtocolUDP
	protocolTCP := corev1.ProtocolTCP

	// DNS egress rule (always allowed).
	dnsRule := networkingv1.NetworkPolicyEgressRule{
		Ports: []networkingv1.NetworkPolicyPort{
			{Protocol: &protocolUDP, Port: &dnsPort53UDP},
			{Protocol: &protocolTCP, Port: &dnsPort53TCP},
		},
	}

	egressRules := []networkingv1.NetworkPolicyEgressRule{dnsRule}

	// Resolve domain IPs and create egress rules.
	for _, domain := range domains {
		ips := resolveDomainIPs(domain)
		for _, ip := range ips {
			egressRules = append(egressRules, networkingv1.NetworkPolicyEgressRule{
				To: []networkingv1.NetworkPolicyPeer{
					{
						IPBlock: &networkingv1.IPBlock{
							CIDR: ip + "/32",
						},
					},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{Protocol: &protocolTCP, Port: &httpsPort},
				},
			})
		}
	}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress:      egressRules,
		},
	}
}

func generateMCPSidecar(mcp MCPSidecarOpts, statefulSetName string) corev1.Container {
	container := corev1.Container{
		Name:  "mcp-" + mcp.Name,
		Image: mcp.Image,
	}

	if mcp.Port > 0 {
		container.Ports = []corev1.ContainerPort{
			{
				ContainerPort: int32(mcp.Port),
				Protocol:      corev1.ProtocolTCP,
			},
		}
	}

	// Reference env vars from the credential Secret.
	credSecretName := statefulSetName + "-creds"
	for _, envVar := range mcp.EnvVars {
		container.Env = append(container.Env, corev1.EnvVar{
			Name: envVar,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: credSecretName,
					},
					Key:      envVar,
					Optional: boolPtr(true),
				},
			},
		})
	}

	return container
}

// resolveDomainIPs resolves a domain name to its IP addresses.
// For wildcard domains (starting with "."), the base domain is resolved.
func resolveDomainIPs(domain string) []string {
	lookupDomain := domain
	if len(domain) > 0 && domain[0] == '.' {
		lookupDomain = domain[1:]
	}

	ips, err := net.LookupHost(lookupDomain)
	if err != nil {
		return nil
	}
	return ips
}

func boolPtr(b bool) *bool {
	return &b
}

package ws

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// OpenShiftCapabilities describes which OpenShift-specific APIs are available.
type OpenShiftCapabilities struct {
	HasRoutes         bool
	HasEgressFirewall bool
}

// DetectOpenShift checks for OpenShift-specific API groups via the discovery client.
func DetectOpenShift(client *K8sClient) (*OpenShiftCapabilities, error) {
	caps := &OpenShiftCapabilities{
		HasRoutes:         client.HasAPIGroup("route.openshift.io/v1"),
		HasEgressFirewall: client.HasAPIGroup("k8s.ovn.org/v1"),
	}
	return caps, nil
}

// GenerateRoute creates an OpenShift Route targeting the headless Service.
func GenerateRoute(wsName, ns, serviceName string, labels map[string]string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "route.openshift.io/v1",
			"kind":       "Route",
			"metadata": map[string]interface{}{
				"name":      serviceName,
				"namespace": ns,
				"labels":    stringMapToInterface(labels),
			},
			"spec": map[string]interface{}{
				"to": map[string]interface{}{
					"kind": "Service",
					"name": serviceName,
				},
				"port": map[string]interface{}{
					"targetPort": "placeholder",
				},
			},
		},
	}
}

// GenerateEgressFirewall creates an OVN EgressFirewall with rules consistent
// with the NetworkPolicy egress configuration.
func GenerateEgressFirewall(wsName, ns string, labels map[string]string, domains []string) *unstructured.Unstructured {
	resName := k8sResourceName(wsName)

	var egressRules []interface{}

	// Allow DNS.
	egressRules = append(egressRules, map[string]interface{}{
		"type": "Allow",
		"to": map[string]interface{}{
			"dnsName": ".",
		},
		"ports": []interface{}{
			map[string]interface{}{
				"protocol": "UDP",
				"port":     53,
			},
			map[string]interface{}{
				"protocol": "TCP",
				"port":     53,
			},
		},
	})

	// Allow resolved domain IPs.
	for _, domain := range domains {
		ips := resolveDomainIPs(domain)
		for _, ip := range ips {
			egressRules = append(egressRules, map[string]interface{}{
				"type": "Allow",
				"to": map[string]interface{}{
					"cidrSelector": ip + "/32",
				},
				"ports": []interface{}{
					map[string]interface{}{
						"protocol": "TCP",
						"port":     443,
					},
				},
			})
		}
	}

	// Default deny.
	egressRules = append(egressRules, map[string]interface{}{
		"type": "Deny",
		"to": map[string]interface{}{
			"cidrSelector": "0.0.0.0/0",
		},
	})

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "k8s.ovn.org/v1",
			"kind":       "EgressFirewall",
			"metadata": map[string]interface{}{
				"name":      resName,
				"namespace": ns,
				"labels":    stringMapToInterface(labels),
			},
			"spec": map[string]interface{}{
				"egress": egressRules,
			},
		},
	}
}

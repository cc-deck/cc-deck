package k8s

import (
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/cc-deck/cc-deck/internal/config"
	"github.com/cc-deck/cc-deck/internal/network"
)

// NetworkPolicyParams holds parameters for building network policies.
type NetworkPolicyParams struct {
	SessionName   string
	Namespace     string
	Backend       config.BackendType
	AllowedEgress []string
}

// BuildNetworkPolicy creates a default-deny egress NetworkPolicy with
// exceptions for DNS and backend-specific CIDR allowlists.
func BuildNetworkPolicy(p NetworkPolicyParams) (*networkingv1.NetworkPolicy, error) {
	name := ResourcePrefix(p.SessionName)
	labels := standardLabels(p.SessionName)

	dnsPort := intstr.FromInt32(53)

	egressRules := []networkingv1.NetworkPolicyEgressRule{
		// Rule 1: Allow DNS (UDP+TCP 53) to kube-dns in kube-system
		{
			Ports: []networkingv1.NetworkPolicyPort{
				{
					Protocol: protocolPtr(corev1.ProtocolUDP),
					Port:     &dnsPort,
				},
				{
					Protocol: protocolPtr(corev1.ProtocolTCP),
					Port:     &dnsPort,
				},
			},
			To: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": "kube-system",
						},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"k8s-app": "kube-dns",
						},
					},
				},
			},
		},
	}

	// Rule 2: Backend-specific egress rules (CIDR-based for standard NetworkPolicy)
	backendRules, err := backendEgressRules(p.Backend)
	if err != nil {
		return nil, fmt.Errorf("building backend egress rules: %w", err)
	}
	egressRules = append(egressRules, backendRules...)

	// Rule 3: User-specified allowed egress hosts
	userRules, err := resolveHostRules(p.AllowedEgress)
	if err != nil {
		return nil, fmt.Errorf("building user egress rules: %w", err)
	}
	egressRules = append(egressRules, userRules...)

	return &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "NetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.Namespace,
			Labels:    labels,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
			},
			Egress: egressRules,
		},
	}, nil
}

// backendEgressRules returns CIDR-based egress rules for the given backend.
// Standard K8s NetworkPolicy only supports IP/CIDR, not FQDN.
// Uses the domain group system to resolve backend domains.
func backendEgressRules(backend config.BackendType) ([]networkingv1.NetworkPolicyEgressRule, error) {
	httpsPort := intstr.FromInt32(443)

	hosts := backendDNSNames(backend)
	if len(hosts) == 0 {
		return nil, fmt.Errorf("unknown backend: %q", backend)
	}

	rules, err := resolveHostRules(hosts)
	if err != nil {
		// If DNS resolution fails, fall back to allowing all HTTPS traffic
		// so the session is still functional.
		return []networkingv1.NetworkPolicyEgressRule{
			{
				Ports: []networkingv1.NetworkPolicyPort{
					{
						Protocol: protocolPtr(corev1.ProtocolTCP),
						Port:     &httpsPort,
					},
				},
			},
		}, nil
	}

	return rules, nil
}

// resolveHostRules resolves hostnames to IPs and creates CIDR-based egress rules.
// Each host gets its own rule with port 443 TCP.
func resolveHostRules(hosts []string) ([]networkingv1.NetworkPolicyEgressRule, error) {
	if len(hosts) == 0 {
		return nil, nil
	}

	httpsPort := intstr.FromInt32(443)
	var rules []networkingv1.NetworkPolicyEgressRule

	for _, host := range hosts {
		cidrs, err := hostToCIDRs(host)
		if err != nil {
			return nil, fmt.Errorf("resolving %q: %w", host, err)
		}

		var peers []networkingv1.NetworkPolicyPeer
		for _, cidr := range cidrs {
			peers = append(peers, networkingv1.NetworkPolicyPeer{
				IPBlock: &networkingv1.IPBlock{
					CIDR: cidr,
				},
			})
		}

		rules = append(rules, networkingv1.NetworkPolicyEgressRule{
			Ports: []networkingv1.NetworkPolicyPort{
				{
					Protocol: protocolPtr(corev1.ProtocolTCP),
					Port:     &httpsPort,
				},
			},
			To: peers,
		})
	}

	return rules, nil
}

// hostToCIDRs resolves a hostname to /32 CIDRs. If the input is already a CIDR,
// it is returned as-is.
func hostToCIDRs(host string) ([]string, error) {
	// Check if already a CIDR
	if _, _, err := net.ParseCIDR(host); err == nil {
		return []string{host}, nil
	}

	// Check if already an IP
	if ip := net.ParseIP(host); ip != nil {
		return []string{ip.String() + "/32"}, nil
	}

	// Resolve hostname
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("DNS lookup failed for %q: %w", host, err)
	}

	var cidrs []string
	for _, ip := range ips {
		if ip4 := ip.To4(); ip4 != nil {
			cidrs = append(cidrs, ip4.String()+"/32")
		}
	}

	if len(cidrs) == 0 {
		return nil, fmt.Errorf("no IPv4 addresses found for %q", host)
	}

	return cidrs, nil
}

// EgressFirewallParams holds parameters for building an OpenShift EgressFirewall.
type EgressFirewallParams struct {
	SessionName   string
	Namespace     string
	Backend       config.BackendType
	AllowedEgress []string
}

// BuildEgressFirewall creates an OpenShift EgressFirewall (k8s.ovn.org/v1)
// with FQDN-based allowlists for the AI backend and user-specified hosts.
// Returns an unstructured object since EgressFirewall is not in client-go's typed API.
func BuildEgressFirewall(p EgressFirewallParams) *unstructured.Unstructured {
	name := ResourcePrefix(p.SessionName)
	labels := standardLabels(p.SessionName)

	// Build the egress rules list
	var egressRules []interface{}

	// Backend-specific FQDN rules
	for _, dns := range backendDNSNames(p.Backend) {
		egressRules = append(egressRules, map[string]interface{}{
			"type": "Allow",
			"to": map[string]interface{}{
				"dnsName": dns,
			},
			"ports": []interface{}{
				map[string]interface{}{
					"port":     int64(443),
					"protocol": "TCP",
				},
			},
		})
	}

	// User-specified egress hosts (as FQDN rules)
	for _, host := range p.AllowedEgress {
		// If it looks like a CIDR or IP, use cidrSelector instead of dnsName
		if _, _, err := net.ParseCIDR(host); err == nil {
			egressRules = append(egressRules, map[string]interface{}{
				"type": "Allow",
				"to": map[string]interface{}{
					"cidrSelector": host,
				},
				"ports": []interface{}{
					map[string]interface{}{
						"port":     int64(443),
						"protocol": "TCP",
					},
				},
			})
		} else if net.ParseIP(host) != nil {
			egressRules = append(egressRules, map[string]interface{}{
				"type": "Allow",
				"to": map[string]interface{}{
					"cidrSelector": host + "/32",
				},
				"ports": []interface{}{
					map[string]interface{}{
						"port":     int64(443),
						"protocol": "TCP",
					},
				},
			})
		} else {
			egressRules = append(egressRules, map[string]interface{}{
				"type": "Allow",
				"to": map[string]interface{}{
					"dnsName": host,
				},
				"ports": []interface{}{
					map[string]interface{}{
						"port":     int64(443),
						"protocol": "TCP",
					},
				},
			})
		}
	}

	// Default-deny rule (must be last)
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
				"name":      name,
				"namespace": p.Namespace,
				"labels":    toStringInterfaceMap(labels),
			},
			"spec": map[string]interface{}{
				"egress": egressRules,
			},
		},
	}
}

// backendDNSNames returns the FQDN hostnames for a given backend type.
// Uses the domain group system (built-in "anthropic" and "vertexai" groups)
// to resolve backend domains.
func backendDNSNames(backend config.BackendType) []string {
	var groupName string
	switch backend {
	case config.BackendAnthropic:
		groupName = "anthropic"
	case config.BackendVertex:
		groupName = "vertexai"
	default:
		return nil
	}

	resolver := network.NewResolver(nil)
	domains, err := resolver.ExpandGroup(groupName)
	if err != nil {
		return nil
	}

	// Filter out wildcard patterns (not valid FQDNs for DNS resolution)
	var fqdns []string
	for _, d := range domains {
		if len(d) > 0 && d[0] != '.' {
			fqdns = append(fqdns, d)
		}
	}
	return fqdns
}

// toStringInterfaceMap converts map[string]string to map[string]interface{}.
func toStringInterfaceMap(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func protocolPtr(p corev1.Protocol) *corev1.Protocol {
	return &p
}

package k8s

import (
	"net"
	"testing"

	"github.com/rhuss/cc-mux/cc-deck/internal/config"
)

func TestBuildNetworkPolicy_HasDefaultDenyEgress(t *testing.T) {
	p := NetworkPolicyParams{
		SessionName: "test",
		Namespace:   "default",
		Backend:     config.BackendAnthropic,
	}
	np, err := BuildNetworkPolicy(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if np.Name != "cc-deck-test" {
		t.Errorf("expected name cc-deck-test, got %s", np.Name)
	}
	if np.Namespace != "default" {
		t.Errorf("expected namespace default, got %s", np.Namespace)
	}

	if len(np.Spec.PolicyTypes) != 1 || np.Spec.PolicyTypes[0] != "Egress" {
		t.Errorf("expected Egress policy type, got %v", np.Spec.PolicyTypes)
	}

	// Should have at least DNS rule + backend rule
	if len(np.Spec.Egress) < 2 {
		t.Errorf("expected at least 2 egress rules, got %d", len(np.Spec.Egress))
	}
}

func TestBuildNetworkPolicy_DNSRule(t *testing.T) {
	p := NetworkPolicyParams{
		SessionName: "test",
		Namespace:   "default",
		Backend:     config.BackendAnthropic,
	}
	np, err := BuildNetworkPolicy(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dnsRule := np.Spec.Egress[0]

	// DNS should have 2 ports (UDP 53 + TCP 53)
	if len(dnsRule.Ports) != 2 {
		t.Fatalf("expected 2 DNS ports, got %d", len(dnsRule.Ports))
	}
	if dnsRule.Ports[0].Port.IntValue() != 53 {
		t.Errorf("expected port 53, got %d", dnsRule.Ports[0].Port.IntValue())
	}

	// Should target kube-system namespace
	if len(dnsRule.To) != 1 {
		t.Fatalf("expected 1 DNS peer, got %d", len(dnsRule.To))
	}
	nsLabels := dnsRule.To[0].NamespaceSelector.MatchLabels
	if nsLabels["kubernetes.io/metadata.name"] != "kube-system" {
		t.Errorf("expected kube-system namespace selector, got %v", nsLabels)
	}
	podLabels := dnsRule.To[0].PodSelector.MatchLabels
	if podLabels["k8s-app"] != "kube-dns" {
		t.Errorf("expected kube-dns pod selector, got %v", podLabels)
	}
}

func TestBuildNetworkPolicy_Labels(t *testing.T) {
	p := NetworkPolicyParams{
		SessionName: "mysession",
		Namespace:   "ns1",
		Backend:     config.BackendAnthropic,
	}
	np, err := BuildNetworkPolicy(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedLabels := standardLabels("mysession")
	for k, v := range expectedLabels {
		if np.Labels[k] != v {
			t.Errorf("expected label %s=%s, got %s", k, v, np.Labels[k])
		}
	}

	// Pod selector should match the same labels
	for k, v := range expectedLabels {
		if np.Spec.PodSelector.MatchLabels[k] != v {
			t.Errorf("expected pod selector label %s=%s, got %s", k, v, np.Spec.PodSelector.MatchLabels[k])
		}
	}
}

func TestBuildNetworkPolicy_WithUserAllowedEgress(t *testing.T) {
	p := NetworkPolicyParams{
		SessionName:   "test",
		Namespace:     "default",
		Backend:       config.BackendAnthropic,
		AllowedEgress: []string{"10.0.0.0/8"},
	}
	np, err := BuildNetworkPolicy(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have DNS + backend + user rule
	if len(np.Spec.Egress) < 3 {
		t.Errorf("expected at least 3 egress rules with user egress, got %d", len(np.Spec.Egress))
	}
}

func TestHostToCIDRs_CIDR(t *testing.T) {
	cidrs, err := hostToCIDRs("10.0.0.0/8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cidrs) != 1 || cidrs[0] != "10.0.0.0/8" {
		t.Errorf("expected [10.0.0.0/8], got %v", cidrs)
	}
}

func TestHostToCIDRs_IP(t *testing.T) {
	cidrs, err := hostToCIDRs("192.168.1.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cidrs) != 1 || cidrs[0] != "192.168.1.1/32" {
		t.Errorf("expected [192.168.1.1/32], got %v", cidrs)
	}
}

func TestHostToCIDRs_Hostname(t *testing.T) {
	cidrs, err := hostToCIDRs("localhost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cidrs) == 0 {
		t.Error("expected at least one CIDR for localhost")
	}
	for _, cidr := range cidrs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			t.Errorf("invalid CIDR %q: %v", cidr, err)
		}
	}
}

func TestBuildEgressFirewall_Anthropic(t *testing.T) {
	p := EgressFirewallParams{
		SessionName: "test",
		Namespace:   "default",
		Backend:     config.BackendAnthropic,
	}
	ef := BuildEgressFirewall(p)

	if ef.GetAPIVersion() != "k8s.ovn.org/v1" {
		t.Errorf("expected apiVersion k8s.ovn.org/v1, got %s", ef.GetAPIVersion())
	}
	if ef.GetKind() != "EgressFirewall" {
		t.Errorf("expected kind EgressFirewall, got %s", ef.GetKind())
	}
	if ef.GetName() != "cc-deck-test" {
		t.Errorf("expected name cc-deck-test, got %s", ef.GetName())
	}

	spec, ok := ef.Object["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("spec not found or wrong type")
	}
	egress, ok := spec["egress"].([]interface{})
	if !ok {
		t.Fatal("egress rules not found or wrong type")
	}

	// Should have at least one allow rule + the deny-all rule
	if len(egress) < 2 {
		t.Fatalf("expected at least 2 egress rules, got %d", len(egress))
	}

	// First rule should be Allow for api.anthropic.com
	firstRule := egress[0].(map[string]interface{})
	if firstRule["type"] != "Allow" {
		t.Errorf("expected first rule type Allow, got %v", firstRule["type"])
	}
	to := firstRule["to"].(map[string]interface{})
	if to["dnsName"] != "api.anthropic.com" {
		t.Errorf("expected dnsName api.anthropic.com, got %v", to["dnsName"])
	}

	// Last rule should be Deny 0.0.0.0/0
	lastRule := egress[len(egress)-1].(map[string]interface{})
	if lastRule["type"] != "Deny" {
		t.Errorf("expected last rule type Deny, got %v", lastRule["type"])
	}
}

func TestBuildEgressFirewall_Vertex(t *testing.T) {
	p := EgressFirewallParams{
		SessionName: "test",
		Namespace:   "default",
		Backend:     config.BackendVertex,
	}
	ef := BuildEgressFirewall(p)

	spec := ef.Object["spec"].(map[string]interface{})
	egress := spec["egress"].([]interface{})

	// Vertex should have multiple googleapis entries + deny-all
	// At minimum: oauth2.googleapis.com, aiplatform.googleapis.com, + deny
	if len(egress) < 3 {
		t.Fatalf("expected at least 3 egress rules for Vertex, got %d", len(egress))
	}
}

func TestBuildEgressFirewall_WithUserEgress(t *testing.T) {
	p := EgressFirewallParams{
		SessionName:   "test",
		Namespace:     "default",
		Backend:       config.BackendAnthropic,
		AllowedEgress: []string{"custom.example.com", "10.0.0.0/8"},
	}
	ef := BuildEgressFirewall(p)

	spec := ef.Object["spec"].(map[string]interface{})
	egress := spec["egress"].([]interface{})

	// Should have: backend + 2 user hosts + deny-all
	if len(egress) < 4 {
		t.Fatalf("expected at least 4 egress rules, got %d", len(egress))
	}

	// Check the CIDR-based user rule
	cidrRule := egress[2].(map[string]interface{})
	to := cidrRule["to"].(map[string]interface{})
	if to["cidrSelector"] != "10.0.0.0/8" {
		t.Errorf("expected cidrSelector 10.0.0.0/8, got %v", to["cidrSelector"])
	}
}

func TestBuildEgressFirewall_UserIP(t *testing.T) {
	p := EgressFirewallParams{
		SessionName:   "test",
		Namespace:     "default",
		Backend:       config.BackendAnthropic,
		AllowedEgress: []string{"192.168.1.1"},
	}
	ef := BuildEgressFirewall(p)

	spec := ef.Object["spec"].(map[string]interface{})
	egress := spec["egress"].([]interface{})

	// IP should be converted to /32 CIDR
	ipRule := egress[1].(map[string]interface{})
	to := ipRule["to"].(map[string]interface{})
	if to["cidrSelector"] != "192.168.1.1/32" {
		t.Errorf("expected cidrSelector 192.168.1.1/32, got %v", to["cidrSelector"])
	}
}

func TestBackendDNSNames(t *testing.T) {
	anthropic := backendDNSNames(config.BackendAnthropic)
	if len(anthropic) != 1 || anthropic[0] != "api.anthropic.com" {
		t.Errorf("unexpected Anthropic DNS names: %v", anthropic)
	}

	vertex := backendDNSNames(config.BackendVertex)
	if len(vertex) < 2 {
		t.Errorf("expected multiple Vertex DNS names, got %v", vertex)
	}

	unknown := backendDNSNames("unknown")
	if unknown != nil {
		t.Errorf("expected nil for unknown backend, got %v", unknown)
	}
}

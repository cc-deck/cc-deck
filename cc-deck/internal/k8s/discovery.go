package k8s

import (
	"k8s.io/client-go/discovery"
)

// ClusterCapabilities describes detected cluster features.
type ClusterCapabilities struct {
	IsOpenShift     bool
	HasOVNKubernetes bool
}

// DetectCapabilities probes the cluster's discovery API to determine
// whether it is OpenShift and whether OVN-Kubernetes is available.
func DetectCapabilities(dc discovery.DiscoveryInterface) (*ClusterCapabilities, error) {
	caps := &ClusterCapabilities{}

	groups, err := dc.ServerGroups()
	if err != nil {
		return nil, err
	}

	for _, g := range groups.Groups {
		switch g.Name {
		case "route.openshift.io":
			caps.IsOpenShift = true
		case "k8s.ovn.org":
			caps.HasOVNKubernetes = true
		}
	}

	return caps, nil
}

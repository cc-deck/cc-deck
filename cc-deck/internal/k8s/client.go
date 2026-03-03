package k8s

import (
	// Ensure client-go is pulled in as a dependency
	_ "k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/tools/clientcmd"
)

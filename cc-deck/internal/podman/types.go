package podman

// RunOpts configures a new container.
type RunOpts struct {
	Name     string
	Image    string
	Volumes  []string
	Secrets  []SecretMount
	Ports    []string
	AllPorts bool
	Cmd      []string
}

// SecretMount describes how a secret is injected into a container.
type SecretMount struct {
	Name   string
	Target string
}

// ContainerInfo holds the state of a container.
type ContainerInfo struct {
	ID      string
	Name    string
	State   string
	Running bool
}

package podman

// RunOpts configures a new container.
type RunOpts struct {
	Name     string
	Image    string
	Volumes  []string
	Secrets  []SecretMount
	Ports    []string
	AllPorts bool
	Envs     []string // Environment variables as "KEY=VALUE"
	Cmd      []string
}

// SecretMount describes how a secret is injected into a container.
// By default, secrets are injected as environment variables (type=env).
// When AsFile is true, the secret is mounted as a file at /run/secrets/<Name>
// and Target is set as an env var pointing to the file path.
type SecretMount struct {
	Name   string
	Target string
	AsFile bool
}

// ContainerInfo holds the state of a container.
type ContainerInfo struct {
	ID      string
	Name    string
	State   string
	Running bool
}

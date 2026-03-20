package compose

import (
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GenerateOptions holds parameters for generating compose output.
type GenerateOptions struct {
	// SessionName is used as the compose project name.
	SessionName string
	// ImageRef is the session container image (e.g., "quay.io/cc-deck/demo:latest").
	ImageRef string
	// Domains is the expanded, deduplicated domain list for the proxy allowlist.
	// If nil or empty, no proxy sidecar is generated.
	Domains []string
	// ProxyImage is the container image for the tinyproxy sidecar.
	ProxyImage string
	// ProxyPort is the port for the proxy sidecar (default: 8888).
	ProxyPort int
	// EnvVars is a map of additional environment variables for the session container.
	EnvVars map[string]string
}

const defaultProxyImage = "docker.io/vimagick/tinyproxy:latest"

// ComposeOutput holds all generated files for a compose deployment.
type ComposeOutput struct {
	ComposeYAML    string
	EnvExample     string
	TinyproxyConf  string
	Whitelist      string
}

// Generate produces all files for a compose deployment with optional proxy filtering.
func Generate(opts GenerateOptions) (*ComposeOutput, error) {
	if opts.SessionName == "" {
		return nil, fmt.Errorf("session name is required")
	}
	if opts.ImageRef == "" {
		return nil, fmt.Errorf("image ref is required")
	}

	proxyPort := opts.ProxyPort
	if proxyPort == 0 {
		proxyPort = DefaultProxyPort
	}

	proxyImage := opts.ProxyImage
	if proxyImage == "" {
		proxyImage = defaultProxyImage
	}

	hasProxy := len(opts.Domains) > 0
	composeYAML, err := generateComposeYAML(opts, proxyImage, proxyPort, hasProxy)
	if err != nil {
		return nil, fmt.Errorf("generating compose.yaml: %w", err)
	}

	out := &ComposeOutput{
		ComposeYAML: composeYAML,
		EnvExample:  generateEnvExample(),
	}

	if hasProxy {
		out.TinyproxyConf = GenerateTinyproxyConf(ProxyConfig{
			Port: proxyPort,
		})
		out.Whitelist = GenerateWhitelist(opts.Domains)
	}

	return out, nil
}

// composeFile represents the top-level compose.yaml structure.
type composeFile struct {
	Services map[string]composeService `yaml:"services"`
	Networks map[string]composeNetwork `yaml:"networks,omitempty"`
}

type composeService struct {
	Image       string            `yaml:"image"`
	ContainerName string          `yaml:"container_name,omitempty"`
	Networks    []string          `yaml:"networks,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	EnvFile     []string          `yaml:"env_file,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
	DependsOn   []string          `yaml:"depends_on,omitempty"`
	Command     string            `yaml:"command,omitempty"`
	Stdin       bool              `yaml:"stdin_open,omitempty"`
	TTY         bool              `yaml:"tty,omitempty"`
}

type composeNetwork struct {
	Driver   string `yaml:"driver,omitempty"`
	Internal bool   `yaml:"internal,omitempty"`
}

func generateComposeYAML(opts GenerateOptions, proxyImage string, proxyPort int, hasProxy bool) (string, error) {
	cf := composeFile{
		Services: make(map[string]composeService),
	}

	sessionEnv := make(map[string]string)
	for k, v := range opts.EnvVars {
		sessionEnv[k] = v
	}

	sessionSvc := composeService{
		Image:         opts.ImageRef,
		ContainerName: opts.SessionName,
		EnvFile:       []string{".env"},
		Environment:   sessionEnv,
		Command:       "sleep infinity",
	}

	if hasProxy {
		proxyAddr := fmt.Sprintf("http://proxy:%d", proxyPort)
		sessionSvc.Environment["HTTP_PROXY"] = proxyAddr
		sessionSvc.Environment["HTTPS_PROXY"] = proxyAddr
		sessionSvc.Environment["http_proxy"] = proxyAddr
		sessionSvc.Environment["https_proxy"] = proxyAddr
		sessionSvc.Networks = []string{"internal"}
		sessionSvc.DependsOn = []string{"proxy"}

		proxySvc := composeService{
			Image:         proxyImage,
			ContainerName: opts.SessionName + "-proxy",
			Networks:      []string{"internal", "default"},
			Volumes: []string{
				"./" + filepath.Join("proxy", "tinyproxy.conf") + ":/etc/tinyproxy/tinyproxy.conf:ro",
				"./" + filepath.Join("proxy", "whitelist") + ":/etc/tinyproxy/whitelist",
			},
		}

		cf.Services["proxy"] = proxySvc
		cf.Networks = map[string]composeNetwork{
			"internal": {
				Driver:   "bridge",
				Internal: true,
			},
			"default": {},
		}
	}

	cf.Services["session"] = sessionSvc

	data, err := yaml.Marshal(cf)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func generateEnvExample() string {
	return `# Backend credentials
# For Anthropic:
# ANTHROPIC_API_KEY=sk-ant-...
#
# For Vertex AI:
# GOOGLE_APPLICATION_CREDENTIALS=/run/secrets/gcloud-adc
# CLAUDE_CODE_USE_VERTEX=1
# CLOUD_ML_PROJECT_ID=your-project
# CLOUD_ML_REGION=us-central1
`
}

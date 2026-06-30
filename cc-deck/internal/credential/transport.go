package credential

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/cc-deck/cc-deck/internal/agent"
	"github.com/cc-deck/cc-deck/internal/podman"
)

// InjectContainer prepares credentials for a container workspace.
// Returns env var flags, secret mounts, and keys for the definition.
func InjectContainer(ctx context.Context, wsName string, spec agent.CredentialSpec, resolved ResolvedCredentials) ([]string, []podman.SecretMount, []string, error) {
	var envs []string
	var secrets []podman.SecretMount
	var credentialKeys []string

	for key, val := range resolved.EnvVars {
		if resolved.FileCredential != nil && key == resolved.FileCredential.EnvVar {
			continue
		}
		sName := containerSecretName(wsName, key)

		if info, statErr := os.Stat(val); statErr == nil && !info.IsDir() {
			data, readErr := os.ReadFile(val)
			if readErr != nil {
				return nil, nil, nil, fmt.Errorf("reading credential file %q: %w", val, readErr)
			}
			if err := podman.SecretCreate(ctx, sName, data); err != nil {
				return nil, nil, nil, fmt.Errorf("creating secret %q: %w", key, err)
			}
			secrets = append(secrets, podman.SecretMount{
				Name:   sName,
				AsFile: true,
			})
			envs = append(envs, fmt.Sprintf("%s=/run/secrets/%s", key, sName))
		} else {
			if err := podman.SecretCreate(ctx, sName, []byte(val)); err != nil {
				return nil, nil, nil, fmt.Errorf("creating secret %q: %w", key, err)
			}
			secrets = append(secrets, podman.SecretMount{
				Name:   sName,
				Target: key,
			})
		}
		credentialKeys = append(credentialKeys, key)
	}

	if resolved.FileCredential != nil {
		key := resolved.FileCredential.EnvVar
		localPath := resolved.FileCredential.LocalPath
		sName := containerSecretName(wsName, key)

		data, readErr := os.ReadFile(localPath)
		if readErr != nil {
			return nil, nil, nil, fmt.Errorf("reading credential file %q: %w", localPath, readErr)
		}
		if err := podman.SecretCreate(ctx, sName, data); err != nil {
			return nil, nil, nil, fmt.Errorf("creating file secret %q: %w", key, err)
		}
		secrets = append(secrets, podman.SecretMount{
			Name:   sName,
			AsFile: true,
		})
		envs = append(envs, fmt.Sprintf("%s=/run/secrets/%s", key, sName))
		credentialKeys = append(credentialKeys, key)
	}

	for _, key := range resolved.UnsetVars {
		envs = append(envs, key+"=")
	}

	return envs, secrets, credentialKeys, nil
}

// SSHClient is the subset of ssh.Client used by InjectSSH.
type SSHClient interface {
	Run(ctx context.Context, cmd string) (string, error)
	Upload(ctx context.Context, localPath, remotePath string) error
}

// InjectSSH writes credentials to a remote host via SSH.
func InjectSSH(ctx context.Context, client SSHClient, resolved ResolvedCredentials) error {
	if len(resolved.EnvVars) == 0 && resolved.FileCredential == nil {
		return nil
	}

	var lines []string

	if resolved.FileCredential != nil {
		key := resolved.FileCredential.EnvVar
		localPath := resolved.FileCredential.LocalPath
		remoteName := key

		if _, err := client.Run(ctx, "mkdir -p ~/.config/cc-deck"); err != nil {
			return fmt.Errorf("creating remote config directory: %w", err)
		}
		remotePath := fmt.Sprintf("~/.config/cc-deck/%s", remoteName)
		if err := client.Upload(ctx, localPath, remotePath); err != nil {
			return fmt.Errorf("uploading credential file: %w", err)
		}
		if _, err := client.Run(ctx, fmt.Sprintf("chmod 600 %s", remotePath)); err != nil {
			return fmt.Errorf("setting credential file permissions: %w", err)
		}
		lines = append(lines, fmt.Sprintf("export %s=\"$HOME/.config/cc-deck/%s\"", key, remoteName))
	}

	for key, val := range resolved.EnvVars {
		if resolved.FileCredential != nil && key == resolved.FileCredential.EnvVar {
			continue
		}
		if info, err := os.Stat(val); err == nil && !info.IsDir() {
			remoteName := key
			if _, mkErr := client.Run(ctx, "mkdir -p ~/.config/cc-deck"); mkErr != nil {
				return fmt.Errorf("creating remote config directory: %w", mkErr)
			}
			remotePath := fmt.Sprintf("~/.config/cc-deck/%s", remoteName)
			if upErr := client.Upload(ctx, val, remotePath); upErr != nil {
				return fmt.Errorf("copying credential file for %s: %w", key, upErr)
			}
			if _, chErr := client.Run(ctx, fmt.Sprintf("chmod 600 %s", remotePath)); chErr != nil {
				return fmt.Errorf("setting credential file permissions: %w", chErr)
			}
			lines = append(lines, fmt.Sprintf("export %s=\"$HOME/.config/cc-deck/%s\"", key, remoteName))
			continue
		}
		lines = append(lines, fmt.Sprintf("export %s=%q", key, val))
	}

	for _, key := range resolved.UnsetVars {
		lines = append(lines, fmt.Sprintf("unset %s", key))
	}

	if len(lines) == 0 {
		return nil
	}

	content := strings.Join(lines, "\n") + "\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	writeCmd := fmt.Sprintf(
		"mkdir -p ~/.config/cc-deck && echo %q | base64 -d > ~/.config/cc-deck/credentials.env && chmod 600 ~/.config/cc-deck/credentials.env",
		encoded)

	if _, err := client.Run(ctx, writeCmd); err != nil {
		return fmt.Errorf("writing credential file on remote: %w", err)
	}

	return nil
}

// K8sCredentialResult holds the Kubernetes resources generated for credential injection.
type K8sCredentialResult struct {
	SecretData   map[string][]byte
	EnvVars      []K8sEnvVar
	FileEnvVars  []K8sFileEnvVar
	VolumeMounts []K8sVolumeMount
	UnsetVars    []string
}

// K8sEnvVar describes a Kubernetes env var sourced from a Secret key.
type K8sEnvVar struct {
	Name      string
	SecretKey string
}

// K8sFileEnvVar describes a Kubernetes env var with a fixed value (mount path).
type K8sFileEnvVar struct {
	Name  string
	Value string
}

// K8sVolumeMount describes a volume mount for a file credential.
type K8sVolumeMount struct {
	Name      string
	MountPath string
	SubPath   string
}

// InjectK8s produces Kubernetes Secret data, env var references, and volume
// mounts for credential injection into a K8s workspace.
func InjectK8s(spec agent.CredentialSpec, resolved ResolvedCredentials) (*K8sCredentialResult, error) {
	result := &K8sCredentialResult{
		SecretData: make(map[string][]byte),
	}

	for key, val := range resolved.EnvVars {
		result.SecretData[key] = []byte(val)
		result.EnvVars = append(result.EnvVars, K8sEnvVar{
			Name:      key,
			SecretKey: key,
		})
	}

	if resolved.FileCredential != nil {
		key := resolved.FileCredential.EnvVar
		localPath := resolved.FileCredential.LocalPath
		data, err := os.ReadFile(localPath)
		if err != nil {
			return nil, fmt.Errorf("reading credential file %q: %w", localPath, err)
		}
		result.SecretData[key] = data
		mountPath := "/run/secrets/" + key
		result.VolumeMounts = append(result.VolumeMounts, K8sVolumeMount{
			Name:      "credentials",
			MountPath: mountPath,
			SubPath:   key,
		})
		result.FileEnvVars = append(result.FileEnvVars, K8sFileEnvVar{
			Name:  key,
			Value: mountPath,
		})
	}

	result.UnsetVars = resolved.UnsetVars
	return result, nil
}

// OpenShellClient is the subset of the OpenShell SDK client used for credential injection.
type OpenShellClient interface {
	ExecRun(ctx context.Context, sandboxID string, cmd []string) error
	FileUpload(ctx context.Context, sandboxID, localPath, remotePath string) error
}

// InjectOpenShell uploads credentials to an OpenShell sandbox.
func InjectOpenShell(ctx context.Context, client OpenShellClient, sandboxID string, resolved ResolvedCredentials) error {
	for key, val := range resolved.EnvVars {
		if info, err := os.Stat(val); err == nil && !info.IsDir() {
			remotePath := "/sandbox/.config/cc-deck/" + key
			if err := client.FileUpload(ctx, sandboxID, val, remotePath); err != nil {
				return fmt.Errorf("uploading credential file for %s: %w", key, err)
			}
			if err := injectOpenShellEnvVar(ctx, client, sandboxID, key, remotePath); err != nil {
				return err
			}
			continue
		}
		if err := injectOpenShellEnvVar(ctx, client, sandboxID, key, val); err != nil {
			return err
		}
	}

	if resolved.FileCredential != nil {
		key := resolved.FileCredential.EnvVar
		localPath := resolved.FileCredential.LocalPath
		remotePath := "/sandbox/.config/cc-deck/" + key
		if err := client.FileUpload(ctx, sandboxID, localPath, remotePath); err != nil {
			return fmt.Errorf("uploading credential file: %w", err)
		}
		if err := injectOpenShellEnvVar(ctx, client, sandboxID, key, remotePath); err != nil {
			return err
		}
	}

	for _, key := range resolved.UnsetVars {
		unsetLine := fmt.Sprintf("unset %s", key)
		for _, rcFile := range []string{".bashrc", ".zshrc"} {
			cmd := []string{"bash", "-c", fmt.Sprintf("echo %q >> /sandbox/%s", unsetLine, rcFile)}
			if err := client.ExecRun(ctx, sandboxID, cmd); err != nil {
				return fmt.Errorf("unsetting %s in %s: %w", key, rcFile, err)
			}
		}
	}

	return nil
}

func injectOpenShellEnvVar(ctx context.Context, client OpenShellClient, sandboxID, key, val string) error {
	exportLine := fmt.Sprintf("export %s=%q", key, val)
	for _, rcFile := range []string{".bashrc", ".zshrc"} {
		cmd := []string{"bash", "-c", fmt.Sprintf("echo %q >> /sandbox/%s", exportLine, rcFile)}
		if err := client.ExecRun(ctx, sandboxID, cmd); err != nil {
			return fmt.Errorf("injecting %s into %s: %w", key, rcFile, err)
		}
	}
	return nil
}

func containerSecretName(wsName, key string) string {
	return "cc-deck-" + wsName + "-" + strings.ToLower(strings.ReplaceAll(key, "_", "-"))
}

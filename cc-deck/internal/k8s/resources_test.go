package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/rhuss/cc-mux/cc-deck/internal/config"
)

func TestApplyGitCredentialConfig_SSH(t *testing.T) {
	container := &corev1.Container{Name: "claude"}
	var volumes []corev1.Volume
	profile := config.Profile{
		Backend:             config.BackendAnthropic,
		GitCredentialType:   config.GitCredentialSSH,
		GitCredentialSecret: "my-ssh-key",
	}

	ApplyGitCredentialConfig(container, &volumes, profile)

	// Should have one volume
	if len(volumes) != 1 {
		t.Fatalf("expected 1 volume, got %d", len(volumes))
	}
	if volumes[0].Name != "git-ssh" {
		t.Errorf("expected volume name git-ssh, got %s", volumes[0].Name)
	}
	if volumes[0].Secret.SecretName != "my-ssh-key" {
		t.Errorf("expected secret my-ssh-key, got %s", volumes[0].Secret.SecretName)
	}
	if *volumes[0].Secret.DefaultMode != int32(0o400) {
		t.Errorf("expected default mode 0400, got %d", *volumes[0].Secret.DefaultMode)
	}

	// Should have one volume mount
	foundMount := false
	for _, vm := range container.VolumeMounts {
		if vm.Name == "git-ssh" {
			foundMount = true
			if vm.MountPath != "/home/claude/.ssh" {
				t.Errorf("expected mount path /home/claude/.ssh, got %s", vm.MountPath)
			}
			if !vm.ReadOnly {
				t.Error("expected SSH mount to be read-only")
			}
		}
	}
	if !foundMount {
		t.Error("git-ssh volume mount not found")
	}

	// Should have GIT_SSH_COMMAND env var
	foundEnv := false
	for _, env := range container.Env {
		if env.Name == "GIT_SSH_COMMAND" {
			foundEnv = true
		}
	}
	if !foundEnv {
		t.Error("GIT_SSH_COMMAND env var not found")
	}
}

func TestApplyGitCredentialConfig_Token(t *testing.T) {
	container := &corev1.Container{Name: "claude"}
	var volumes []corev1.Volume
	profile := config.Profile{
		Backend:             config.BackendAnthropic,
		GitCredentialType:   config.GitCredentialToken,
		GitCredentialSecret: "my-git-token",
	}

	ApplyGitCredentialConfig(container, &volumes, profile)

	// Token mode should not add volumes
	if len(volumes) != 0 {
		t.Errorf("expected 0 volumes for token mode, got %d", len(volumes))
	}

	// Should have GIT_TOKEN and GIT_ASKPASS env vars
	envNames := make(map[string]bool)
	for _, env := range container.Env {
		envNames[env.Name] = true
	}

	if !envNames["GIT_TOKEN"] {
		t.Error("GIT_TOKEN env var not found")
	}
	if !envNames["GIT_ASKPASS"] {
		t.Error("GIT_ASKPASS env var not found")
	}
}

func TestApplyGitCredentialConfig_NoCredentials(t *testing.T) {
	container := &corev1.Container{Name: "claude"}
	var volumes []corev1.Volume
	profile := config.Profile{
		Backend: config.BackendAnthropic,
	}

	ApplyGitCredentialConfig(container, &volumes, profile)

	if len(volumes) != 0 {
		t.Errorf("expected 0 volumes when no git creds, got %d", len(volumes))
	}
	if len(container.Env) != 0 {
		t.Errorf("expected 0 env vars when no git creds, got %d", len(container.Env))
	}
	if len(container.VolumeMounts) != 0 {
		t.Errorf("expected 0 volume mounts when no git creds, got %d", len(container.VolumeMounts))
	}
}

func TestApplyGitCredentialConfig_MissingSecret(t *testing.T) {
	container := &corev1.Container{Name: "claude"}
	var volumes []corev1.Volume
	profile := config.Profile{
		Backend:           config.BackendAnthropic,
		GitCredentialType: config.GitCredentialSSH,
		// No GitCredentialSecret
	}

	ApplyGitCredentialConfig(container, &volumes, profile)

	// Should be a no-op
	if len(volumes) != 0 {
		t.Errorf("expected 0 volumes, got %d", len(volumes))
	}
}

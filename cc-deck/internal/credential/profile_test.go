package credential

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cc-deck/cc-deck/internal/config"
)

func TestResolveProfile_EnvSource(t *testing.T) {
	t.Setenv("TEST_API_KEY", "sk-test-123")

	p := config.Profile{
		Harness: "claude",
		Auth: &config.AuthConfig{
			APIKey: &config.CredentialSource{Env: "TEST_API_KEY"},
		},
	}

	got, err := ResolveProfile("my-profile", p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.EnvVars["TEST_API_KEY"] != "sk-test-123" {
		t.Errorf("EnvVars[TEST_API_KEY] = %q, want %q", got.EnvVars["TEST_API_KEY"], "sk-test-123")
	}
}

func TestResolveProfile_EnvSourceMissing(t *testing.T) {
	// Ensure the variable is not set.
	t.Setenv("MISSING_KEY", "")
	os.Unsetenv("MISSING_KEY")

	p := config.Profile{
		Harness: "claude",
		Auth: &config.AuthConfig{
			APIKey: &config.CredentialSource{Env: "MISSING_KEY"},
		},
	}

	_, err := ResolveProfile("my-profile", p)
	if err == nil {
		t.Fatal("expected error for missing env var, got nil")
	}

	var unavail ErrCredentialUnavailable
	if !errors.As(err, &unavail) {
		t.Fatalf("expected ErrCredentialUnavailable, got %T: %v", err, err)
	}
	if unavail.Profile != "my-profile" {
		t.Errorf("Profile = %q, want %q", unavail.Profile, "my-profile")
	}
	if unavail.Reference != "MISSING_KEY" {
		t.Errorf("Reference = %q, want %q", unavail.Reference, "MISSING_KEY")
	}
}

func TestResolveProfile_FileSource(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "api-key.txt")
	if err := os.WriteFile(keyFile, []byte("file-secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	p := config.Profile{
		Harness: "claude",
		Auth: &config.AuthConfig{
			APIKey: &config.CredentialSource{File: keyFile},
		},
	}

	got, err := ResolveProfile("file-profile", p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.FileCredentials) != 1 {
		t.Fatalf("FileCredentials length = %d, want 1", len(got.FileCredentials))
	}
	fc := got.FileCredentials[0]
	if fc.EnvVar != "api_key" {
		t.Errorf("EnvVar = %q, want %q", fc.EnvVar, "api_key")
	}
	if fc.LocalPath != keyFile {
		t.Errorf("LocalPath = %q, want %q", fc.LocalPath, keyFile)
	}
}

func TestResolveProfile_FileSourceMissing(t *testing.T) {
	p := config.Profile{
		Harness: "claude",
		Auth: &config.AuthConfig{
			APIKey: &config.CredentialSource{File: "/nonexistent/path/key.txt"},
		},
	}

	_, err := ResolveProfile("file-profile", p)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}

	var unavail ErrCredentialUnavailable
	if !errors.As(err, &unavail) {
		t.Fatalf("expected ErrCredentialUnavailable, got %T: %v", err, err)
	}
	if unavail.Reference != "/nonexistent/path/key.txt" {
		t.Errorf("Reference = %q, want %q", unavail.Reference, "/nonexistent/path/key.txt")
	}
}

func TestResolveProfile_SecretSource(t *testing.T) {
	p := config.Profile{
		Harness: "claude",
		Auth: &config.AuthConfig{
			APIKey: &config.CredentialSource{Secret: "my-k8s-secret"},
		},
	}

	_, err := ResolveProfile("k8s-profile", p)
	if err == nil {
		t.Fatal("expected error for secret source, got nil")
	}

	var unsupported ErrSecretSourceUnsupported
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected ErrSecretSourceUnsupported, got %T: %v", err, err)
	}
	if unsupported.Profile != "k8s-profile" {
		t.Errorf("Profile = %q, want %q", unsupported.Profile, "k8s-profile")
	}
}

func TestResolveProfile_Login(t *testing.T) {
	p := config.Profile{
		Harness: "claude",
		Auth: &config.AuthConfig{
			Login: true,
		},
	}

	got, err := ResolveProfile("login-profile", p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.EnvVars) != 0 {
		t.Errorf("EnvVars should be empty, got %v", got.EnvVars)
	}
	if len(got.FileCredentials) != 0 {
		t.Errorf("FileCredentials should be empty, got %v", got.FileCredentials)
	}
}

func TestResolveProfile_NoAuth(t *testing.T) {
	p := config.Profile{
		Harness: "claude",
	}

	got, err := ResolveProfile("bare-profile", p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.EnvVars) != 0 {
		t.Errorf("EnvVars should be empty, got %v", got.EnvVars)
	}
	if len(got.FileCredentials) != 0 {
		t.Errorf("FileCredentials should be empty, got %v", got.FileCredentials)
	}
}

func TestResolveProfile_CredentialsFileSource(t *testing.T) {
	dir := t.TempDir()
	credFile := filepath.Join(dir, "creds.json")
	if err := os.WriteFile(credFile, []byte(`{"type":"service_account"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	p := config.Profile{
		Harness: "claude",
		Backend: config.BackendVertex,
		Project: "my-project",
		Region:  "us-east5",
		Auth: &config.AuthConfig{
			APIKey:      &config.CredentialSource{Env: "SKIP_THIS"},
			Credentials: &config.CredentialSource{File: credFile},
		},
	}

	// Set the env var so api_key resolution succeeds too.
	t.Setenv("SKIP_THIS", "some-key")

	got, err := ResolveProfile("vertex-profile", p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have one file credential for the credentials source.
	var found bool
	for _, fc := range got.FileCredentials {
		if fc.EnvVar == "credentials" && fc.LocalPath == credFile {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FileCredentials entry with EnvVar=credentials, got %v", got.FileCredentials)
	}
}

func TestResolveProfile_LegacySecretFields(t *testing.T) {
	// Legacy profiles use APIKeySecret which maps to a secret source via
	// EffectiveAuth, so ResolveProfile should reject them.
	p := config.Profile{
		APIKeySecret: "legacy-secret-name",
	}

	_, err := ResolveProfile("legacy", p)
	if err == nil {
		t.Fatal("expected error for legacy secret source, got nil")
	}

	var unsupported ErrSecretSourceUnsupported
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected ErrSecretSourceUnsupported, got %T: %v", err, err)
	}
}

package ssh

import (
	"testing"
)

func TestBuildCredentialSet_AuthNone(t *testing.T) {
	creds, err := BuildCredentialSet("none", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(creds) != 0 {
		t.Errorf("expected empty creds for auth=none, got %d entries", len(creds))
	}
}

func TestBuildCredentialSet_ExplicitEnvVars(t *testing.T) {
	envVars := map[string]string{
		"MY_VAR": "my_value",
	}
	creds, err := BuildCredentialSet("none", nil, envVars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds["MY_VAR"] != "my_value" {
		t.Errorf("MY_VAR = %q, want %q", creds["MY_VAR"], "my_value")
	}
}

func TestBuildCredentialSet_APIMode(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-123")

	creds, err := BuildCredentialSet("api", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds["ANTHROPIC_API_KEY"] != "sk-test-123" {
		t.Errorf("ANTHROPIC_API_KEY = %q, want %q", creds["ANTHROPIC_API_KEY"], "sk-test-123")
	}
}

func TestBuildCredentialSet_VertexMode(t *testing.T) {
	t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "my-project")
	t.Setenv("CLOUD_ML_REGION", "us-central1")

	creds, err := BuildCredentialSet("vertex", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds["CLAUDE_CODE_USE_VERTEX"] != "1" {
		t.Errorf("CLAUDE_CODE_USE_VERTEX = %q, want %q", creds["CLAUDE_CODE_USE_VERTEX"], "1")
	}
	if creds["ANTHROPIC_VERTEX_PROJECT_ID"] != "my-project" {
		t.Errorf("ANTHROPIC_VERTEX_PROJECT_ID = %q, want %q", creds["ANTHROPIC_VERTEX_PROJECT_ID"], "my-project")
	}
}

func TestBuildCredentialSet_BedrockMode(t *testing.T) {
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIA123")

	creds, err := BuildCredentialSet("bedrock", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds["CLAUDE_CODE_USE_BEDROCK"] != "1" {
		t.Errorf("CLAUDE_CODE_USE_BEDROCK = %q, want %q", creds["CLAUDE_CODE_USE_BEDROCK"], "1")
	}
	if creds["AWS_REGION"] != "us-east-1" {
		t.Errorf("AWS_REGION = %q, want %q", creds["AWS_REGION"], "us-east-1")
	}
}

func TestBuildCredentialSet_CredentialKeys(t *testing.T) {
	t.Setenv("CUSTOM_KEY", "custom_value")

	creds, err := BuildCredentialSet("none", []string{"CUSTOM_KEY"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds["CUSTOM_KEY"] != "custom_value" {
		t.Errorf("CUSTOM_KEY = %q, want %q", creds["CUSTOM_KEY"], "custom_value")
	}
}

func TestDetectAuthMode(t *testing.T) {
	// Clean environment.
	t.Setenv("CLAUDE_CODE_USE_VERTEX", "")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	if got := detectAuthMode(); got != "none" {
		t.Errorf("detectAuthMode() = %q, want %q", got, "none")
	}

	// API key takes priority per spec.
	t.Setenv("ANTHROPIC_API_KEY", "sk-123")
	if got := detectAuthMode(); got != "api" {
		t.Errorf("detectAuthMode() = %q, want %q", got, "api")
	}

	// API key takes priority over Vertex when both are set.
	t.Setenv("CLAUDE_CODE_USE_VERTEX", "1")
	if got := detectAuthMode(); got != "api" {
		t.Errorf("detectAuthMode() with both API key and Vertex = %q, want %q (API key wins)", got, "api")
	}

	// Vertex wins when only Vertex is set.
	t.Setenv("ANTHROPIC_API_KEY", "")
	if got := detectAuthMode(); got != "vertex" {
		t.Errorf("detectAuthMode() with only Vertex = %q, want %q", got, "vertex")
	}
}

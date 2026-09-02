package config

import (
	"strings"
	"testing"
)

func TestAddProfile_Valid(t *testing.T) {
	cfg := &Config{}
	p := Profile{Backend: BackendAnthropic, APIKeySecret: "my-secret"}

	if err := cfg.AddProfile("dev", p); err != nil {
		t.Fatalf("AddProfile() error = %v", err)
	}
	if cfg.Profiles == nil {
		t.Fatal("AddProfile() should initialize Profiles map")
	}
	got, ok := cfg.Profiles["dev"]
	if !ok {
		t.Fatal("expected profile 'dev' to be present")
	}
	if got.APIKeySecret != "my-secret" {
		t.Errorf("got.APIKeySecret = %q, want my-secret", got.APIKeySecret)
	}
}

func TestAddProfile_Invalid(t *testing.T) {
	cfg := &Config{}
	p := Profile{Backend: BackendAnthropic} // missing APIKeySecret

	err := cfg.AddProfile("dev", p)
	if err == nil {
		t.Fatal("AddProfile() error = nil, want error for invalid profile")
	}
	if !strings.Contains(err.Error(), "dev") {
		t.Errorf("error message should reference profile name: %v", err)
	}
	if _, ok := cfg.Profiles["dev"]; ok {
		t.Error("invalid profile should not be added to config")
	}
}

func TestAddProfile_Replace(t *testing.T) {
	cfg := &Config{}
	p1 := Profile{Backend: BackendAnthropic, APIKeySecret: "secret1"}
	p2 := Profile{Backend: BackendAnthropic, APIKeySecret: "secret2"}

	if err := cfg.AddProfile("dev", p1); err != nil {
		t.Fatalf("AddProfile() error = %v", err)
	}
	if err := cfg.AddProfile("dev", p2); err != nil {
		t.Fatalf("AddProfile() error = %v", err)
	}
	if cfg.Profiles["dev"].APIKeySecret != "secret2" {
		t.Errorf("profile was not replaced: got %q", cfg.Profiles["dev"].APIKeySecret)
	}
}

func TestGetProfile_Found(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"dev": {Backend: BackendAnthropic, APIKeySecret: "s"},
	}}

	p, err := cfg.GetProfile("dev")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if p.APIKeySecret != "s" {
		t.Errorf("p.APIKeySecret = %q, want s", p.APIKeySecret)
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	cfg := &Config{}
	_, err := cfg.GetProfile("missing")
	if err == nil {
		t.Fatal("GetProfile() error = nil, want error for missing profile")
	}
}

func TestDeleteProfile_Found(t *testing.T) {
	cfg := &Config{
		DefaultProfile: "dev",
		Profiles: map[string]Profile{
			"dev": {Backend: BackendAnthropic, APIKeySecret: "s"},
		},
	}

	if err := cfg.DeleteProfile("dev"); err != nil {
		t.Fatalf("DeleteProfile() error = %v", err)
	}
	if _, ok := cfg.Profiles["dev"]; ok {
		t.Error("profile should have been deleted")
	}
	if cfg.DefaultProfile != "" {
		t.Errorf("DefaultProfile = %q, want empty after deleting the default profile", cfg.DefaultProfile)
	}
}

func TestDeleteProfile_KeepsOtherDefault(t *testing.T) {
	cfg := &Config{
		DefaultProfile: "other",
		Profiles: map[string]Profile{
			"dev":   {Backend: BackendAnthropic, APIKeySecret: "s"},
			"other": {Backend: BackendAnthropic, APIKeySecret: "s2"},
		},
	}

	if err := cfg.DeleteProfile("dev"); err != nil {
		t.Fatalf("DeleteProfile() error = %v", err)
	}
	if cfg.DefaultProfile != "other" {
		t.Errorf("DefaultProfile = %q, want unchanged 'other'", cfg.DefaultProfile)
	}
}

func TestDeleteProfile_NotFound(t *testing.T) {
	cfg := &Config{}
	if err := cfg.DeleteProfile("missing"); err == nil {
		t.Fatal("DeleteProfile() error = nil, want error for missing profile")
	}
}

func TestSetDefaultProfile_Found(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"dev": {Backend: BackendAnthropic, APIKeySecret: "s"},
	}}

	if err := cfg.SetDefaultProfile("dev"); err != nil {
		t.Fatalf("SetDefaultProfile() error = %v", err)
	}
	if cfg.DefaultProfile != "dev" {
		t.Errorf("DefaultProfile = %q, want dev", cfg.DefaultProfile)
	}
}

func TestSetDefaultProfile_NotFound(t *testing.T) {
	cfg := &Config{}
	if err := cfg.SetDefaultProfile("missing"); err == nil {
		t.Fatal("SetDefaultProfile() error = nil, want error for missing profile")
	}
	if cfg.DefaultProfile != "" {
		t.Errorf("DefaultProfile = %q, want unchanged empty", cfg.DefaultProfile)
	}
}

func TestListProfiles_Empty(t *testing.T) {
	cfg := &Config{}
	names := cfg.ListProfiles()
	if len(names) != 0 {
		t.Errorf("ListProfiles() = %v, want empty", names)
	}
}

func TestListProfiles_Sorted(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"zeta":  {Backend: BackendAnthropic, APIKeySecret: "s"},
		"alpha": {Backend: BackendAnthropic, APIKeySecret: "s"},
		"mid":   {Backend: BackendAnthropic, APIKeySecret: "s"},
	}}

	names := cfg.ListProfiles()
	want := []string{"alpha", "mid", "zeta"}
	if len(names) != len(want) {
		t.Fatalf("ListProfiles() = %v, want %v", names, want)
	}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("ListProfiles()[%d] = %q, want %q", i, names[i], n)
		}
	}
}

func TestProfileValidate_Anthropic(t *testing.T) {
	p := Profile{Backend: BackendAnthropic, APIKeySecret: "s"}
	if err := p.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestProfileValidate_AnthropicMissingKey(t *testing.T) {
	p := Profile{Backend: BackendAnthropic}
	if err := p.Validate(); err == nil {
		t.Error("Validate() error = nil, want error for missing api_key_secret")
	}
}

func TestProfileValidate_Vertex(t *testing.T) {
	p := Profile{Backend: BackendVertex, Project: "proj", Region: "us-central1"}
	if err := p.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestProfileValidate_VertexMissingProject(t *testing.T) {
	p := Profile{Backend: BackendVertex, Region: "us-central1"}
	if err := p.Validate(); err == nil {
		t.Error("Validate() error = nil, want error for missing project")
	}
}

func TestProfileValidate_VertexMissingRegion(t *testing.T) {
	p := Profile{Backend: BackendVertex, Project: "proj"}
	if err := p.Validate(); err == nil {
		t.Error("Validate() error = nil, want error for missing region")
	}
}

func TestProfileValidate_UnknownBackend(t *testing.T) {
	p := Profile{Backend: "bogus"}
	if err := p.Validate(); err == nil {
		t.Error("Validate() error = nil, want error for unknown backend")
	}
}

func TestPromptProfile_Anthropic(t *testing.T) {
	in := strings.NewReader("anthropic\nmy-secret\nclaude-3\n")
	var out strings.Builder

	p, err := PromptProfile(in, &out)
	if err != nil {
		t.Fatalf("PromptProfile() error = %v", err)
	}
	if p.Backend != BackendAnthropic {
		t.Errorf("p.Backend = %q, want %q", p.Backend, BackendAnthropic)
	}
	if p.APIKeySecret != "my-secret" {
		t.Errorf("p.APIKeySecret = %q, want my-secret", p.APIKeySecret)
	}
	if p.Model != "claude-3" {
		t.Errorf("p.Model = %q, want claude-3", p.Model)
	}
}

func TestPromptProfile_Vertex(t *testing.T) {
	in := strings.NewReader("vertex\nmy-project\nus-central1\nmy-creds\n\n")
	var out strings.Builder

	p, err := PromptProfile(in, &out)
	if err != nil {
		t.Fatalf("PromptProfile() error = %v", err)
	}
	if p.Backend != BackendVertex {
		t.Errorf("p.Backend = %q, want %q", p.Backend, BackendVertex)
	}
	if p.Project != "my-project" {
		t.Errorf("p.Project = %q, want my-project", p.Project)
	}
	if p.Region != "us-central1" {
		t.Errorf("p.Region = %q, want us-central1", p.Region)
	}
	if p.CredentialsSecret != "my-creds" {
		t.Errorf("p.CredentialsSecret = %q, want my-creds", p.CredentialsSecret)
	}
}

func TestPromptProfile_UnknownBackend(t *testing.T) {
	in := strings.NewReader("openai\n")
	var out strings.Builder

	_, err := PromptProfile(in, &out)
	if err == nil {
		t.Fatal("PromptProfile() error = nil, want error for unknown backend")
	}
}

func TestPromptProfile_MissingAPIKeySecret(t *testing.T) {
	in := strings.NewReader("anthropic\n\n")
	var out strings.Builder

	_, err := PromptProfile(in, &out)
	if err == nil {
		t.Fatal("PromptProfile() error = nil, want error for empty api_key_secret")
	}
}

func TestPromptProfile_MissingProject(t *testing.T) {
	in := strings.NewReader("vertex\n\n")
	var out strings.Builder

	_, err := PromptProfile(in, &out)
	if err == nil {
		t.Fatal("PromptProfile() error = nil, want error for empty project")
	}
}

func TestPromptProfile_MissingRegion(t *testing.T) {
	in := strings.NewReader("vertex\nmy-project\n\n")
	var out strings.Builder

	_, err := PromptProfile(in, &out)
	if err == nil {
		t.Fatal("PromptProfile() error = nil, want error for empty region")
	}
}

func TestPromptProfile_EOFBeforeBackend(t *testing.T) {
	in := strings.NewReader("")
	var out strings.Builder

	_, err := PromptProfile(in, &out)
	if err == nil {
		t.Fatal("PromptProfile() error = nil, want error on empty input")
	}
}

func TestPromptProfile_EOFDuringAnthropicFields(t *testing.T) {
	in := strings.NewReader("anthropic\n")
	var out strings.Builder

	_, err := PromptProfile(in, &out)
	if err == nil {
		t.Fatal("PromptProfile() error = nil, want error when input ends before api_key_secret")
	}
}

func TestPromptProfile_EOFDuringVertexFields(t *testing.T) {
	in := strings.NewReader("vertex\nmy-project\nus-central1\n")
	var out strings.Builder

	_, err := PromptProfile(in, &out)
	if err == nil {
		t.Fatal("PromptProfile() error = nil, want error when input ends before credentials_secret")
	}
}

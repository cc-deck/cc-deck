# Quickstart: Credential Transport Abstraction

## Adding credential specs to an existing agent

In `internal/agent/<agent>.go`, implement the `CredentialSpecs()` method:

```go
func (c *ClaudeAgent) CredentialSpecs() []CredentialSpec {
    return []CredentialSpec{
        {
            Name:     "api",
            Priority: 10,
            EnvVars: []EnvVarSpec{
                {Name: "ANTHROPIC_API_KEY", Required: true},
            },
        },
        {
            Name:     "vertex",
            Priority: 20,
            EnvVars: []EnvVarSpec{
                {Name: "CLAUDE_CODE_USE_VERTEX", FixedValue: "1"},
                {Name: "ANTHROPIC_VERTEX_PROJECT_ID", Required: true},
                {Name: "CLOUD_ML_REGION"},
                {Name: "ANTHROPIC_MODEL"},
            },
            FileCredential: &FileCredentialSpec{
                EnvVar:      "GOOGLE_APPLICATION_CREDENTIALS",
                DefaultPath: "~/.config/gcloud/application_default_credentials.json",
            },
            Endpoints: []Endpoint{
                {Host: "oauth2.googleapis.com", Port: 443},
            },
        },
    }
}
```

## Creating a workspace with auth mode selection

```bash
# Auto-detect (prompts if multiple modes available)
cc-deck ws new myws --agent claude

# Explicit mode
cc-deck ws new myws --agent claude --auth-mode vertex

# Check what's available
cc-deck ws ls
# NAME   TYPE       AUTH           PROJECT  ...
# myws   container  claude/vertex  myproj   ...
```

## Adding a new agent with custom credentials

1. Create `internal/agent/newagent.go`
2. Implement all Agent interface methods including `CredentialSpecs()`
3. Register via `init()`: `Register(&NewAgent{})`
4. No other code changes needed for credentials to work across all workspace types

## Testing credential resolution

```go
func TestCredentialDetection(t *testing.T) {
    t.Setenv("ANTHROPIC_API_KEY", "test-key")
    
    agent := &ClaudeAgent{}
    specs := agent.CredentialSpecs()
    
    available := credential.Detect(specs)
    assert.Len(t, available, 1)
    assert.Equal(t, "api", available[0].Spec.Name)
}
```

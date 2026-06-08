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
            },
            FileCredential: &FileCredentialSpec{
                EnvVar:      "GOOGLE_APPLICATION_CREDENTIALS",
                DefaultPath: "~/.config/gcloud/application_default_credentials.json",
            },
        },
    }
}
```

That's it. No workspace type code changes needed. The detect-all system picks up the new agent's credentials automatically.

## Creating a workspace (detect-all model)

```bash
# All available credentials are auto-detected and injected
cc-deck ws new myws --type container
# Prints: "Credentials: claude/api, opencode/openai"

# All detected modes are injected, no prompting
# If you have both Claude API key and Vertex credentials,
# both are injected automatically

# Check what's injected (verbose mode)
cc-deck ws ls -v
# NAME   TYPE       INFRA    SESSION  PROJECT   AUTH                         ...
# myws   container  running  none     myproj    claude/api opencode/openai   ...
```

## Testing credential detection

```go
func TestDetectAll(t *testing.T) {
    t.Setenv("ANTHROPIC_API_KEY", "test-key")
    t.Setenv("OPENAI_API_KEY", "test-openai")

    modes := credential.DetectAll()

    // Should find Claude "api" and OpenCode "openai"
    assert.Len(t, modes, 2)
}

func TestMultipleModesInjected(t *testing.T) {
    t.Setenv("ANTHROPIC_API_KEY", "test-key")
    t.Setenv("OPENAI_API_KEY", "test-openai")

    modes := credential.DetectAll()

    // All detected modes are injected, no conflict resolution
    assert.Len(t, modes, 2)
}
```

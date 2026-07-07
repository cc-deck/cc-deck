# Idea Inbox

Ideas captured from code reviews for future brainstorming.

### credential-aware-domain-resolution

- **Source**: triage
- **Date**: 2026-07-07
- **Reference**: PR #10 (078-network-policy-generalization)
- **Summary**: Agents that support multiple credential backends (e.g., OpenCode supports both OpenAI and Anthropic) only declare their primary API domain group. When a user configures an alternate credential mode, the required domains for that backend are missing from the generated policy. Additionally, the policy component endpoint list and the builtin domain group can diverge (e.g., openrouter.ai in opencode.yaml but not in the openai builtin group).

> Devin flagged that OpenCodeAgent.RequiredDomainGroups() returns only ["openai"] despite CredentialSpecs() declaring both openai and anthropic modes. Separately, opencode.yaml includes openrouter.ai:443 but the openai builtin group does not, creating asymmetry between sandbox and container-level domain filtering.

### agent-name-validation

- **Source**: triage
- **Date**: 2026-07-07
- **Reference**: PR #10 (078-network-policy-generalization)
- **Summary**: Manifest.Validate() accepts any string in the agents field. A typo like "cluade" silently produces a policy missing that agent's components with no error or warning. Registry-aware validation during policy assembly would catch this.

> CodeRabbit suggested validating agent names against the agent registry in Manifest.Validate(). Deferred because adding the agent import to manifest.go couples manifest parsing to init() registration order, which is fragile in tests. A better approach may be a warning during policy assembly where the agent package is already imported.

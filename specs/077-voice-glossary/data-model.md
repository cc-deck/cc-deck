# Data Model: Voice Glossary

## New Entity: Glossary

| Field | Type | Description |
|-------|------|-------------|
| `globalTerms` | `[]string` | Terms from config, loaded at startup |
| `cache` | `map[string][]string` | Project glossaries keyed by directory path |

## Modified Entity: Config (Voice section)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Glossary` | `[]string` | `nil` | Global glossary terms for Whisper prompt |

## File Format: .cc-deck/voice-glossary.txt

- One term per line
- Blank lines ignored
- Lines starting with `#` are comments (ignored)
- Terms are trimmed of whitespace
- Example:

```
# Kubernetes ecosystem
Kubernetes
kubectl
Helm
Ingress

# Cloud providers
gRPC
OpenShell
```

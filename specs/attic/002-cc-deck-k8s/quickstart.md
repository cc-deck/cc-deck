# Quickstart: cc-deck Development

## Prerequisites

- Go 1.22+
- A Kubernetes or OpenShift cluster (kubectl/oc configured)
- `claude` binary on PATH (for local testing)

## Project Setup

```bash
cd cc-deck
go mod tidy
go build -o cc-deck ./cmd/cc-deck
```

## First Run

```bash
# 1. Create a credential profile
./cc-deck profile add anthropic-dev

# 2. Create the API key Secret on the cluster
kubectl create secret generic cc-deck-anthropic-key \
  --from-literal=api-key=$ANTHROPIC_API_KEY

# 3. Deploy a session
./cc-deck deploy myproject

# 4. Connect
./cc-deck connect myproject

# 5. When done
./cc-deck delete myproject
```

## Testing

```bash
go test ./...
```

## Project Structure

```
cc-deck/
├── cmd/cc-deck/        # CLI entry point
├── internal/
│   ├── config/         # XDG config, profile management
│   ├── k8s/            # Kubernetes client, resource creation
│   ├── session/        # Session lifecycle (deploy, connect, delete)
│   ├── sync/           # File sync (push/pull)
│   └── network/        # NetworkPolicy, EgressFirewall generation
├── go.mod
└── go.sum
```

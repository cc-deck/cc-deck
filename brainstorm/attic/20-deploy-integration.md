# 20: Deploy Integration

## Problem

cc-deck currently embeds deployment descriptors in the Go binary. This is inflexible
and doesn't account for MCP sidecars or custom image configurations. The build directory
(brainstorm 18) already contains all the information needed to generate deployment
manifests. The deploy command should read from the build directory and produce
target-specific output.

## Design

`cc-deck deploy` replaces the currently embedded deployment descriptors. It reads
the `cc-deck-build.yaml` manifest and generates deployment manifests for the
chosen target.

### Command Interface

```
cc-deck deploy --compose <build-dir> [--output <dir>]
cc-deck deploy --k8s <build-dir> [--output <dir>] [--namespace <ns>]
```

Both commands are deterministic (no AI needed). They read the manifest and
generate files.

## Compose Target

Generates a `compose.yaml` for local development with podman compose or docker compose.

```yaml
# Generated compose.yaml
services:
  cc-deck:
    image: my-team/cc-deck-dev:latest
    volumes:
      - cc-deck-data:/home/coder
    env_file:
      - .env                    # All secrets in one place
    environment:
      - ANTHROPIC_API_KEY       # Passed through from .env
    ports:
      - "6160:6160"             # Zellij web client (optional)
    depends_on:
      - github-mcp
      - slack-mcp

  github-mcp:
    image: ghcr.io/modelcontextprotocol/github-mcp:latest
    env_file:
      - .env
    environment:
      - GITHUB_TOKEN
    network_mode: "service:cc-deck"   # Share localhost with main container

  slack-mcp:
    image: ghcr.io/org/slack-mcp:latest
    env_file:
      - .env
    environment:
      - SLACK_TOKEN
      - SLACK_TEAM_ID
    network_mode: "service:cc-deck"   # Share localhost with main container

volumes:
  cc-deck-data:
```

### Compose Secret Handling

Secrets are managed via a `.env` file (not checked into version control):

```
# .env (generated template, user fills in values)
ANTHROPIC_API_KEY=
GITHUB_TOKEN=
SLACK_TOKEN=
SLACK_TEAM_ID=
```

`cc-deck deploy --compose` generates:
1. `compose.yaml` with service definitions
2. `.env.example` as a template (empty values, lists all required vars)
3. `.gitignore` entry for `.env`

All MCP sidecars use `network_mode: "service:cc-deck"` to share the loopback
interface with the main container. No TLS needed since all traffic stays on
localhost within the Pod-like network namespace.

### Compose Generation Rules

1. Main cc-deck container uses the image from `image.name:image.tag`
2. Each MCP entry becomes a service sharing the main container's network
3. No TLS between containers (localhost communication)
4. Auth env vars sourced from `.env` file (never baked in)
5. `.env.example` generated as a template for required secrets

## Kubernetes Target

Generates Kustomize-based manifests following the established MCP deployment pattern
from the home k3s cluster.

### Generated Structure

```
k8s/
  kustomization.yaml
  statefulset.yaml
  service.yaml
```

### StatefulSet Pattern

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: cc-deck
spec:
  serviceName: cc-deck
  replicas: 1
  template:
    spec:
      containers:
        # Main cc-deck container
        - name: cc-deck
          image: my-team/cc-deck-dev:latest
          env:
            - name: ANTHROPIC_API_KEY
              valueFrom:
                secretKeyRef:
                  name: cc-deck-api
                  key: api-key
          volumeMounts:
            - name: data
              mountPath: /home/coder
          resources:
            requests:
              cpu: 500m
              memory: 1Gi
            limits:
              cpu: "2"
              memory: 4Gi

        # MCP sidecar containers (one per MCP entry)
        # All sidecars share localhost with the main container (same Pod)
        # No TLS needed for loopback communication
        - name: github-mcp
          image: ghcr.io/modelcontextprotocol/github-mcp:latest
          ports:
            - containerPort: 8000
          env:
            - name: GITHUB_TOKEN
              valueFrom:
                secretKeyRef:
                  name: github-mcp-creds
                  key: token
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi

        - name: slack-mcp
          image: ghcr.io/org/slack-mcp:latest
          ports:
            - containerPort: 3001
          env:
            - name: SLACK_TOKEN
              valueFrom:
                secretKeyRef:
                  name: slack-mcp-creds
                  key: token
            - name: SLACK_TEAM_ID
              valueFrom:
                secretKeyRef:
                  name: slack-mcp-creds
                  key: team-id
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi

  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 10Gi
```

### Kustomization

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: cc-deck

resources:
  - statefulset.yaml
  - service.yaml

labels:
  - includeSelectors: true
    pairs:
      app: cc-deck

images:
  - name: my-team/cc-deck-dev
    newTag: latest

generatorOptions:
  disableNameSuffixHash: true
```

### K8s Generation Rules

1. MCP containers become **sidecars** in the same Pod (not separate Deployments)
2. Since MCP sidecars communicate over localhost (loopback), **no TLS needed** for
   inter-container traffic. This is different from the home k3s MCP setup where
   servers are externally exposed via LoadBalancer and need nginx TLS termination.
3. Auth env vars reference Secrets (user creates Secrets separately)
4. Resource requests/limits set from sensible defaults (overridable in manifest)
5. PVC for persistent home directory (user works on repos inside the container)

## Relationship to Flavors (Brainstorm 04)

The flavor system from brainstorm 04 references pre-built images. With the build
feature, the workflow becomes:

1. `cc-deck build init` + AI commands = create custom image
2. `cc-deck build` + `cc-deck push` = publish to registry
3. Register as a flavor in `~/.config/cc-deck/config.yaml`
4. `cc-deck deploy` generates k8s/compose manifests from the build directory

The deploy command replaces the need for embedded deployment descriptors in the
Go binary. The currently embedded descriptors should be removed once this is
implemented.

## Verification

### `cc-deck deploy verify --compose <build-dir>`

1. Runs `podman compose up -d` in the output directory
2. Waits for all services to be healthy
3. Tests MCP connectivity from the main container
4. Reports pass/fail
5. Tears down

### `cc-deck deploy verify --k8s <build-dir> --kubeconfig <path>`

1. Applies manifests to a test namespace
2. Waits for Pod readiness
3. Tests MCP connectivity
4. Reports pass/fail
5. Cleans up test namespace

## Open Questions

- Should the k8s manifests support Helm chart output in addition to Kustomize?
  (Probably not for v1, Kustomize is sufficient.)
- Network policies: should we generate deny-all + allowlist by default?
  (Yes, as discussed in brainstorm 02.)
- Should compose support GPU passthrough for ML workloads?
  (Defer to v2, add `--gpu` flag later.)

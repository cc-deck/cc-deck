# Quickstart: OpenShell Build Target

## Prerequisites

- cc-deck CLI installed
- `podman` available
- An existing project with `.cc-deck/setup/build.yaml` (run `cc-deck build init` if needed)

## 1. Add OpenShell target to build.yaml

```yaml
version: 3

tools:
  - name: git
  - name: "Python 3.12"
  - name: "Node.js 22 LTS"

network:
  allowed_domains:
    - github
    - anthropic
    - python
    - nodejs

targets:
  openshell:
    name: my-project-sandbox
    # tag: latest
    # base: ghcr.io/nvidia/openshell-community/sandboxes/base:latest
    # registry: ghcr.io/myorg
```

## 2. Generate the Containerfile and policy

```bash
claude /cc-deck.build --target openshell
```

This generates:
- `openshell/Containerfile` with tool installations
- `openshell/policy.yaml` with network restrictions from `allowed_domains`
- `openshell/build.sh` for CLI rebuilds

## 3. Build the image

```bash
cc-deck build run --target openshell
```

Or use the standalone script:

```bash
cd .cc-deck/setup/openshell && ./build.sh
```

## 4. Verify the policy

```bash
podman run --rm my-project-sandbox:latest cat /etc/openshell/policy.yaml
```

## 5. (Optional) Add per-binary policy overrides

```yaml
targets:
  openshell:
    name: my-project-sandbox
    policy:
      network_policies:
        git_hosting:
          name: git-hosting
          endpoints:
            - host: github.com
              port: 443
          binaries:
            - path: /usr/bin/git
```

This restricts `github.com` access to the `git` binary only, overriding the permissive auto-generated rule.

## 6. Push to registry

```bash
cc-deck build run --target openshell --push
```

Requires `registry` to be set in `targets.openshell`.

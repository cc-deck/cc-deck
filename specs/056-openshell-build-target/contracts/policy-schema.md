# Contract: OpenShell Policy Schema

## Overview

The OpenShell build target generates a `policy.yaml` file that is embedded in the container image at `/etc/openshell/policy.yaml`. This contract defines the YAML schema, default values, and merge semantics.

## Policy YAML Schema (v1)

```yaml
version: 1

filesystem_policy:
  include_workdir: true
  read_only:
    - /usr
    - /lib
    - /proc
    - /etc
    - /var/log
  read_write:
    - /sandbox
    - /tmp
    - /dev/null
    - /dev/urandom
    - /dev/random
    - /dev/pts

landlock:
  compatibility: best_effort

process:
  run_as_user: sandbox
  run_as_group: sandbox

network_policies:
  <slug>:
    name: <display-name>
    endpoints:
      - host: <domain>
        port: <port>
    binaries:          # optional
      - path: <path>
```

## Default Policy

When no `targets.openshell.policy` overrides are provided, the generated policy uses:

- `filesystem_policy`: FR-013 defaults (read_only: `/usr`, `/lib`, `/proc`, `/etc`, `/var/log`; read_write: `/sandbox`, `/tmp`, `/dev/null`, `/dev/urandom`, `/dev/random`, `/dev/pts`)
- `landlock.compatibility`: `best_effort`
- `process.run_as_user` / `run_as_group`: `sandbox`
- `network_policies`: Auto-generated from `network.allowed_domains`

## Network Policy Generation

### Auto-generation from allowed_domains

For each domain in `network.allowed_domains`:

1. Resolve domain groups using the existing `network.Resolver` (e.g., `github` expands to `github.com`, `*.github.com`, etc.)
2. Create a `network_policies` entry with:
   - Key: slugified domain name (e.g., `github_com`)
   - Name: domain name (e.g., `github.com`)
   - Endpoints: `{host: <domain>, port: 443}`
   - Binaries: all discovered binary paths from tools that use the domain (inferred during Containerfile generation)

### Merge with explicit overrides

When `targets.openshell.policy` is defined:

1. Start with the auto-generated policy from `network.allowed_domains`
2. For each section in the explicit policy (`filesystem_policy`, `landlock`, `process`): replace the default section entirely
3. For `network_policies`: match explicit entries against auto-generated entries by endpoint host
   - If an explicit entry has an endpoint with a host that matches an auto-generated entry's endpoint host, the explicit entry replaces the auto-generated one
   - Explicit entries for hosts not in `allowed_domains` are added as-is
   - Auto-generated entries for hosts not overridden by explicit entries are preserved

## Invariants

- The `version` field is always `1`
- The `filesystem_policy` section is always present (either default or overridden)
- The `landlock` section is always present (either default or overridden)
- The `process` section is always present (either default or overridden)
- The `network_policies` section may be empty (when `allowed_domains` is empty and no explicit policies defined)
- Binary paths in `network_policies` use absolute paths
- All endpoint ports default to 443 when auto-generated

## Build Command Contract

The `/cc-deck.build --target openshell` command:

1. Reads `targets.openshell` from `build.yaml`
2. Generates `openshell/Containerfile` using the OpenShell base image
3. Generates `openshell/policy.yaml` from `network.allowed_domains` + explicit overrides
4. The Containerfile COPYs `openshell/policy.yaml` to `/etc/openshell/policy.yaml`
5. Generates `openshell/build.sh` for standalone rebuilds

The `cc-deck build run --target openshell` command:

1. Detects `openshell/Containerfile` in the setup directory
2. Loads manifest for image reference (`targets.openshell.name:tag`)
3. Builds the image with `podman build`
4. Optionally pushes with `--push` if `registry` is configured

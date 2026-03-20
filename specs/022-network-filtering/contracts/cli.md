# CLI Contract: Domain Management

## `cc-deck domains init`

Seeds `~/.config/cc-deck/domains.yaml` with commented built-in group definitions.

```
cc-deck domains init [--force]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--force` | bool | false | Overwrite existing file (default: preserve user modifications) |

**Output**: Path to created/updated file.
**Exit 0**: File created or updated.
**Exit 1**: File exists with user modifications and `--force` not specified (warn, do not overwrite).

## `cc-deck domains list`

Lists all available domain groups with their source.

```
cc-deck domains list [--output text|json|yaml]
```

**Output format** (text):
```
GROUP        SOURCE     DOMAINS
python       builtin    3
nodejs       extended   4 (builtin + 1 user)
company      user       5
dev-stack    user       12 (includes: python, golang, company)
```

**Exit 0**: Always (even if no groups exist).

## `cc-deck domains show <group>`

Displays expanded domains for a group.

```
cc-deck domains show <group> [--output text|json|yaml]
```

**Output format** (text):
```
Group: python (extended)
Source: builtin + ~/.config/cc-deck/domains.yaml

  pypi.org                    [builtin]
  files.pythonhosted.org      [builtin]
  pypi.python.org             [builtin]
  pypi.internal.corp          [user]
```

**Exit 0**: Group found and expanded.
**Exit 1**: Group not found (error with list of available groups).

## `cc-deck domains blocked <session>`

Displays blocked requests from the proxy access log (Podman sessions only).

```
cc-deck domains blocked <session> [--since <duration>] [--output text|json|yaml]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--since` | duration | 1h | Show blocks from the last N duration |

**Output format** (text):
```
TIMESTAMP            DOMAIN                  METHOD
2026-03-16 10:23:45  evil-server.com         CONNECT
2026-03-16 10:24:01  unknown-cdn.net         CONNECT
```

**Exit 0**: Log parsed (even if no blocks found).
**Exit 1**: Session not found or not a Podman session.

## `cc-deck domains add <session> <domain>`

Adds a domain to a running Podman session by reconfiguring the proxy.

```
cc-deck domains add <session> <domain-or-group>
```

**Exit 0**: Proxy reconfigured.
**Exit 1**: Session not found, not Podman, or domain already present.

## `cc-deck domains remove <session> <domain>`

Removes a domain from a running Podman session.

```
cc-deck domains remove <session> <domain-or-group>
```

**Exit 0**: Proxy reconfigured.
**Exit 1**: Session not found, not Podman, or domain not present.

---

# CLI Contract: Deploy with Network Filtering

## `cc-deck deploy --compose`

Extended with domain filtering flags.

```
cc-deck deploy --compose <build-dir> [--allowed-domains <spec>] [--output <dir>]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--allowed-domains` | string | (from manifest) | Domain override: `+group` (add), `-group` (remove), `group,group` (replace), `all` (disable filtering) |
| `--output` | string | `<build-dir>` | Output directory for generated files |

**Generated files** (when manifest has `network` section):
- `compose.yaml`: Session + proxy sidecar + internal network
- `.env.example`: Credential template
- `proxy/tinyproxy.conf`: Proxy configuration
- `proxy/whitelist`: Expanded domain allowlist

**Generated files** (when manifest has no `network` section):
- `compose.yaml`: Session only (no proxy, no internal network)
- `.env.example`: Credential template

## `cc-deck deploy --k8s`

Extended `--allow-egress` renamed to `--allowed-domains` for consistency.

```
cc-deck deploy --k8s <build-dir> [--allowed-domains <spec>] [--namespace <ns>]
```

Domain groups are expanded before passing to `BuildNetworkPolicy()` / `BuildEgressFirewall()`.

---

# Config File Contract: `domains.yaml`

Location: `$XDG_CONFIG_HOME/cc-deck/domains.yaml`

```yaml
# Group that extends a built-in
<group-name>:
  extends: builtin          # optional: merge with built-in group
  domains:                  # required: list of domain patterns
    - domain.example.com

# Group that includes other groups
<group-name>:
  includes:                 # optional: merge domains from other groups
    - <other-group>
  domains:                  # optional if includes is present
    - domain.example.com
```

**Top-level keys are group names.** No wrapper `groups:` key.

**Parsing rules**:
- Missing file: non-fatal, return empty config
- Invalid YAML: fatal error with file path and line number
- Unknown fields: ignored (forward compatibility)
- Circular includes: fatal error listing the cycle path

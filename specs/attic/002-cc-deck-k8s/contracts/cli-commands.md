# Contract: CLI Command Schema

## Command Tree

```
cc-deck
├── deploy <name>           # Create a new session
├── connect <name>          # Attach to a running session
├── list                    # List all sessions
├── delete <name>           # Remove a session and its resources
├── logs <name>             # Stream Pod logs
├── sync <name>             # Push local directory to Pod
│   └── --pull              # Pull from Pod to local
├── profile
│   ├── add <name>          # Add a credential profile
│   ├── list                # List all profiles
│   ├── use <name>          # Set default profile
│   └── show <name>         # Show profile details
└── version                 # Show CLI version
```

## Global Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--kubeconfig` | | string | `$KUBECONFIG` or `~/.kube/config` | Path to kubeconfig |
| `--namespace` | `-n` | string | Current context namespace | Target namespace |
| `--profile` | `-p` | string | Config default | Credential profile to use |
| `--config` | | string | `$XDG_CONFIG_HOME/cc-deck/config.yaml` | Config file path |
| `--verbose` | `-v` | bool | false | Verbose output |
| `--output` | `-o` | string | `text` | Output format (text, json, yaml) |

## deploy

```
cc-deck deploy <name> [flags]

Flags:
  --profile <name>        Credential profile (overrides default)
  --namespace <name>      Target namespace
  --storage <size>        PVC size (default: "10Gi")
  --image <ref>           Container image (default: from config)
  --sync-dir <path>       Local directory to sync on deploy
  --allow-egress <host>   Additional egress host (repeatable)
  --no-network-policy     Skip NetworkPolicy creation
```

## connect

```
cc-deck connect <name> [flags]

Flags:
  --method <exec|web|port-forward>   Connection method (default: auto-detect)
  --web                              Shorthand for --method web
  --port <port>                      Local port for port-forward (default: 8082)
```

## sync

```
cc-deck sync <name> [flags]

Flags:
  --pull                  Pull changes from Pod to local (default: push)
  --dir <path>            Local directory (default: current directory)
  --exclude <pattern>     Exclude pattern (repeatable, e.g., "node_modules")
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Usage error (invalid flags/args) |
| 3 | Cluster unreachable |
| 4 | Session not found |
| 5 | Resource conflict (already exists) |

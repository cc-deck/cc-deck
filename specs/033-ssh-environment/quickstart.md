# Quickstart: SSH Remote Execution Environment

## Define an SSH Environment

Add to `~/.config/cc-deck/environments.yaml`:

```yaml
version: 1
environments:
  - name: remote-dev
    type: ssh
    host: user@dev.example.com
    workspace: ~/projects/my-app
    auth: auto
```

## Create the Environment

```bash
cc-deck env create remote-dev
```

This runs pre-flight checks (SSH connectivity, tool availability) and offers to install missing tools.

## Attach

```bash
cc-deck attach remote-dev
```

Opens an SSH connection to the remote Zellij session. Detach with `Ctrl+o d` to leave the session running.

## Check Status

```bash
cc-deck status remote-dev
```

## Refresh Credentials

```bash
cc-deck env refresh-creds remote-dev
```

Pushes fresh local credentials to the remote without attaching.

## Sync Files

```bash
cc-deck env push remote-dev ./src
cc-deck env pull remote-dev ./output ./local-output
```

## Harvest Git Commits

```bash
cc-deck env harvest remote-dev --branch feature/remote-work --create-pr
```

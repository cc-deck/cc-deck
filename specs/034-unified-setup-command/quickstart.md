# Quickstart: Unified Setup Command

## Prerequisites

- cc-deck CLI installed (`brew install cc-deck/tap/cc-deck`)
- Claude Code installed
- For SSH targets: Ansible installed (`brew install ansible` on macOS)

## Container Image Workflow

### 1. Initialize

```bash
cd your-project
cc-deck setup init --target container
```

Creates `.cc-deck/setup/` with a manifest template and installs Claude commands.

### 2. Capture Your Environment

In Claude Code:
```
/cc-deck.capture
```

Follow the prompts to select tools, shell config, plugins, and MCP servers. The command discovers your local setup and writes it into the manifest.

### 3. Build the Image

In Claude Code:
```
/cc-deck.build --target container
```

Claude generates a Containerfile, builds the image, and self-corrects on failures. To also push:
```
/cc-deck.build --target container --push
```

### 4. Verify

```bash
cc-deck setup verify --target container
```

## SSH Remote Provisioning Workflow

### 1. Initialize

```bash
cd your-project
cc-deck setup init --target ssh
```

Creates `.cc-deck/setup/` with manifest template and Ansible role skeletons.

### 2. Configure the SSH Target

Edit `.cc-deck/setup/cc-deck-setup.yaml` and fill in the `targets.ssh` section:
```yaml
targets:
  ssh:
    host: dev@your-server
    port: 22
    identity_file: ~/.ssh/id_ed25519
    create_user: true
    user: dev
```

### 3. Capture Your Environment

In Claude Code:
```
/cc-deck.capture
```

Same as the container workflow. The capture command is target-agnostic.

### 4. Provision the Remote

In Claude Code:
```
/cc-deck.build --target ssh
```

Claude generates Ansible playbooks from your manifest, runs them against the remote, and self-corrects on task failures. After convergence, the playbooks can be re-run standalone:

```bash
cd .cc-deck/setup
ansible-playbook -i inventory.ini site.yml
```

### 5. Register an Environment

```bash
cc-deck env create my-remote --type ssh --host dev@your-server
```

The create command runs a lightweight probe to verify the host is provisioned.

### 6. Attach

```bash
cc-deck env attach my-remote
```

## Dual-Target Workflow

Initialize for both targets at once:
```bash
cc-deck setup init --target container,ssh
```

Capture once, build for each target:
```
/cc-deck.capture
/cc-deck.build --target container
/cc-deck.build --target ssh
```

## Checking for Drift

```bash
cc-deck setup diff
```

Shows changes between your manifest and the last-generated artifacts.

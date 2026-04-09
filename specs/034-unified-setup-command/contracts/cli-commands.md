# CLI Command Contracts: cc-deck setup

## cc-deck setup init

**Synopsis**: `cc-deck setup init [dir] [--target <targets>]`

**Purpose**: Scaffold a setup directory with manifest template, Claude commands, and target-specific file structures.

### Arguments

| Argument | Type | Default | Description |
|----------|------|---------|-------------|
| `dir` | positional, optional | `.cc-deck/setup/` | Directory to create |
| `--target` | flag, optional | (none) | Comma-separated targets: `container`, `ssh`, or `container,ssh` |

### Behavior

1. Resolve `dir` to absolute path. If relative, resolve from project root (git root or cwd).
2. Create directory structure:
   - `dir/cc-deck-setup.yaml` (manifest template)
   - `dir/.gitignore`
3. If `--target container` or both:
   - Uncomment the `targets.container` section in the manifest template
   - Create `dir/build-context/` directory
4. If `--target ssh` or both:
   - Uncomment the `targets.ssh` section in the manifest template
   - Create role skeleton directories: `roles/{base,tools,zellij,claude,cc_deck,shell_config,mcp}/tasks/` and `roles/{base,tools,zellij,claude,cc_deck,shell_config,mcp}/defaults/`
   - Create `group_vars/` directory
5. If `--target` omitted:
   - Generate full manifest template with all target sections commented out
6. Install Claude commands to `<project-root>/.claude/commands/`:
   - `cc-deck.capture.md`
   - `cc-deck.build.md`
7. Print summary of created files

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Directory already exists with manifest (use `--force` to overwrite) |

### Post-conditions

- Setup directory exists with manifest template
- Claude commands installed to project `.claude/commands/`
- No Ansible playbook content generated (that happens during `/cc-deck.build`)

---

## cc-deck setup verify

**Synopsis**: `cc-deck setup verify [dir] [--target <target>]`

**Purpose**: Smoke-test a provisioned target for expected tool availability.

### Arguments

| Argument | Type | Default | Description |
|----------|------|---------|-------------|
| `dir` | positional, optional | `.cc-deck/setup/` | Setup directory containing manifest |
| `--target` | flag, required | (none) | Target to verify: `container` or `ssh` |

### Behavior

1. Load manifest from `dir/cc-deck-setup.yaml`
2. If `--target container`:
   - Resolve image name from `targets.container`
   - Run checks inside the container via `podman run --rm <image> <check-cmd>`
   - Checks: `cc-deck version`, `claude --version`, `zellij --version`, each tool from `tools` list
3. If `--target ssh`:
   - Connect to host from `targets.ssh`
   - Run same checks via SSH
   - Checks: `cc-deck version`, `claude --version`, `zellij --version`, each tool from `tools` list
4. Print pass/fail per tool
5. Exit with non-zero if any check fails

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All checks passed |
| 1 | One or more checks failed |
| 2 | Cannot connect to target |

---

## cc-deck setup diff

**Synopsis**: `cc-deck setup diff [dir] [--target <target>]`

**Purpose**: Compare current manifest against last-generated artifacts and report drift.

### Arguments

| Argument | Type | Default | Description |
|----------|------|---------|-------------|
| `dir` | positional, optional | `.cc-deck/setup/` | Setup directory |
| `--target` | flag, optional | (auto-detect) | Target to diff: `container` or `ssh` |

### Behavior

1. Load manifest from `dir/cc-deck-setup.yaml`
2. If `--target container` (or auto-detected from Containerfile existence):
   - Parse the existing Containerfile for installed tools
   - Compare against manifest `tools`, `github_tools`, `plugins`, `settings`
   - Report additions (in manifest, not in Containerfile) and removals
3. If `--target ssh` (or auto-detected from `roles/` existence):
   - Parse existing role task files for installed tools
   - Compare against manifest `tools`, `github_tools`, `plugins`, `settings`
   - Report additions and removals
4. If both targets exist and `--target` not specified, report drift for both

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | No drift detected |
| 1 | Drift detected |

---

## Migration: cc-deck image -> cc-deck setup

The `cc-deck image` command group (init, verify, diff) is renamed to `cc-deck setup`. The command registration in `main.go` changes from `image` to `setup`. No backwards compatibility alias is needed.

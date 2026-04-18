# Data Model: 039-cli-rename-ws-build

**Date**: 2026-04-18

## Summary

This feature makes no data model changes. Per FR-011, config file paths, state file paths, YAML structure, and internal type names remain unchanged.

## Entities (unchanged)

- **EnvironmentType constants**: `local`, `container`, `compose`, `ssh`, `k8s-deploy`, `k8s-sandbox` (no rename)
- **Config files**: `~/.config/cc-deck/config.yaml`, `environments.yaml`, `domains.yaml` (no path changes)
- **State files**: `~/.local/state/cc-deck/state.yaml` (no path changes)
- **Internal `env` package**: `internal/env/` directory and all types unchanged

## Changed Artifacts

| Current | New | Type |
|---------|-----|------|
| `cc-deck-setup.yaml` | `cc-deck-build.yaml` | Build manifest filename (user-facing) |
| `internal/setup/` | `internal/build/` | Go package directory (internal) |

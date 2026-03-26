# Quickstart: Environment Interface and CLI

**Feature**: 023-env-interface | **Date**: 2026-03-20

## Build and Test

```bash
# From project root (NEVER from cc-deck/ subdirectory)
make install     # Build Rust plugin + Go CLI, install plugin
make test        # Run all tests (Go + Rust)
make lint        # Run linters (Go vet + Rust clippy)
```

## Verify New Commands

```bash
# Check env command group is registered
cc-deck env --help

# Create a local environment
cc-deck env create mydev --type local

# List environments
cc-deck env list
cc-deck env list -o json

# Attach to local environment (starts Zellij)
cc-deck env attach mydev

# Check detailed status
cc-deck env status mydev

# Delete environment
cc-deck env delete mydev

# Verify backward compatibility
cc-deck list          # Should delegate to env list
cc-deck deploy --help # Should show deprecation hint
```

## Key Files to Implement

```
cc-deck/internal/env/
  types.go          # EnvironmentType, EnvironmentState, error types
  interface.go      # Environment interface, CreateOpts, SyncOpts
  factory.go        # NewEnvironment factory
  state.go          # StateStore implementation (state.yaml read/write)
  validate.go       # ValidateEnvName
  migrate.go        # Config.yaml session migration
  local.go          # LocalEnvironment implementation

cc-deck/internal/cmd/
  env.go            # NewEnvCmd parent + all subcommand builders

cc-deck/cmd/cc-deck/
  main.go           # Add rootCmd.AddCommand(cmd.NewEnvCmd(gf))
```

## State File Location

```bash
# Default: ~/.local/state/cc-deck/state.yaml
# Override via $XDG_STATE_HOME

# Check current XDG_STATE_HOME
echo ${XDG_STATE_HOME:-~/.local/state}
```

## Testing the Local Environment

```bash
# 1. Create
cc-deck env create test-local --type local

# 2. Verify state file
cat ~/.local/state/cc-deck/state.yaml

# 3. Attach (starts Zellij session cc-deck-test-local)
cc-deck env attach test-local

# 4. In another terminal, verify session exists
zellij list-sessions | grep cc-deck-test-local

# 5. Status with agent sessions
cc-deck env status test-local

# 6. Cleanup
cc-deck env delete test-local
```

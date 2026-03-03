# Brainstorm: Repository Restructuring

**Date:** 2026-03-03
**Status:** parked

## Problem Framing

The cc-mux repository currently has the Rust Zellij plugin code at the root (`src/`, `Cargo.toml`). With the addition of the Go CLI (cc-deck), the repo needs to be restructured into a monorepo with two subdirectories.

## Proposed Structure

```
cc-mux/                          # Repository root, branded as "CC Deck"
├── cc-deck/                     # Go CLI (K8s deployment + management)
│   ├── cmd/
│   ├── internal/
│   ├── go.mod
│   └── go.sum
├── cc-zellij-plugin/            # Rust WASM plugin (Zellij session management)
│   ├── src/
│   ├── Cargo.toml
│   └── Cargo.lock
├── specs/                       # SDD specs (shared)
├── brainstorm/                  # Brainstorm docs (shared)
└── CLAUDE.md                    # Project-level agent context
```

## Decision

Parked: Restructure after the cc-deck spec is created and approved.

## Open Threads

- Move `src/`, `Cargo.toml`, `Cargo.lock`, `.gitignore`, `zellij-*.kdl` into `cc-zellij-plugin/`
- Update CLAUDE.md with both Go and Rust commands
- Update Zellij layout paths to reference the new plugin location
- Consider a top-level Makefile for building both components

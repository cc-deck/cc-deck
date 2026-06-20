# Implementation Plan: OpenShell SSH-to-HTTPS Git Clone Conversion

**Branch**: `072-openshell-ssh-to-https` | **Date**: 2026-06-20 | **Spec**: [spec.md](spec.md)

## Summary

Convert SSH git URLs to HTTPS in `buildCloneCommand()` for OpenShell workspaces, and add `insteadOf` git config to OpenShell images. OpenShell's HTTP CONNECT proxy cannot resolve DNS for SSH (UDP bypasses proxy), but HTTPS works because DNS resolves inside the CONNECT tunnel.

## Technical Context

**Language/Version**: Go 1.25
**Primary Dependencies**: `ws/repos.go` (clone logic), `cc-deck.build.md` (build skill)
**Testing**: `go test ./internal/ws/...` via `make test`
**Target Platform**: macOS (CLI), Linux (sandbox images)
**Project Type**: CLI tool
**Constraints**: Must not affect non-OpenShell workspace types

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Tests and documentation | PASS | Unit tests for URL conversion. No new CLI commands. |
| II. Interface contracts | N/A | No new interfaces |
| III. Build and tool rules | PASS | Using `make test` |

## Project Structure

### Files to Modify

```text
cc-deck/internal/ws/repos.go                      # Add sshToHTTPS(), modify buildCloneCommand()
cc-deck/internal/ws/repos_test.go                  # Add conversion tests
cc-deck/internal/build/commands/cc-deck.build.md   # Add insteadOf to C2
```

## Design Decision

Add a `convertSSH bool` parameter to `buildCloneCommand()`. The caller decides based on workspace type. New `sshToHTTPS()` function handles the URL conversion: `git@<host>:<path>` becomes `https://<host>/<path>`.

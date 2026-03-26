# Implementation Plan: cc-deck (Kubernetes CLI)

**Branch**: `002-cc-deck-k8s` | **Date**: 2026-03-03 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/002-cc-deck-k8s/spec.md`

## Summary

cc-deck is a Go CLI built with Cobra that deploys Claude Code + Zellij sessions as StatefulSets on Kubernetes/OpenShift clusters. It uses client-go with Server-Side Apply for idempotent resource management, Viper for unified configuration (flags + config file), and adrg/xdg for XDG-conformant config paths. OpenShift detection via discovery API enables automatic Route and EgressFirewall creation. Credential profiles support both Anthropic API and Google Vertex AI backends.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: cobra (CLI), viper (config), client-go (K8s API), adrg/xdg (XDG paths), serde/yaml (config parsing)
**Storage**: XDG config file (`~/.config/cc-deck/config.yaml`) for local state; K8s PVCs for remote persistent storage
**Testing**: `go test` with table-driven tests, testcontainers or envtest for K8s integration tests
**Target Platform**: Linux, macOS (CLI binary); Kubernetes 1.24+, OpenShift 4.12+ (deployment target)
**Project Type**: CLI tool
**Constraints**: Must work with both kubectl and oc; NetworkPolicy FQDN filtering requires OpenShift EgressFirewall or compatible CNI

## Constitution Check

Constitution is a template (not yet populated). No gates to evaluate.

## Project Structure

### Documentation (this feature)

```text
specs/002-cc-deck-k8s/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── cli-commands.md
│   └── k8s-resources.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### Source Code

```text
cc-deck/
├── cmd/
│   └── cc-deck/
│       └── main.go              # Entry point, root command
├── internal/
│   ├── config/
│   │   ├── config.go            # Config loading/saving, XDG paths
│   │   └── profile.go           # Profile CRUD operations
│   ├── k8s/
│   │   ├── client.go            # K8s client creation, kubeconfig loading
│   │   ├── discovery.go         # OpenShift/API detection
│   │   ├── resources.go         # Resource builders (StatefulSet, Service, PVC)
│   │   ├── network.go           # NetworkPolicy, EgressFirewall generation
│   │   └── apply.go             # Server-Side Apply helpers
│   ├── session/
│   │   ├── deploy.go            # Deploy workflow
│   │   ├── connect.go           # Connect workflow (exec, web, port-forward)
│   │   ├── delete.go            # Delete workflow
│   │   └── list.go              # List/status workflow
│   ├── sync/
│   │   └── sync.go              # Push/pull file sync via kubectl cp/tar
│   └── cmd/
│       ├── deploy.go            # cobra deploy command
│       ├── connect.go           # cobra connect command
│       ├── delete.go            # cobra delete command
│       ├── list.go              # cobra list command
│       ├── logs.go              # cobra logs command
│       ├── sync.go              # cobra sync command
│       ├── profile.go           # cobra profile subcommands
│       └── version.go           # cobra version command
├── go.mod
└── go.sum
```

**Structure Decision**: Standard Go project layout. `internal/` for non-exported packages. `cmd/` for the binary entry point. Commands in `internal/cmd/` register themselves with the root cobra command. Business logic in `internal/session/`, `internal/k8s/`, `internal/config/`, `internal/sync/`.

## Complexity Tracking

No constitution violations to justify.

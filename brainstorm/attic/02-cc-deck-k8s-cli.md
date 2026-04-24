# Brainstorm: cc-deck Kubernetes CLI

**Date:** 2026-03-03
**Status:** active

## Problem Framing

Developers want to run Claude Code sessions on remote Kubernetes/OpenShift clusters rather than locally. This requires deploying containerized Zellij + Claude Code environments, managing credentials (Anthropic API and Google Vertex AI), securing network egress, syncing git repositories, and connecting to sessions easily. A CLI tool ("cc-deck") would automate all of this.

## Approaches Considered

### A: Monolithic CLI (Go) - CHOSEN
Single Go binary handles everything: K8s manifest generation, deployment, connection, git sync, credential management.
- Pros: Single tool, cohesive UX, easy to install
- Cons: Large scope, K8s manifest generation built-in

### B: CLI + Kustomize base
Go CLI for orchestration, kustomize directory for K8s manifests.
- Pros: Manifests inspectable and customizable
- Cons: Two artifacts to distribute

### C: Helm chart + thin CLI
Helm chart for deployment, thin CLI for connect/sync.
- Pros: Familiar to K8s users
- Cons: Helm not always available on OpenShift

## Key Design Decisions

- **Name:** cc-deck (Go CLI), cc-zellij-plugin (Rust WASM plugin). "CC Deck" is the overall brand.
- **Project structure:** Monorepo with `cc-deck/` (Go) and `cc-zellij-plugin/` (Rust) subdirectories
- **StatefulSet** for stable Pod names and PVC binding
- **Credential profiles** with Anthropic and Vertex AI backends
- **XDG-conformant config** at `~/.config/cc-deck/config.yaml`
- **Pre-built base image** configured at runtime via env vars, ConfigMaps, volume mounts
- **Multiple connection methods:** kubectl exec, Zellij web client, port-forward
- **Egress NetworkPolicy** deny-all with allowlist by default
- **Git sync:** Initial rsync from local + remote clone credentials in Pod

## Open Threads

- Repo restructuring: move existing Rust code from `src/` to `cc-zellij-plugin/` (parked, do after spec)
- Base container image: define Dockerfile with Zellij + Claude Code + git + common tools
- Vertex AI Workload Identity vs service account key for GKE/OpenShift differences

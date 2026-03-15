# Contributing to cc-deck

Contributions are welcome. Whether you are fixing a bug, improving documentation, or proposing a new feature, this guide explains how the process works.

cc-deck uses [Spec-Driven Development (SDD)](https://specify.ing) for feature planning and implementation. SDD is a methodology where features are fully specified before code is written, producing reviewable design artifacts that serve as the contract between what was planned and what gets built. The tooling is provided by the [SDD plugin](https://specify.ing/plugins/claude-code/) for Claude Code, which automates spec generation, planning, task breakdown, and implementation tracking. The underlying scaffolding and template engine is [spec-kit](https://github.com/specify-dev/spec-kit), which manages the spec directory structure, templates, and constitution files.

## Quick Start

```bash
git clone https://github.com/cc-deck/cc-deck.git
cd cc-deck
make install        # Build and install everything
make test           # Run all tests
make lint           # Run linters
```

See the [README](README.md#build-from-source) for prerequisites.

## Bug Fixes and Minor Changes

For bugs and small improvements, you can submit a pull request with code changes directly. No specification is required.

After your change lands, run `/sdd:evolve` in Claude Code to update any affected specifications. This keeps the specs in sync with the actual codebase.

```
/sdd:evolve
```

## Spec-Driven Development

For larger features (new modules, new CLI commands, new deployment patterns, significant behavior changes), cc-deck follows [Spec-Driven Development (SDD)](https://specify.ing). The core idea is simple: write a specification first, get it reviewed, then implement against it. This keeps the design deliberate, reviewable, and traceable.

SDD produces three artifacts per feature: a specification (`spec.md`) that defines the what and why, a plan (`plan.md`) that defines the how, and a task breakdown (`tasks.md`) that defines the execution order. All three live in `specs/<feature>/` and are version-controlled alongside the code they describe.

The [SDD plugin for Claude Code](https://specify.ing/plugins/claude-code/) automates most of this workflow. It generates specs from natural language descriptions, creates implementation plans with architecture decisions, breaks plans into dependency-ordered tasks, and tracks progress as you implement.

### How SDD Works

The workflow has two phases:

1. **Specification PR**: Write a feature spec and submit it for review
2. **Implementation PR(s)**: One or more follow-up PRs that implement the approved spec

### Phase 1: Specification

Create a feature branch and use the SDD plugin to generate the spec:

```bash
git checkout -b 020-my-feature
```

In Claude Code, use the SDD commands to generate the design artifacts:

```
/sdd:brainstorm     # Refine the idea through discussion
/speckit.specify    # Generate the feature specification (spec.md)
/speckit.plan       # Generate the implementation plan (plan.md)
/speckit.tasks      # Generate the task breakdown (tasks.md)
```

This creates a spec directory at `specs/020-my-feature/` containing:

- `spec.md` - Feature specification (requirements, acceptance criteria)
- `plan.md` - Implementation plan (architecture, file structure, tech stack)
- `tasks.md` - Task breakdown (ordered, dependency-aware, parallelism markers)

#### The Spec PR

Submit the specification as its own PR. Include a `REVIEWERS.md` file in the spec directory that lists:

- Who should review the spec
- Any open questions or alternatives considered
- Risk areas that need extra scrutiny

```
specs/020-my-feature/
├── spec.md
├── plan.md
├── tasks.md
└── REVIEWERS.md
```

The spec PR is reviewed and merged before any implementation begins.

### Phase 2: Implementation

After the spec PR is merged, submit one or more implementation PRs that follow the task plan. Each implementation PR should reference the spec it implements.

Implementation PRs can be incremental. You do not need to implement the entire spec in a single PR. The task breakdown in `tasks.md` is designed with phases and dependencies that support incremental delivery.

Use the SDD plugin to drive implementation:

```
/speckit.implement   # Execute tasks from tasks.md
```

### Installing the SDD Plugin

The [SDD plugin](https://specify.ing/plugins/claude-code/) for Claude Code provides the spec commands (`/sdd:*` and `/speckit.*`). Install it from within Claude Code:

```
/plugins install sdd
```

Or add it to your `~/.claude/settings.json`:

```json
{
  "plugins": ["sdd"]
}
```

See the [SDD documentation](https://specify.ing) for the full methodology and plugin reference.

### SDD Commands Reference

| Command | Purpose |
|---------|---------|
| `/sdd:brainstorm` | Refine ideas through collaborative discussion |
| `/speckit.specify` | Generate or update the feature specification |
| `/speckit.plan` | Generate the implementation plan |
| `/speckit.tasks` | Generate the task breakdown |
| `/speckit.implement` | Execute tasks from the plan |
| `/sdd:evolve` | Update specs after code changes |
| `/sdd:review-spec` | Review a spec for completeness |
| `/sdd:review-code` | Check code compliance against spec |

## Code Style

- **Rust**: Follow standard Rust conventions. Run `cargo clippy` before submitting.
- **Go**: Follow standard Go conventions. Run `go vet` before submitting.
- **AsciiDoc**: One sentence per line (semantic line breaks) in `.adoc` files.

## Build Targets

| Target | Description |
|--------|-------------|
| `make build` | Build WASM plugin and Go CLI |
| `make install` | Build and install into Zellij |
| `make test` | Run all tests (Go + Rust) |
| `make lint` | Run linters (Go vet + Rust clippy) |
| `make cross-cli` | Cross-compile CLI for linux/amd64 and linux/arm64 |
| `make demo-image` | Build the demo container image |
| `make base-image` | Build the base container image |

## Project Structure

```
cc-zellij-plugin/   Zellij sidebar plugin (Rust, WASM)
cc-deck/            CLI tool (Go)
docs/               Antora documentation source (AsciiDoc)
demo-image/         Demo container image build
base-image/         Base container image build
specs/              Feature specifications (SDD)
```

## Questions?

Open an issue on GitHub or check the [documentation](https://cc-deck.github.io/docs/).

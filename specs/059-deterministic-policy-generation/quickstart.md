# Quickstart: Deterministic Policy Generation

## What Changed

Policy generation moved from hardcoded Go maps to declarative YAML component files. The `build refresh` command now assembles `openshell/policy.yaml` deterministically from components instead of generating it at runtime.

## Basic Usage

```bash
# Capture workspace (fetches catalog components)
cc-deck capture

# Generate policy from components + manifest
cc-deck build refresh
```

The generated `openshell/policy.yaml` is ready for use. No manual `openshell policy set` needed.

## Adding Custom Endpoints

Create a component file in `.cc-deck/setup/openshell/policies/`:

```yaml
# .cc-deck/setup/openshell/policies/internal-api.yaml
key: internal_api
name: Internal API
match:
  always: true
endpoints:
  - host: api.internal.corp
    port: 8443
```

Run `cc-deck build refresh` to include it in the policy.

## Understanding Components

Components live in three tiers (highest precedence wins):

1. **User-local**: `.cc-deck/setup/openshell/policies/` (your custom endpoints)
2. **Cached catalog**: `.cc-deck/setup/openshell/components/` (fetched by `capture`)
3. **Embedded**: built into the `cc-deck` binary (fallback)

Each component declares match conditions. Only components matching your manifest's tools, credentials, or features are included.

## Verifying Determinism

```bash
cc-deck build refresh
cp openshell/policy.yaml /tmp/policy-1.yaml
cc-deck build refresh
diff openshell/policy.yaml /tmp/policy-1.yaml
# No differences
```

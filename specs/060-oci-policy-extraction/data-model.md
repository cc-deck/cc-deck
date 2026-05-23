# Data Model: OCI Policy Extraction

## Entities

### OCI Image Reference

A string identifying a container image. Can be a local daemon reference or a remote registry reference.

**Formats**:
- `registry.example.com/repo:tag` (remote with tag)
- `registry.example.com/repo@sha256:...` (remote with digest)
- `localhost/repo:tag` (local daemon)
- `repo:tag` (short form, resolved by credential chain)

### Image Label

Key-value metadata stored in the OCI image config.

**Fields**:
- Key: `dev.cc-deck.policy-layer` (follows OCI reverse-DNS convention)
- Value: Layer diff ID as `sha256:<hex>` string

### Extracted Policy

The policy file content extracted from an image layer.

**Fields**:
- Content: Raw bytes of `/etc/openshell/policy.yaml` from the image
- Source: Either "label" (fast path) or "scan" (fallback path)
- TempPath: Filesystem path to the temporary file written for `CreateSandbox`

## Relationships

```
OCI Image Reference
  └── resolves to → OCI Image (local daemon or remote registry)
       ├── Config
       │    └── Labels
       │         └── dev.cc-deck.policy-layer → Layer Diff ID
       └── Layers (ordered, bottom to top)
            └── Layer N (tar archive)
                 └── etc/openshell/policy.yaml → Policy File Content
```

## State Transitions

### Build-Time Flow
```
Image Built (no label) → Layer Scanned → Label Added → Image Updated (with label)
```

### Runtime Extraction Flow
```
Image Reference Parsed
  → Config Fetched
    → Label Found? ─── Yes → Fetch Labeled Layer → Extract File → Done
    │                   No ↓
    └──────────────── Scan Layers (reverse) → File Found? ─── Yes → Extract → Done
                                                              No → Error
```

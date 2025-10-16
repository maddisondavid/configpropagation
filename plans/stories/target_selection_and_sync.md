# Story: Target Selection & Sync

## Goal
Discover Namespaces via label selector and create/update managed ConfigMaps in those Namespaces.

## Why
Enables dynamic onboarding/offboarding and core propagation (sections 5.1, 5.2, 6, 13).

## Scope
- Namespace cache + selector evaluation.
- Create/update target ConfigMaps with managed metadata and data payload.

## Implementation
- Namespace selection: `src/adapters/kube_indexers.py` (Namespace informer + label index).
- Sync logic: `src/agents/sync.py`
  - Compute target manifest: name same as source; copy metadata baseline with managed labels/annotations; copy `data` filtered by `dataKeys`.
  - Upsert with server-side apply or patch; set owner via annotation; never mutate user-owned resources (guard reads annotation before write).
- Hash: `src/core/hash.py` (stable hash from effective data and optional keys order).

## Tests
- `tests/agents/test_target_selection.py`: matchLabels/expressions cases; dynamic add/remove.
- `tests/agents/test_sync_upsert.py`: creates when missing; patches when changed; preserves user metadata.

## Acceptance
- Matching Namespaces get a copy within SLA window.
- Removing label prunes/detaches per policy (see pruning story).
- Data filtered to `dataKeys` when provided.

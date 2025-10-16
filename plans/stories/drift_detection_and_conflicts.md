# Story: Drift Detection & Conflict Policy

## Goal
Detect drift via content hash and enforce conflict policy: `overwrite` (default) or `skip`.

## Why
Ensures single source of truth or records exceptions (sections 5.4, 7, 9, 13).

## Scope
- Stable hash computation and storage on targets.
- Drift evaluation and policy application per reconcile.

## Implementation
- Hashing: `src/core/hash.py` (already used) -> sha256 over normalized JSON of `data`.
- Target annotation: write authoritative hash at successful update.
- Drift check: `src/agents/sync.py` compares target annotation vs source hash.
- Policy:
  - `overwrite`: patch target to match source; update hash annotation; emit event `PropagateOverwritten`.
  - `skip`: do not write; set status.outOfSync with reason=Drift; emit event `PropagateSkipped`.

## Tests
- `tests/agents/test_drift_policy.py`: manual edit to target -> overwrite corrects; skip records drift and leaves target.

## Acceptance
- Hash annotation present on managed targets.
- Drift surfaced in status and metrics; overwrite/skip respected.

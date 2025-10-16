# Story: Failure Handling & Resilience

## Goal
Ensure partial progress with retries, isolate RBAC gaps, and flag oversized payloads.

## Why
Improves robustness and predictable progress (sections 5.7, 8, 9, 10, 11).

## Scope
- Per-namespace retry with exponential backoff.
- Error classification (RBAC vs transient vs permanent).
- Payload size checks and warnings.

## Implementation
- Backoff: `src/core/retry.py` (decorator/helpers), integrate in `sync.py` operations.
- Error classification: `src/core/errors.py` map API errors to categories.
- Size checks: `src/core/limits.py` compute serialized size of data; warn > ConfigMap limits; block if too large.
- Status and metrics updated on failures; continue others.

## Tests
- `tests/agents/test_rbac_isolation.py`: RBAC-denied namespace listed outOfSync; others proceed.
- `tests/core/test_retry_backoff.py`: retries with cap and jitter.
- `tests/core/test_limits.py`: flags large payload and blocks when exceeding threshold.

## Acceptance
- Failures do not block healthy namespaces.
- Repeated failures emit Events and metrics; backoff applied.
- Oversized payloads produce warnings and/or blocks with clear messages.

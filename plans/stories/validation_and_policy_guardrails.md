# Story: Validation & Policy Guardrails

## Goal
Enforce safe defaults, schema validation, and optional admission policies (defaulting + validating webhooks).

## Why
Prevents dangerous configurations and standardizes behavior (sections 7, 13, 15).

## Scope
- Schema validation in CRD (enums, minimums, requireds).
- Mutating webhook for defaults.
- Validating webhook for risky selectors or settings.

## Implementation
- CRD OpenAPI: ensure `enum` for `strategy.type`, `conflictPolicy`; `minimum` for `batchSize` and `resyncPeriodSeconds`.
- Webhooks (optional): `src/adapters/webhooks/defaulting.py`, `src/adapters/webhooks/validation.py`.
  - Default: strategy=rolling, batchSize=5, prune=true, conflictPolicy=overwrite.
  - Validate: deny `batchSize` < 1; optional guard for wide-open selectors in prod (feature-flagged).
- Config: `configs/webhook/` manifests and service registration.

## Tests
- `tests/adapters/test_webhook_defaulting.py`: omitted fields defaulted.
- `tests/adapters/test_webhook_validation.py`: invalid updates rejected; immutable fields guarded if chosen (e.g., sourceRef optional immutability).

## Acceptance
- Invalid specs rejected with clear errors; safe defaults applied.
- Optional: attempts to change immutable fields blocked with reason.

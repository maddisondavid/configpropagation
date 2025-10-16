# Story: Reconciliation Engine

## Goal
Implement controller reconcile loop that reacts to CR, source ConfigMap, Namespace label changes, and periodic resync.

## Why
Delivers the heart of synchronization and SLA for propagation time (sections 5, 6, 8, 13, 15).

## Scope
- Watches and event handlers.
- Desired state computation (including `dataKeys`).
- Per-namespace work queueing with batching support.

## Implementation
- Controller entry: `src/agents/operator.py` (manager, informers, metrics server).
- Reconcilers: `src/agents/reconciler.py`
  - Triggers: CR add/update/delete, ConfigMap change, Namespace label changes, periodic tick.
  - Steps:
    1) Load CR + source ConfigMap; compute effective data (filter `dataKeys`).
    2) Resolve selected Namespaces from `namespaceSelector` via cache.
    3) Enqueue targets per rollout strategy (immediate/rolling).
    4) Update status (targetCount) and emit events.
- Work queues: `src/core/queue.py` simple rate-limited queue with backoff.
- Kube adapter: `src/adapters/kube_client.py` thin client methods used by reconciler.

## Tests
- `tests/agents/test_reconciler_triggers.py`: verify reconcile on CR and source change.
- `tests/agents/test_reconciler_plan.py`: computes effective data and target set correctly.

## Acceptance
- Creating CR triggers reconciliation and plan logs/events.
- Updating source ConfigMap enqueues updates within seconds.
- Labeling a Namespace in/out updates planned targets.

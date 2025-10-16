# Story: Config Propagation CRD

## Goal
Define the `ConfigPropagation` CRD and metadata conventions to declare propagation from a single source ConfigMap to many target namespaces.

## Why
Enables source-of-truth sync, label targeting, rollout control, conflict policy, and lifecycle per spec sections 3, 5, 7, 13.

## Scope
- CRD (OpenAPI schema, defaults, validation).
- Managed labels/annotations and finalizer.
- Core types/constants for controller use.

## API
group: `configpropagator.platform.example.com`
version: `v1alpha1`
kind: `ConfigPropagation`

spec:
- `sourceRef` { namespace: string, name: string } (required)
- `namespaceSelector` { matchLabels?, matchExpressions? } (required)
- `dataKeys` [string] (optional)
- `strategy` { type: "rolling"|"immediate", batchSize: int>=1 (default 5) }
- `conflictPolicy`: "overwrite"|"skip" (default "overwrite")
- `prune`: bool (default true)
- `resyncPeriodSeconds`: int>=10 (optional)

status:
- `conditions`: Ready, Progressing, Degraded
- `targetCount`, `syncedCount`, `outOfSyncCount`
- `outOfSync`: [{ namespace, reason, message }]
- `lastSyncTime`

managed metadata:
- label: `configpropagator.platform.example.com/managed: "true"`
- annotation: `configpropagator.platform.example.com/source: <ns>/<name>`
- annotation: `configpropagator.platform.example.com/hash: <sha256>`

finalizer:
- `configpropagator.platform.example.com/finalizer`

## Implementation
- Add CRD: `configs/crd/configpropagation.yaml` (schema, defaults via x-kubernetes, printer columns).
- Add core types: `src/core/types.py` (dataclasses/TypedDict or pydantic models for Spec/Status).
- Add constants: `src/core/constants.py` (labels, annotations, finalizer, condition types).
- Add validation helpers: `src/core/validation.py` (batchSize>=1; allowed enums; positive resync).

## Tests
- `tests/core/test_crd_schema.py`: ensure required fields; enum validation; defaults applied by webhook or controller.
- `tests/core/test_constants.py`: assert key stability.

## Acceptance
- CRD installs; invalid enum or batchSize<1 rejected.
- Defaulting produces: rolling, batchSize=5, prune=true, conflict=overwrite.
- Managed metadata contract documented and used by controller.

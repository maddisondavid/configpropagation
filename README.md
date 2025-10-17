# configpropagation

A reference Kubernetes controller that keeps ConfigMaps in multiple namespaces synchronized with a designated source ConfigMap. The controller watches `ConfigPropagation` custom resources (CRs), tracks drift using hashing, and applies opinionated defaults so cluster operators can share configuration safely and predictably.

## How it Works
1. **Watch resources** – The reconciler listens for updates to the `ConfigPropagation` CR, the source ConfigMap, and matching namespaces.
2. **Default + validate** – Each reconcile loop applies server-side defaults and validation, ensuring strategies and policies conform to supported values.
3. **Project desired state** – The controller fetches source data, filters keys if requested, determines target namespaces, and plans rollouts based on the chosen update strategy.
4. **Synchronize targets** – Managed ConfigMaps are created or updated with managed labels, annotations, and hashes to avoid redundant work and honor conflict policies.
5. **Garbage-collect** – Namespaces that are no longer selected are either pruned or detached based on the spec, and finalization reuses the same cleanup logic.

## Getting Started
1. **Build the controller**
   ```bash
   go build ./...
   ```
2. **Run tests** to verify functionality before deploying:
   ```bash
   go test ./...
   ```
3. **Deploy** – Package the controller into your preferred deployment mechanism (Deployment, Helm chart, etc.) and apply the CRD plus controller manifest.
4. **Configure access** – Grant the service account permission to read ConfigMaps in the source namespace, read namespaces, and create/update/delete ConfigMaps cluster-wide.

> **Tips:**
> - The controller respects the `BATCH_SIZE` environment variable when a `strategy.batchSize` is not set in a CR.
> - Admission guardrails can be toggled per cluster via `STRICT_SELECTOR_GUARD` (rejects wide-open selectors) and `ENFORCE_SOURCE_IMMUTABILITY` (blocks changing the source ConfigMap on updates).

## Example `ConfigPropagation`
```yaml
apiVersion: platform.example.com/v1alpha1
kind: ConfigPropagation
metadata:
  name: shared-config
spec:
  sourceRef:
    namespace: platform
    name: base-config
  namespaceSelector:
    matchLabels:
      team: payments
  dataKeys:
    - app.properties
    - logging.yaml
  strategy:
    type: rolling
    batchSize: 3
  conflictPolicy: overwrite
  prune: true
  resyncPeriodSeconds: 300
```

## Spec Reference
The CR spec lives in `pkg/core/types.go`. Fields mirror Kubernetes conventions and are validated in `pkg/core/validation.go`.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `sourceRef.namespace` | string | ✅ | Namespace of the source ConfigMap to copy from. |
| `sourceRef.name` | string | ✅ | Name of the source ConfigMap. |
| `namespaceSelector` | object | ✅ | Label selector that picks target namespaces; supports `matchLabels` and `matchExpressions` just like core Kubernetes selectors. |
| `dataKeys` | string array | ❌ | Optional whitelist of keys within the source ConfigMap. When omitted, all keys are propagated. |
| `strategy.type` | string | ❌ | Update rollout mode. Supports `rolling` (default) and `immediate`. Rolling applies the batch-size window before updating the rest. |
| `strategy.batchSize` | int32 | ❌ | Number of namespaces updated per reconcile when `strategy.type=rolling`. Defaults to the `BATCH_SIZE` env var (falling back to `5`). Must be ≥1. |
| `conflictPolicy` | string | ❌ | How to handle existing unmanaged ConfigMaps. `overwrite` (default) replaces data, `skip` leaves them untouched. |
| `prune` | bool | ❌ | Whether to delete ConfigMaps from namespaces that no longer match the selector. Defaults to `true`. If `false`, managed markers are removed but data is preserved. |
| `resyncPeriodSeconds` | int32 | ❌ | Optional periodic resync interval. Must be ≥10 seconds if set. |

## Status Fields
The controller reports progress and drift under `.status` with familiar condition patterns and per-namespace diagnostics.

- `conditions`: Readiness, progress, and degradation signals.
- `targetCount`, `syncedCount`, `outOfSyncCount`: Aggregated rollout metrics.
- `outOfSync`: Array of namespace-specific issues (e.g., hash mismatches or permission errors).
- `lastSyncTime`: Timestamp of the most recent synchronization in RFC3339 format.

## Operational Tips
- Schedule reconciles via `.spec.resyncPeriodSeconds` for ConfigMaps that change outside controller watch scope.
- Combine label selectors and expressions to target whole teams or environments.
- Use `conflictPolicy: skip` for namespaces that occasionally need local overrides.
- Disable pruning when performing phased migrations so previous targets keep a final copy after deselection.

For performance tuning guidance—including worker counts and batching strategies—see `docs/performance.md`.

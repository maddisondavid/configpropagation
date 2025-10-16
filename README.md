# configpropagation

A reference Kubernetes operator that propagates configuration from a source object to labeled target namespaces.

## Key Features
- Hash-based drift detection and reconciliation logic scoped under `src/controllers/configpropagation`.
- Core primitives for strategies, conflict policies, queueing, and validation under `src/core`.
- Comprehensive unit tests covering reconciliation scenarios and spec validation.

## Configuration Defaults
The controller applies opinionated defaults when reconciling a `ConfigPropagation` resource:
- `strategy.type` defaults to rolling updates and `strategy.batchSize` falls back to the `BATCH_SIZE` environment variable (default `5`).
- `conflictPolicy` defaults to `Overwrite` when unspecified.
- `prune` defaults to `true` to clean up orphaned resources.

## Development
- Run `go test ./...` to execute the unit test suite.
- Use `make lint` (when configured) and standard Go formatting before submitting changes.
- See `docs/performance.md` for tuning guidance on batch size, workers, and resync intervals.

# Story: Pruning & Finalization

## Goal
Clean up managed ConfigMaps when deselected or when CR is deleted. Use finalizers for safe lifecycle.

## Why
Prevents configuration sprawl and orphaned resources (sections 5.5, 7, 13).

## Scope
- Prune mode when `prune=true`.
- Detach mode when `prune=false` (remove managed markers, leave copy).
- Finalizer for ordered teardown.

## Implementation
- Finalizer management: `src/agents/reconciler.py` add/remove finalizer; block delete until cleanup.
- Cleanup: `src/agents/cleanup.py`
  - Enumerate targets with `managed=true` + source annotation matching CR.
  - If deselected and prune: delete; else detach (remove labels/annotations).
  - On CR delete: apply same policy to all previously selected namespaces.

## Tests
- `tests/agents/test_prune_on_unlabel.py`: deselection triggers delete/detach as configured.
- `tests/agents/test_finalizer_cleanup.py`: CR deletion removes all targets then finalizer.

## Acceptance
- CR delete yields no orphaned finalizers.
- Deselected namespaces are cleaned up within SLA per policy.

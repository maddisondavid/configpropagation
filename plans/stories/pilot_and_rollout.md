# Story: Pilot & Rollout

## Goal
Execute a phased rollout: pilot, scale-up, and organization-wide adoption with KPIs collection.

## Why
De-risks adoption and tunes defaults (sections 15, 2).

## Scope
- Pilot in a small set of Namespaces; capture KPIs (p95 propagation time, error rates, drift MTTD).
- Scale to critical apps; tune batch size and alert thresholds.
- Org-wide adoption; publish dashboards and runbooks; enforce defaults.

## Implementation
- `plans/config-propagator-operator.md`: extend with concrete environments and thresholds.
- Scripts (optional): `tools/kpi_collector.py` to scrape metrics and compute p95.
- Update defaults in webhook after pilot learnings.

## Tests
- N/A (execution plan). Validate KPI script locally if added.

## Acceptance
- Pilot report with metrics and findings.
- Tuned defaults documented; rollout checklist produced and followed.

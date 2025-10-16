# Story: Documentation & Runbooks

## Goal
Provide docs for onboarding, operations, dashboards, and governance.

## Why
Supports adoption, troubleshooting, and compliance (sections 14, 9, 12).

## Scope
- Quick start, service catalog entry, runbook, dashboard templates, label governance doc.

## Implementation
- `docs/quickstart.md`: label taxonomy, define CR, choose strategy.
- `docs/runbook.md`: RBAC errors, payload limits, rollback via source revert, finalizer cleanup.
- `docs/changelog.md`: track notable changes.
- `docs/dependencies.md`: record dependencies/licenses.
- `assets/dashboards/` Grafana JSON templates mapping to metrics.
- `configs/README.md`: env vars, flags, webhook enablement, installation steps.

## Tests
- N/A (docs). Include links validation via CI optional.

## Acceptance
- Docs are complete and actionable; dashboard templates import and render correctly.

# Story: RBAC & Installation

## Goal
Provide least-privilege RBAC and deployment manifests/Helm for the operator.

## Why
Secure, compliant installation enabling multi-namespace operations (sections 11, 12).

## Scope
- RBAC rules scoped to: read source ns/ConfigMap; list/watch Namespaces; manage ConfigMaps in target Namespaces; manage CRD/CRs; Events.
- Deployment with configurable flags (batch size, resync, metrics addr).

## Implementation
- Manifests: `configs/rbac/role.yaml`, `rolebinding.yaml`, `clusterrole.yaml`, `clusterrolebinding.yaml` (as needed), `serviceaccount.yaml`.
- Deployment: `configs/deploy/operator.yaml` (or Helm chart in `configs/helm/configpropagator/`).
- Flags/env: `configs/README.md` document tunables and env vars.

## Tests
- `tests/fixtures/manifests/` golden files; linter validates verbs and resources required by adapters.

## Acceptance
- Install succeeds with provided manifests; operator can read source, list Namespaces, and manage target ConfigMaps per spec.
- No extra privileges beyond necessary resources.

# Story: RBAC & Installation

## Goal
Provide least-privilege RBAC and deployment manifests/Helm for the operator.

## Why
Secure, compliant installation enabling multi-namespace operations (sections 11, 12).

## Scope
- RBAC rules scoped to: read source ns/ConfigMap; list/watch Namespaces; manage ConfigMaps in target Namespaces; manage CRD/CRs; Events.
- Deployment with configurable flags (batch size, resync, metrics addr) exposed via Helm values.
- Package ships with an installation story that reuses the existing Dockerfile and Helm chart.
- Provide a Stafford configuration for local development installs (namespace scoping, Helm overrides, image tags).

## Implementation
- RBAC manifests: `configs/rbac/role.yaml`, `rolebinding.yaml`, `clusterrole.yaml`, `clusterrolebinding.yaml` (as needed), `serviceaccount.yaml`; mirrored as Helm templates under `charts/configpropagation/templates/rbac-*.yaml`.
- Helm deployment: document chart values in `charts/configpropagation/values.yaml`; ensure templates consume existing image published from the root `Dockerfile` and surface key flags/env via `values.yaml`.
- Delivery assets: ship a `configs/stafford/dev.yaml` profile describing a development Stafford install (image repository/tag, namespace list, feature gates, config map sources).
- Flags/env: `configs/README.md` documents tunables, Stafford usage, and Helm values for operators.

## Tests
- `tests/fixtures/manifests/` golden files; linter validates verbs and resources required by adapters.
- Helm chart unit tests (e.g., `helm template --values configs/stafford/dev.yaml`) validate templates render expected RBAC bindings and deployment.

## Acceptance
- Install succeeds with provided manifests; operator can read source, list Namespaces, and manage target ConfigMaps per spec.
- `helm install` using the documented chart and Stafford dev config deploys the operator image built from the Dockerfile without additional manual edits.
- No extra privileges beyond necessary resources.

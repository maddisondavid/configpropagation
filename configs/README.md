# Configuration Assets

This directory packages the manifests and helper assets required to install the
ConfigPropagation controller with either plain YAML or the provided Helm chart.

## RBAC manifests

`configs/rbac/` contains the least-privilege service account, role, and role
bindings used by the controller:

- `serviceaccount.yaml` – Service account scoped to the `configpropagation-system`
  namespace.
- `role.yaml` / `rolebinding.yaml` – Namespace-scoped permissions that cover the
  leader-election ConfigMap and Lease objects.
- `clusterrole.yaml` / `clusterrolebinding.yaml` – Cluster-scoped permissions
  that allow the controller to read ConfigMaps from the source namespace, list
  namespaces, emit events, and manage the `ConfigPropagation` custom resources.

Apply them as-is or tailor the namespace before applying:

```bash
kubectl create namespace configpropagation-system
kubectl apply -f configs/rbac/
```

## Helm chart values

The `charts/configpropagation` chart exposes operational tunables through
`values.yaml`. Key options include:

| Value | Description |
| ----- | ----------- |
| `image.repository` / `image.tag` | Controller image to deploy (defaults to the Dockerfile build output). |
| `env.batchSize`, `env.workers`, `env.resyncSeconds` | Mirrors the controller environment variables for rollout tuning. |
| `metrics.enabled`, `metrics.bindAddress`, `metrics.port` | Expose or disable the metrics endpoint and choose the bind address. Set `metrics.bindAddress` to `0` to fully disable metrics. |
| `healthProbe.bindAddress` | Address used by the readiness and liveness probes. |
| `webhook.enabled`, `webhook.port` | Toggle admission webhooks and configure their port. |
| `leaderElection.enabled` | Enables the `--leader-elect` flag when running multiple replicas. |
| `rbac.leaderElection.createRole` | Skip Role/RoleBinding creation when reusing an existing leader-election Role. |
| `args`, `env.extra` | Pass additional controller arguments or environment variables for feature gates and config sources. |

Render the manifests for inspection:

```bash
helm template configpropagation charts/configpropagation \
  --namespace configpropagation-system \
  --values configs/stafford/dev.yaml
```

## Stafford (Skaffold) development profile

Local developer installs can use the provided Skaffold configuration and values
file under `configs/stafford/`:

- `configs/stafford/dev.yaml` – Helm values tailored for iterative development
  (image overrides, namespace, and sample feature flags).
- `skaffold.yaml` – Builds the controller image from the repository Dockerfile
  and installs the chart into the `configpropagation-system` namespace with the
  above values.

To iterate locally:

```bash
skaffold dev --profile local
```

The `dev` profile pushes image updates with a predictable tag, scopes the
controller to development namespaces, and exposes feature gates through
`env.extra` so changes can be tested without editing manifests directly.

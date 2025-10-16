# ConfigPropagation Examples

Rendered samples demonstrating common `ConfigPropagation` custom resource shapes. Apply them to a cluster after installing the CRD in `configs/crd/configpropagation.yaml`.

- `configpropagation-minimal.yaml` – smallest valid resource with just the required fields.
- `configpropagation-rolling.yaml` – rolling rollout with label selector expressions, key filtering, and explicit defaults.
- `configpropagation-immediate-skip.yaml` – immediate rollout using conflict skipping and disabled pruning.

Use `kubectl apply -f <file>` to create the example resources and inspect their status with `kubectl get configpropagations -n <namespace>`.

package configpropagation

import (
	"fmt"
	"sort"

	"configpropagation/pkg/adapters"
	"configpropagation/pkg/core"
)

// Key identifies a ConfigPropagation by namespace/name.
type Key struct {
	Namespace string
	Name      string
}

// Reconciler wires the kube client and a simple work queue.
type Reconciler struct {
	client adapters.KubeClient
	queue  *core.WorkQueue[Key]
}

func NewReconciler(client adapters.KubeClient) *Reconciler {
	return &Reconciler{client: client, queue: core.NewWorkQueue[Key]()}
}

// OnCRChange enqueues a reconcile when the CR changes.
func (r *Reconciler) OnCRChange(ns, name string) { r.queue.Add(Key{Namespace: ns, Name: name}) }

// OnSourceChange enqueues a reconcile when the source ConfigMap changes.
func (r *Reconciler) OnSourceChange(ns, name string) { r.queue.Add(Key{Namespace: ns, Name: name}) }

// OnNamespaceLabelChange enqueues a reconcile for selection changes.
func (r *Reconciler) OnNamespaceLabelChange(ns, name string) {
	r.queue.Add(Key{Namespace: ns, Name: name})
}

// Reconcile performs one loop for the next item in the queue.
func (r *Reconciler) Reconcile(spec *core.ConfigPropagationSpec) ([]string, error) {
	if spec == nil {
		return nil, fmt.Errorf("spec is nil")
	}
	core.DefaultSpec(spec)
	if err := core.ValidateSpec(spec); err != nil {
		return nil, err
	}
	return reconcileImpl(r.client, spec)
}

// Internal implementation separated for testability and full coverage.
func reconcileImpl(client adapters.KubeClient, spec *core.ConfigPropagationSpec) ([]string, error) {
	srcData, err := client.GetSourceConfigMap(spec.SourceRef.Namespace, spec.SourceRef.Name)
	if err != nil {
		return nil, fmt.Errorf("get source: %w", err)
	}
	effective := computeEffective(srcData, spec.DataKeys)
	targets, err := listTargets(client, spec.NamespaceSelector)
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}
	sort.Strings(targets)
	batch := int32(5)
	if spec.Strategy.BatchSize != nil {
		batch = *spec.Strategy.BatchSize
	}
	planned := planTargets(targets, spec.Strategy.Type, batch)
	if err := syncTargets(client, planned, spec.SourceRef.Name, effective, spec.SourceRef.Namespace, spec.ConflictPolicy); err != nil {
		return nil, err
	}
	// Cleanup deselected namespaces per prune policy
	if err := cleanupDeselected(client, spec, targets); err != nil {
		return nil, err
	}
	return planned, nil
}

func nilIfEmpty[K comparable, V any](m map[K]V) map[K]V {
	if len(m) == 0 {
		return nil
	}
	return m
}

func computeEffective(src map[string]string, keys []string) map[string]string {
	if src == nil {
		src = map[string]string{}
	}
	out := map[string]string{}
	if len(keys) == 0 {
		for k, v := range src {
			out[k] = v
		}
		return out
	}
	for _, k := range keys {
		if v, ok := src[k]; ok {
			out[k] = v
		}
	}
	return out
}

func listTargets(c adapters.KubeClient, sel *core.LabelSelector) ([]string, error) {
	var exprs []adapters.LabelSelectorRequirement
	for _, e := range sel.MatchExpressions {
		exprs = append(exprs, adapters.LabelSelectorRequirement{Key: e.Key, Operator: e.Operator, Values: e.Values})
	}
	return c.ListNamespacesBySelector(nilIfEmpty(sel.MatchLabels), exprs)
}

func syncTargets(c adapters.KubeClient, planned []string, name string, effective map[string]string, srcNS string, conflictPolicy string) error {
	hash := core.HashData(effective)
	labels := map[string]string{core.ManagedLabel: "true"}
	source := fmt.Sprintf("%s/%s", srcNS, name)
	annotations := map[string]string{
		core.SourceAnnotation: source,
		core.HashAnnotation:   hash,
	}
	for _, ns := range planned {
		_, tgtLabels, tgtAnn, found, err := c.GetTargetConfigMap(ns, name)
		if err != nil {
			return fmt.Errorf("get target %s/%s: %w", ns, name, err)
		}
		if found {
			if tgtLabels[core.ManagedLabel] != "true" && tgtAnn[core.SourceAnnotation] != source {
				continue
			}
			if tgtAnn[core.HashAnnotation] == hash {
				continue
			}
			if conflictPolicy == core.ConflictSkip {
				continue
			}
		}
		if err := c.UpsertConfigMap(ns, name, effective, labels, annotations); err != nil {
			return fmt.Errorf("upsert %s/%s: %w", ns, name, err)
		}
	}
	return nil
}

func planTargets(all []string, strategy string, batchSize int32) []string {
	if strategy == core.StrategyImmediate {
		return append([]string(nil), all...)
	}
	if batchSize < 1 {
		batchSize = 1
	}
	if int(batchSize) >= len(all) {
		return append([]string(nil), all...)
	}
	return append([]string(nil), all[:batchSize]...)
}

// cleanupDeselected removes or detaches targets in namespaces that were previously managed
// but are no longer selected by the label selector.
func cleanupDeselected(c adapters.KubeClient, spec *core.ConfigPropagationSpec, currentlySelected []string) error {
	prune := true
	if spec.Prune != nil {
		prune = *spec.Prune
	}
	source := fmt.Sprintf("%s/%s", spec.SourceRef.Namespace, spec.SourceRef.Name)
	managed, err := c.ListManagedTargetNamespaces(source, spec.SourceRef.Name)
	if err != nil {
		return fmt.Errorf("list managed: %w", err)
	}
	// Build set of selected
	selSet := map[string]struct{}{}
	for _, ns := range currentlySelected {
		selSet[ns] = struct{}{}
	}
	for _, ns := range managed {
		if _, ok := selSet[ns]; ok {
			continue
		}
		if prune {
			if err := c.DeleteConfigMap(ns, spec.SourceRef.Name); err != nil {
				return fmt.Errorf("delete %s/%s: %w", ns, spec.SourceRef.Name, err)
			}
		} else {
			// Detach: remove managed markers
			labels := map[string]string{}
			annotations := map[string]string{}
			if err := c.UpdateConfigMapMetadata(ns, spec.SourceRef.Name, labels, annotations); err != nil {
				return fmt.Errorf("detach %s/%s: %w", ns, spec.SourceRef.Name, err)
			}
		}
	}
	return nil
}

// Finalize performs full cleanup across all managed targets for this CR.
func (r *Reconciler) Finalize(spec *core.ConfigPropagationSpec) error {
	if spec == nil {
		return fmt.Errorf("spec is nil")
	}
	core.DefaultSpec(spec)
	if err := core.ValidateSpec(spec); err != nil {
		return err
	}
	// Cleanup with empty selection set
	return cleanupDeselected(r.client, spec, []string{})
}

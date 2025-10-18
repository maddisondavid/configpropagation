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
	client  adapters.KubeClient
	queue   *core.WorkQueue[Key]
	planner *core.RolloutPlanner
}

// OnCRChange enqueues a reconcile when the CR changes.
func (r *Reconciler) OnCRChange(ns, name string) { r.queue.Add(Key{Namespace: ns, Name: name}) }

// OnSourceChange enqueues a reconcile when the source ConfigMap changes.
func (r *Reconciler) OnSourceChange(ns, name string) { r.queue.Add(Key{Namespace: ns, Name: name}) }

// OnNamespaceLabelChange enqueues a reconcile for selection changes.
func (r *Reconciler) OnNamespaceLabelChange(ns, name string) {
	r.queue.Add(Key{Namespace: ns, Name: name})
}

func NewReconciler(client adapters.KubeClient) *Reconciler {
	return &Reconciler{
		client:  client,
		queue:   core.NewWorkQueue[Key](),
		planner: core.NewRolloutPlanner(),
	}
}

// Reconcile performs one loop for the next item in the queue.
func (r *Reconciler) Reconcile(key Key, spec *core.ConfigPropagationSpec) (core.RolloutResult, error) {
	if spec == nil {
		return core.RolloutResult{}, fmt.Errorf("spec is nil")
	}
	core.DefaultSpec(spec)
	if err := core.ValidateSpec(spec); err != nil {
		return core.RolloutResult{}, err
	}
	return reconcileImpl(r.client, r.planner, key, spec)
}

// Internal implementation separated for testability and full coverage.
func reconcileImpl(client adapters.KubeClient, planner *core.RolloutPlanner, key Key, spec *core.ConfigPropagationSpec) (core.RolloutResult, error) {
	srcData, err := client.GetSourceConfigMap(spec.SourceRef.Namespace, spec.SourceRef.Name)
	if err != nil {
		return core.RolloutResult{}, fmt.Errorf("get source: %w", err)
	}
	effective := computeEffective(srcData, spec.DataKeys)
	targets, err := listTargets(client, spec.NamespaceSelector)
	if err != nil {
		return core.RolloutResult{}, fmt.Errorf("list namespaces: %w", err)
	}
	sort.Strings(targets)
	batch := int32(5)
	if spec.Strategy.BatchSize != nil {
		batch = *spec.Strategy.BatchSize
	}
	hash := core.HashData(effective)
	planned, _ := planTargets(planner, key, hash, targets, spec.Strategy.Type, batch)
	syncSummary, err := syncTargets(client, planned, spec.SourceRef.Name, effective, hash, spec.SourceRef.Namespace, spec.ConflictPolicy)
	if err != nil {
		return core.RolloutResult{}, err
	}
	completed := len(syncSummary.completed)
	if spec.Strategy.Type == core.StrategyRolling {
		completed = planner.MarkCompleted(core.NamespacedName{Namespace: key.Namespace, Name: key.Name}, hash, syncSummary.completed)
	}
	if spec.Strategy.Type == core.StrategyImmediate {
		planner.Forget(core.NamespacedName{Namespace: key.Namespace, Name: key.Name})
	}
	// Cleanup deselected namespaces per prune policy
	cleanupSummary, err := cleanupDeselected(client, spec, targets)
	if err != nil {
		return core.RolloutResult{}, err
	}
	result := core.RolloutResult{
		Planned:          planned,
		TotalTargets:     len(targets),
		CompletedCount:   completed,
		CompletedTargets: append([]string(nil), syncSummary.completed...),
		CreatedTargets:   append([]string(nil), syncSummary.created...),
		UpdatedTargets:   append([]string(nil), syncSummary.updated...),
		SkippedTargets:   append([]core.OutOfSyncItem(nil), syncSummary.skipped...),
		PrunedTargets:    append([]string(nil), cleanupSummary.pruned...),
		DetachedTargets:  append([]string(nil), cleanupSummary.detached...),
	}
	return result, nil
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

type targetSyncSummary struct {
	created   []string
	updated   []string
	skipped   []core.OutOfSyncItem
	completed []string
}

func syncTargets(c adapters.KubeClient, planned []string, name string, effective map[string]string, hash string, srcNS string, conflictPolicy string) (targetSyncSummary, error) {
	summary := targetSyncSummary{}
	labels := map[string]string{core.ManagedLabel: "true"}
	source := fmt.Sprintf("%s/%s", srcNS, name)
	annotations := map[string]string{
		core.SourceAnnotation: source,
		core.HashAnnotation:   hash,
	}
	for _, ns := range planned {
		_, tgtLabels, tgtAnn, found, err := c.GetTargetConfigMap(ns, name)
		if err != nil {
			return summary, fmt.Errorf("get target %s/%s: %w", ns, name, err)
		}
		if found {
			if tgtLabels[core.ManagedLabel] != "true" && tgtAnn[core.SourceAnnotation] != source {
				summary.skipped = append(summary.skipped, core.OutOfSyncItem{Namespace: ns, Reason: "NotManaged", Message: "target is not managed by ConfigPropagation"})
				continue
			}
			if tgtAnn[core.HashAnnotation] == hash {
				summary.completed = append(summary.completed, ns)
				continue
			}
			if conflictPolicy == core.ConflictSkip {
				summary.skipped = append(summary.skipped, core.OutOfSyncItem{Namespace: ns, Reason: "Conflict", Message: "target has diverged; skipping due to conflict policy"})
				continue
			}
		}
		if err := c.UpsertConfigMap(ns, name, effective, labels, annotations); err != nil {
			return summary, fmt.Errorf("upsert %s/%s: %w", ns, name, err)
		}
		if found {
			summary.updated = append(summary.updated, ns)
		} else {
			summary.created = append(summary.created, ns)
		}
		summary.completed = append(summary.completed, ns)
	}
	return summary, nil
}

func planTargets(planner *core.RolloutPlanner, key Key, hash string, all []string, strategy string, batchSize int32) ([]string, int) {
	id := core.NamespacedName{Namespace: key.Namespace, Name: key.Name}
	return planner.Plan(id, hash, strategy, batchSize, all)
}

// cleanupDeselected removes or detaches targets in namespaces that were previously managed
// but are no longer selected by the label selector.
type cleanupSummary struct {
	pruned   []string
	detached []string
}

func cleanupDeselected(c adapters.KubeClient, spec *core.ConfigPropagationSpec, currentlySelected []string) (cleanupSummary, error) {
	summary := cleanupSummary{}
	prune := true
	if spec.Prune != nil {
		prune = *spec.Prune
	}
	source := fmt.Sprintf("%s/%s", spec.SourceRef.Namespace, spec.SourceRef.Name)
	managed, err := c.ListManagedTargetNamespaces(source, spec.SourceRef.Name)
	if err != nil {
		return summary, fmt.Errorf("list managed: %w", err)
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
				return summary, fmt.Errorf("delete %s/%s: %w", ns, spec.SourceRef.Name, err)
			}
			summary.pruned = append(summary.pruned, ns)
		} else {
			// Detach: remove managed markers
			labels := map[string]string{}
			annotations := map[string]string{}
			if err := c.UpdateConfigMapMetadata(ns, spec.SourceRef.Name, labels, annotations); err != nil {
				return summary, fmt.Errorf("detach %s/%s: %w", ns, spec.SourceRef.Name, err)
			}
			summary.detached = append(summary.detached, ns)
		}
	}
	return summary, nil
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
	_, err := cleanupDeselected(r.client, spec, []string{})
	return err
}

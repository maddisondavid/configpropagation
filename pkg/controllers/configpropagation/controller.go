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
	backoff func() core.BackoffStrategy
}

// OnCRChange enqueues a reconcile when the CR changes.
func (r *Reconciler) OnCRChange(ns, name string) { r.queue.Add(Key{Namespace: ns, Name: name}) }

// OnSourceChange enqueues a reconcile when the source ConfigMap changes.
func (r *Reconciler) OnSourceChange(ns, name string) { r.queue.Add(Key{Namespace: ns, Name: name}) }

// OnNamespaceLabelChange enqueues a reconcile for selection changes.
func (r *Reconciler) OnNamespaceLabelChange(ns, name string) {
	r.queue.Add(Key{Namespace: ns, Name: name})
}

// NewReconciler constructs a Reconciler instance.
func NewReconciler(client adapters.KubeClient) *Reconciler {
	return &Reconciler{
		client:  client,
		queue:   core.NewWorkQueue[Key](),
		planner: core.NewRolloutPlanner(),
		backoff: core.DefaultBackoff,
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
	return reconcileImpl(r.client, r.planner, key, spec, r.backoff)
}

// Internal implementation separated for testability and full coverage.
func reconcileImpl(client adapters.KubeClient, planner *core.RolloutPlanner, key Key, spec *core.ConfigPropagationSpec, newBackoff func() core.BackoffStrategy) (core.RolloutResult, error) {
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
	planned, completedBefore := planTargets(planner, key, hash, targets, spec.Strategy.Type, batch)
	outcome := syncTargets(client, planned, spec.SourceRef.Name, effective, hash, spec.SourceRef.Namespace, spec.ConflictPolicy, newBackoff)

	result := core.RolloutResult{
		Planned:      append([]string(nil), planned...),
		TotalTargets: len(targets),
		Synced:       append([]string(nil), outcome.synced...),
		Failed:       append([]core.OutOfSyncItem(nil), outcome.failed...),
		Warnings:     append([]core.NamespaceWarning(nil), outcome.warnings...),
		Retries:      make(map[string]int, len(outcome.retries)),
	}
	for ns, attempts := range outcome.retries {
		result.Retries[ns] = attempts
	}

	nsName := core.NamespacedName{Namespace: key.Namespace, Name: key.Name}
	switch spec.Strategy.Type {
	case core.StrategyRolling:
		if len(planned) == 0 {
			result.CompletedCount = completedBefore
		} else {
			result.CompletedCount = planner.MarkCompleted(nsName, hash, outcome.synced)
		}
	default:
		planner.Forget(nsName)
		result.CompletedCount = len(result.Synced)
	}

	if err := cleanupDeselected(client, spec, targets); err != nil {
		return core.RolloutResult{}, err
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

type syncOutcome struct {
	synced   []string
	failed   []core.OutOfSyncItem
	warnings []core.NamespaceWarning
	retries  map[string]int
}

func syncTargets(c adapters.KubeClient, planned []string, name string, effective map[string]string, hash string, srcNS string, conflictPolicy string, newBackoff func() core.BackoffStrategy) syncOutcome {
	outcome := syncOutcome{
		synced:   make([]string, 0, len(planned)),
		failed:   []core.OutOfSyncItem{},
		warnings: []core.NamespaceWarning{},
		retries:  make(map[string]int, len(planned)),
	}
	labels := map[string]string{core.ManagedLabel: "true"}
	source := fmt.Sprintf("%s/%s", srcNS, name)
	annotations := map[string]string{
		core.SourceAnnotation: source,
		core.HashAnnotation:   hash,
	}
	sizeCheck := core.CheckConfigMapSize(effective)
	if sizeCheck.Block {
		msg := fmt.Sprintf("payload size %dB exceeds ConfigMap limit %dB", sizeCheck.Bytes, core.ConfigMapSizeLimitBytes)
		for _, ns := range planned {
			outcome.failed = append(outcome.failed, core.OutOfSyncItem{Namespace: ns, Reason: core.ReasonPayloadTooLarge, Message: msg})
			outcome.retries[ns] = 0
		}
		return outcome
	}
	for _, ns := range planned {
		if sizeCheck.Warn {
			outcome.warnings = append(outcome.warnings, core.NamespaceWarning{
				Namespace: ns,
				Reason:    core.WarningLargePayload,
				Message:   fmt.Sprintf("payload size %dB approaching limit %dB", sizeCheck.Bytes, core.ConfigMapSizeLimitBytes),
			})
		}
		backoff := newBackoff()
		attempts, err := backoff.Retry(func() error {
			_, tgtLabels, tgtAnn, found, err := c.GetTargetConfigMap(ns, name)
			if err != nil {
				return fmt.Errorf("get target %s/%s: %w", ns, name, err)
			}
			if found {
				if tgtLabels[core.ManagedLabel] != "true" && tgtAnn[core.SourceAnnotation] != source {
					return nil
				}
				if tgtAnn[core.HashAnnotation] == hash {
					return nil
				}
				if conflictPolicy == core.ConflictSkip {
					return nil
				}
			}
			if err := c.UpsertConfigMap(ns, name, effective, labels, annotations); err != nil {
				return fmt.Errorf("upsert %s/%s: %w", ns, name, err)
			}
			return nil
		}, func(err error) bool {
			return core.ClassifyError(err) == core.ErrorCategoryTransient
		})
		outcome.retries[ns] = attempts
		if err != nil {
			category := core.ClassifyError(err)
			reason := core.ReasonPermanentError
			switch category {
			case core.ErrorCategoryRBAC:
				reason = core.ReasonRBACDenied
			case core.ErrorCategoryTransient:
				reason = core.ReasonTransientError
			}
			outcome.failed = append(outcome.failed, core.OutOfSyncItem{Namespace: ns, Reason: reason, Message: err.Error()})
			continue
		}
		outcome.synced = append(outcome.synced, ns)
	}
	return outcome
}

func planTargets(planner *core.RolloutPlanner, key Key, hash string, all []string, strategy string, batchSize int32) ([]string, int) {
	id := core.NamespacedName{Namespace: key.Namespace, Name: key.Name}
	return planner.Plan(id, hash, strategy, batchSize, all)
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

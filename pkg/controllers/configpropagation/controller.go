package configpropagation

import (
	"fmt"
	"sort"
	"time"

	"configpropagation/pkg/adapters"
	"configpropagation/pkg/core"
)

// Key identifies a ConfigPropagation by namespace/name.
type Key struct {
	Namespace string
	Name      string
}

// namespacedName converts the key into the core NamespacedName helper type.
func (key Key) namespacedName() core.NamespacedName {
	return core.NamespacedName{Namespace: key.Namespace, Name: key.Name}
}

const (
	eventReasonConfigCreated = "ConfigCreated"
	eventReasonConfigUpdated = "ConfigUpdated"
	eventReasonConfigSkipped = "ConfigSkipped"
	eventReasonConfigPruned  = "ConfigPruned"
	eventReasonConfigError   = "ConfigError"
)

// Reconciler wires the kube client and a simple work queue.
type Reconciler struct {
	clientAdapter   adapters.KubeClient
	workQueue       *core.WorkQueue[Key]
	rolloutPlanner  *core.RolloutPlanner
	eventRecorder   adapters.EventRecorder
	metricsRecorder adapters.MetricsRecorder
}

// OnCRChange enqueues a reconcile when the CR changes.
func (reconciler *Reconciler) OnCRChange(namespace, name string) {
	reconciler.workQueue.Add(Key{Namespace: namespace, Name: name})
}

// OnSourceChange enqueues a reconcile when the source ConfigMap changes.
func (reconciler *Reconciler) OnSourceChange(namespace, name string) {
	reconciler.workQueue.Add(Key{Namespace: namespace, Name: name})
}

// OnNamespaceLabelChange enqueues a reconcile for selection changes.
func (reconciler *Reconciler) OnNamespaceLabelChange(namespace, name string) {
	reconciler.workQueue.Add(Key{Namespace: namespace, Name: name})
}

// NewReconciler builds a reconciler with sane defaults for optional dependencies.
func NewReconciler(client adapters.KubeClient, eventRecorder adapters.EventRecorder, metricsRecorder adapters.MetricsRecorder) *Reconciler {
	if eventRecorder == nil {
		eventRecorder = adapters.NewNoopEventRecorder()
	}
	if metricsRecorder == nil {
		metricsRecorder = adapters.NewNoopMetricsRecorder()
	}
	return &Reconciler{
		clientAdapter:   client,
		workQueue:       core.NewWorkQueue[Key](),
		rolloutPlanner:  core.NewRolloutPlanner(),
		eventRecorder:   eventRecorder,
		metricsRecorder: metricsRecorder,
	}
}

// Reconcile performs one loop for the next item in the queue.
func (reconciler *Reconciler) Reconcile(key Key, spec *core.ConfigPropagationSpec) (core.RolloutResult, error) {
	if spec == nil {
		return core.RolloutResult{}, fmt.Errorf("spec is nil")
	}

	core.DefaultSpec(spec)
	if err := core.ValidateSpec(spec); err != nil {
		return core.RolloutResult{}, err
	}

	start := time.Now()
	result, err := reconciler.reconcileImpl(key, spec)
	duration := time.Since(start)

	if err != nil {
		reconciler.metricsRecorder.ObserveReconcileDuration(duration)
		return core.RolloutResult{}, err
	}

	reconciler.metricsRecorder.ObserveTargets(result.TotalTargets, len(result.OutOfSync))
	reconciler.metricsRecorder.ObserveReconcileDuration(duration)

	return result, nil
}

// Internal implementation separated for testability and full coverage.
func (reconciler *Reconciler) reconcileImpl(key Key, spec *core.ConfigPropagationSpec) (core.RolloutResult, error) {
	sourceConfigData, err := reconciler.clientAdapter.GetSourceConfigMap(spec.SourceRef.Namespace, spec.SourceRef.Name)
	if err != nil {
		return core.RolloutResult{}, reconciler.recordError(key, "source_fetch", fmt.Sprintf("get source %s/%s", spec.SourceRef.Namespace, spec.SourceRef.Name), err)
	}

	effectiveData := computeEffective(sourceConfigData, spec.DataKeys)

	targetNamespaces, err := listTargets(reconciler.clientAdapter, spec.NamespaceSelector)
	if err != nil {
		return core.RolloutResult{}, reconciler.recordError(key, "namespace_list", "list namespaces", err)
	}

	sort.Strings(targetNamespaces)

	batchSize := int32(5)
	if spec.Strategy.BatchSize != nil {
		batchSize = *spec.Strategy.BatchSize
	}

	rolloutHash := core.HashData(effectiveData)

	plannedNamespaces, _ := planTargets(reconciler.rolloutPlanner, key, rolloutHash, targetNamespaces, spec.Strategy.Type, batchSize)

	syncSummary, err := reconciler.syncTargets(key, plannedNamespaces, spec.SourceRef.Name, effectiveData, rolloutHash, spec.SourceRef.Namespace, spec.ConflictPolicy)
	if err != nil {
		return core.RolloutResult{}, err
	}

	identifier := core.NamespacedName{Namespace: key.Namespace, Name: key.Name}

	outOfSyncItems := append([]core.OutOfSyncItem(nil), syncSummary.outOfSync...)
	outOfSyncSet := map[string]struct{}{}
	for _, item := range outOfSyncItems {
		outOfSyncSet[item.Namespace] = struct{}{}
	}

	completedTargetCount := 0
	switch spec.Strategy.Type {
	case core.StrategyRolling:
		completedTargetCount = reconciler.rolloutPlanner.MarkCompleted(identifier, rolloutHash, syncSummary.completed)
		completedNamespaces := reconciler.rolloutPlanner.CompletedNamespaces(identifier, rolloutHash)
		completedSet := make(map[string]struct{}, len(completedNamespaces))
		for _, namespace := range completedNamespaces {
			completedSet[namespace] = struct{}{}
		}

		for _, namespace := range targetNamespaces {
			if _, done := completedSet[namespace]; done {
				continue
			}
			if _, alreadyReported := outOfSyncSet[namespace]; alreadyReported {
				continue
			}
			outOfSyncItems = append(outOfSyncItems, core.OutOfSyncItem{
				Namespace: namespace,
				Reason:    "PendingRollout",
				Message:   "namespace awaiting rollout batch",
			})
		}
	default:
		completedTargetCount = len(syncSummary.completed)
		completedSet := make(map[string]struct{}, len(syncSummary.completed))
		for _, namespace := range syncSummary.completed {
			completedSet[namespace] = struct{}{}
		}

		for _, namespace := range targetNamespaces {
			if _, done := completedSet[namespace]; done {
				continue
			}
			if _, alreadyReported := outOfSyncSet[namespace]; alreadyReported {
				continue
			}
			outOfSyncItems = append(outOfSyncItems, core.OutOfSyncItem{
				Namespace: namespace,
				Reason:    "PendingSync",
				Message:   "namespace not synchronized",
			})
		}

		reconciler.rolloutPlanner.Forget(identifier)
	}
	// Cleanup deselected namespaces per prune policy
	if err := reconciler.cleanupDeselected(key, spec, targetNamespaces); err != nil {
		return core.RolloutResult{}, err
	}
	result := core.RolloutResult{
		Planned:        plannedNamespaces,
		TotalTargets:   len(targetNamespaces),
		CompletedCount: completedTargetCount,
		OutOfSync:      outOfSyncItems,
	}
	return result, nil
}

// nilIfEmpty normalizes empty maps to nil so Kubernetes clients omit them.
func nilIfEmpty[K comparable, V any](m map[K]V) map[K]V {
	if len(m) == 0 {
		return nil
	}
	return m
}

// computeEffective filters the source data down to the selected keys.
func computeEffective(sourceData map[string]string, selectedKeys []string) map[string]string {
	if sourceData == nil {
		sourceData = map[string]string{}
	}
	effective := map[string]string{}

	if len(selectedKeys) == 0 {
		for key, value := range sourceData {
			effective[key] = value
		}
		return effective
	}

	for _, key := range selectedKeys {
		value, exists := sourceData[key]
		if exists {
			effective[key] = value
		}
	}
	return effective
}

// listTargets returns the namespaces matching the provided selector via the adapter.
func listTargets(clientAdapter adapters.KubeClient, selector *core.LabelSelector) ([]string, error) {
	var selectorRequirements []adapters.LabelSelectorRequirement

	for _, expression := range selector.MatchExpressions {
		requirement := adapters.LabelSelectorRequirement{Key: expression.Key, Operator: expression.Operator, Values: expression.Values}
		selectorRequirements = append(selectorRequirements, requirement)
	}

	return clientAdapter.ListNamespacesBySelector(nilIfEmpty(selector.MatchLabels), selectorRequirements)
}

type syncOutcome struct {
	completed []string
	outOfSync []core.OutOfSyncItem
}

// syncTargets writes the desired ConfigMap data into each planned namespace.
func (reconciler *Reconciler) syncTargets(key Key, plannedNamespaces []string, configMapName string, effectiveData map[string]string, contentHash string, sourceNamespace string, conflictPolicy string) (syncOutcome, error) {
	outcome := syncOutcome{}
	labels := map[string]string{core.ManagedLabel: "true"}
	sourceConfigMap := fmt.Sprintf("%s/%s", sourceNamespace, configMapName)

	annotations := map[string]string{
		core.SourceAnnotation: sourceConfigMap,
		core.HashAnnotation:   contentHash,
	}

	for _, targetNamespace := range plannedNamespaces {
		_, targetLabels, targetAnnotations, targetFound, err := reconciler.clientAdapter.GetTargetConfigMap(targetNamespace, configMapName)
		if err != nil {
			return outcome, reconciler.recordError(key, "target_lookup", fmt.Sprintf("get target %s/%s", targetNamespace, configMapName), err)
		}

		managed := false
		if targetFound {
			if targetLabels != nil && targetLabels[core.ManagedLabel] == "true" {
				managed = true
			}
			if targetAnnotations != nil && targetAnnotations[core.SourceAnnotation] == sourceConfigMap {
				managed = true
			}
		}

		if targetFound && !managed && conflictPolicy == core.ConflictSkip {
			reconciler.recordSkip(key, targetNamespace, configMapName, "existing unmanaged ConfigMap (conflictPolicy=skip)")
			outcome.outOfSync = append(outcome.outOfSync, core.OutOfSyncItem{
				Namespace: targetNamespace,
				Reason:    "ConflictPolicySkip",
				Message:   "existing ConfigMap is unmanaged and conflictPolicy=skip",
			})
			continue
		}

		if targetFound && managed && targetAnnotations[core.HashAnnotation] == contentHash {
			reconciler.recordSkip(key, targetNamespace, configMapName, "already up to date")
			outcome.completed = append(outcome.completed, targetNamespace)
			continue
		}

		if err := reconciler.clientAdapter.UpsertConfigMap(targetNamespace, configMapName, effectiveData, labels, annotations); err != nil {
			return outcome, reconciler.recordError(key, "upsert", fmt.Sprintf("upsert %s/%s", targetNamespace, configMapName), err)
		}

		outcome.completed = append(outcome.completed, targetNamespace)
		if targetFound {
			reconciler.recordUpdate(key, targetNamespace, configMapName)
		} else {
			reconciler.recordCreate(key, targetNamespace, configMapName)
		}
	}
	return outcome, nil
}

// planTargets delegates to the rollout planner to determine the next batch of namespaces.
func planTargets(rolloutPlanner *core.RolloutPlanner, key Key, rolloutHash string, candidateNamespaces []string, strategy string, batchSize int32) ([]string, int) {
	id := core.NamespacedName{Namespace: key.Namespace, Name: key.Name}
	return rolloutPlanner.Plan(id, rolloutHash, strategy, batchSize, candidateNamespaces)
}

// cleanupDeselected removes or detaches targets in namespaces that were previously managed
// but are no longer selected by the label selector.
func (reconciler *Reconciler) cleanupDeselected(key Key, spec *core.ConfigPropagationSpec, currentlySelectedNamespaces []string) error {
	shouldPrune := true
	if spec.Prune != nil {
		shouldPrune = *spec.Prune
	}

	sourceIdentifier := fmt.Sprintf("%s/%s", spec.SourceRef.Namespace, spec.SourceRef.Name)

	managedNamespaces, err := reconciler.clientAdapter.ListManagedTargetNamespaces(sourceIdentifier, spec.SourceRef.Name)
	if err != nil {
		return reconciler.recordError(key, "list_managed", "list managed targets", err)
	}
	// Build set of selected
	selectedNamespaceSet := map[string]struct{}{}

	for _, namespace := range currentlySelectedNamespaces {
		selectedNamespaceSet[namespace] = struct{}{}
	}

	for _, namespace := range managedNamespaces {
		if _, stillSelected := selectedNamespaceSet[namespace]; stillSelected {
			continue
		}

		if shouldPrune {
			if err := reconciler.clientAdapter.DeleteConfigMap(namespace, spec.SourceRef.Name); err != nil {
				return reconciler.recordError(key, "prune", fmt.Sprintf("delete %s/%s", namespace, spec.SourceRef.Name), err)
			}
			reconciler.recordPrune(key, namespace, spec.SourceRef.Name)
		} else {
			// Detach: remove managed markers but preserve any other metadata.
			_, labels, annotations, found, err := reconciler.clientAdapter.GetTargetConfigMap(namespace, spec.SourceRef.Name)
			if err != nil {
				return reconciler.recordError(key, "target_lookup", fmt.Sprintf("get target %s/%s", namespace, spec.SourceRef.Name), err)
			}

			if !found {
				continue
			}

			delete(labels, core.ManagedLabel)
			delete(annotations, core.SourceAnnotation)
			delete(annotations, core.HashAnnotation)

			if err := reconciler.clientAdapter.UpdateConfigMapMetadata(namespace, spec.SourceRef.Name, labels, annotations); err != nil {
				return reconciler.recordError(key, "detach", fmt.Sprintf("detach %s/%s", namespace, spec.SourceRef.Name), err)
			}
			reconciler.recordSkip(key, namespace, spec.SourceRef.Name, "detached from management")
		}
	}
	return nil
}

// Finalize performs full cleanup across all managed targets for this CR.
func (reconciler *Reconciler) Finalize(key Key, spec *core.ConfigPropagationSpec) error {
	if spec == nil {
		return fmt.Errorf("spec is nil")
	}

	core.DefaultSpec(spec)
	if err := core.ValidateSpec(spec); err != nil {
		return err
	}
	// Cleanup with empty selection set
	return reconciler.cleanupDeselected(key, spec, []string{})
}

// recordCreate emits metrics and events for created ConfigMaps.
func (reconciler *Reconciler) recordCreate(key Key, namespace, name string) {
	reconciler.metricsRecorder.AddPropagations(adapters.MetricsActionCreate, 1)
	reconciler.eventRecorder.Normalf(key.namespacedName(), eventReasonConfigCreated, "Created ConfigMap %s/%s", namespace, name)
}

// recordUpdate emits metrics and events for updated ConfigMaps.
func (reconciler *Reconciler) recordUpdate(key Key, namespace, name string) {
	reconciler.metricsRecorder.AddPropagations(adapters.MetricsActionUpdate, 1)
	reconciler.eventRecorder.Normalf(key.namespacedName(), eventReasonConfigUpdated, "Updated ConfigMap %s/%s", namespace, name)
}

// recordSkip emits metrics and events when a target is skipped.
func (reconciler *Reconciler) recordSkip(key Key, namespace, name, reason string) {
	reconciler.metricsRecorder.AddPropagations(adapters.MetricsActionSkip, 1)
	reconciler.eventRecorder.Normalf(key.namespacedName(), eventReasonConfigSkipped, "Skipped ConfigMap %s/%s: %s", namespace, name, reason)
}

// recordPrune emits metrics and events when a target is deleted during pruning.
func (reconciler *Reconciler) recordPrune(key Key, namespace, name string) {
	reconciler.metricsRecorder.AddPropagations(adapters.MetricsActionPrune, 1)
	reconciler.eventRecorder.Normalf(key.namespacedName(), eventReasonConfigPruned, "Pruned ConfigMap %s/%s", namespace, name)
}

// recordError increments error metrics and wraps the provided error with context.
func (reconciler *Reconciler) recordError(key Key, stage, message string, err error) error {
	reconciler.metricsRecorder.IncError(stage)
	reconciler.eventRecorder.Warningf(key.namespacedName(), eventReasonConfigError, "%s: %v", message, err)
	return fmt.Errorf("%s: %w", message, err)
}

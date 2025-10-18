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
	clientAdapter  adapters.KubeClient
	workQueue      *core.WorkQueue[Key]
	rolloutPlanner *core.RolloutPlanner
}

type syncSummary struct {
	syncedNamespaces  []string
	createdNamespaces []string
	updatedNamespaces []string
	skippedOutOfSync  []core.OutOfSyncItem
}

type cleanupSummary struct {
	prunedNamespaces []string
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

func NewReconciler(client adapters.KubeClient) *Reconciler {
	return &Reconciler{
		clientAdapter:  client,
		workQueue:      core.NewWorkQueue[Key](),
		rolloutPlanner: core.NewRolloutPlanner(),
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

	return reconcileImpl(reconciler.clientAdapter, reconciler.rolloutPlanner, key, spec)
}

// Internal implementation separated for testability and full coverage.
func reconcileImpl(clientAdapter adapters.KubeClient, rolloutPlanner *core.RolloutPlanner, key Key, spec *core.ConfigPropagationSpec) (core.RolloutResult, error) {
	sourceConfigData, err := clientAdapter.GetSourceConfigMap(spec.SourceRef.Namespace, spec.SourceRef.Name)
	if err != nil {
		return core.RolloutResult{}, fmt.Errorf("get source: %w", err)
	}

	effectiveData := computeEffective(sourceConfigData, spec.DataKeys)

	targetNamespaces, err := listTargets(clientAdapter, spec.NamespaceSelector)
	if err != nil {
		return core.RolloutResult{}, fmt.Errorf("list namespaces: %w", err)
	}

	sort.Strings(targetNamespaces)

	batchSize := int32(5)
	if spec.Strategy.BatchSize != nil {
		batchSize = *spec.Strategy.BatchSize
	}

	rolloutHash := core.HashData(effectiveData)

	plannedNamespaces, completedBefore := planTargets(rolloutPlanner, key, rolloutHash, targetNamespaces, spec.Strategy.Type, batchSize)

	summary, err := syncTargets(clientAdapter, plannedNamespaces, spec.SourceRef.Name, effectiveData, rolloutHash, spec.SourceRef.Namespace, spec.ConflictPolicy)
	if err != nil {
		return core.RolloutResult{}, err
	}

	completedTargetCount := completedBefore
	if spec.Strategy.Type == core.StrategyRolling && len(plannedNamespaces) > 0 {
		completedTargetCount = rolloutPlanner.MarkCompleted(core.NamespacedName{Namespace: key.Namespace, Name: key.Name}, rolloutHash, summary.syncedNamespaces)
	}
	if spec.Strategy.Type == core.StrategyImmediate {
		rolloutPlanner.Forget(core.NamespacedName{Namespace: key.Namespace, Name: key.Name})
		completedTargetCount = len(summary.syncedNamespaces)
	}
	// Cleanup deselected namespaces per prune policy
	cleanup, err := cleanupDeselected(clientAdapter, spec, targetNamespaces)
	if err != nil {
		return core.RolloutResult{}, err
	}
	result := core.RolloutResult{
		Planned:        plannedNamespaces,
		TotalTargets:   len(targetNamespaces),
		CompletedCount: completedTargetCount,
		Counters: core.PropagationCounters{
			Created: len(summary.createdNamespaces),
			Updated: len(summary.updatedNamespaces),
			Skipped: len(summary.skippedOutOfSync),
			Pruned:  len(cleanup.prunedNamespaces),
		},
		OutOfSync: append([]core.OutOfSyncItem(nil), summary.skippedOutOfSync...),
	}
	return result, nil
}

func nilIfEmpty[K comparable, V any](m map[K]V) map[K]V {
	if len(m) == 0 {
		return nil
	}
	return m
}

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

func listTargets(clientAdapter adapters.KubeClient, selector *core.LabelSelector) ([]string, error) {
	var selectorRequirements []adapters.LabelSelectorRequirement

	for _, expression := range selector.MatchExpressions {
		requirement := adapters.LabelSelectorRequirement{Key: expression.Key, Operator: expression.Operator, Values: expression.Values}
		selectorRequirements = append(selectorRequirements, requirement)
	}

	return clientAdapter.ListNamespacesBySelector(nilIfEmpty(selector.MatchLabels), selectorRequirements)
}

func syncTargets(clientAdapter adapters.KubeClient, plannedNamespaces []string, configMapName string, effectiveData map[string]string, contentHash string, sourceNamespace string, conflictPolicy string) (syncSummary, error) {
	labels := map[string]string{core.ManagedLabel: "true"}
	sourceConfigMap := fmt.Sprintf("%s/%s", sourceNamespace, configMapName)

	annotations := map[string]string{
		core.SourceAnnotation: sourceConfigMap,
		core.HashAnnotation:   contentHash,
	}

	summary := syncSummary{}

	for _, targetNamespace := range plannedNamespaces {
		_, targetLabels, targetAnnotations, targetFound, err := clientAdapter.GetTargetConfigMap(targetNamespace, configMapName)
		if err != nil {
			return syncSummary{}, fmt.Errorf("get target %s/%s: %w", targetNamespace, configMapName, err)
		}

		if targetFound {
			if targetLabels[core.ManagedLabel] != "true" && targetAnnotations[core.SourceAnnotation] != sourceConfigMap {
				continue
			}

			if targetAnnotations[core.HashAnnotation] == contentHash {
				summary.syncedNamespaces = append(summary.syncedNamespaces, targetNamespace)
				continue
			}

			if conflictPolicy == core.ConflictSkip {
				summary.skippedOutOfSync = append(summary.skippedOutOfSync, core.OutOfSyncItem{
					Namespace: targetNamespace,
					Reason:    "ConflictSkipped",
					Message:   fmt.Sprintf("existing ConfigMap differs and conflictPolicy=%s", core.ConflictSkip),
				})
				continue
			}
		}

		if err := clientAdapter.UpsertConfigMap(targetNamespace, configMapName, effectiveData, labels, annotations); err != nil {
			return syncSummary{}, fmt.Errorf("upsert %s/%s: %w", targetNamespace, configMapName, err)
		}

		summary.syncedNamespaces = append(summary.syncedNamespaces, targetNamespace)

		if targetFound {
			summary.updatedNamespaces = append(summary.updatedNamespaces, targetNamespace)
			continue
		}

		summary.createdNamespaces = append(summary.createdNamespaces, targetNamespace)
	}
	return summary, nil
}

func planTargets(rolloutPlanner *core.RolloutPlanner, key Key, rolloutHash string, candidateNamespaces []string, strategy string, batchSize int32) ([]string, int) {
	id := core.NamespacedName{Namespace: key.Namespace, Name: key.Name}
	return rolloutPlanner.Plan(id, rolloutHash, strategy, batchSize, candidateNamespaces)
}

// cleanupDeselected removes or detaches targets in namespaces that were previously managed
// but are no longer selected by the label selector.
func cleanupDeselected(clientAdapter adapters.KubeClient, spec *core.ConfigPropagationSpec, currentlySelectedNamespaces []string) (cleanupSummary, error) {
	shouldPrune := true
	if spec.Prune != nil {
		shouldPrune = *spec.Prune
	}

	sourceIdentifier := fmt.Sprintf("%s/%s", spec.SourceRef.Namespace, spec.SourceRef.Name)

	summary := cleanupSummary{}

	managedNamespaces, err := clientAdapter.ListManagedTargetNamespaces(sourceIdentifier, spec.SourceRef.Name)
	if err != nil {
		return cleanupSummary{}, fmt.Errorf("list managed: %w", err)
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
			if err := clientAdapter.DeleteConfigMap(namespace, spec.SourceRef.Name); err != nil {
				return cleanupSummary{}, fmt.Errorf("delete %s/%s: %w", namespace, spec.SourceRef.Name, err)
			}

			summary.prunedNamespaces = append(summary.prunedNamespaces, namespace)
		} else {
			// Detach: remove managed markers
			labels := map[string]string{}
			annotations := map[string]string{}

			if err := clientAdapter.UpdateConfigMapMetadata(namespace, spec.SourceRef.Name, labels, annotations); err != nil {
				return cleanupSummary{}, fmt.Errorf("detach %s/%s: %w", namespace, spec.SourceRef.Name, err)
			}
		}
	}
	return summary, nil
}

// Finalize performs full cleanup across all managed targets for this CR.
func (reconciler *Reconciler) Finalize(spec *core.ConfigPropagationSpec) error {
	if spec == nil {
		return fmt.Errorf("spec is nil")
	}

	core.DefaultSpec(spec)
	if err := core.ValidateSpec(spec); err != nil {
		return err
	}
	// Cleanup with empty selection set
	_, err := cleanupDeselected(reconciler.clientAdapter, spec, []string{})
	return err
}

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
func (r *Reconciler) OnCRChange(namespace, name string) {
	r.queue.Add(Key{Namespace: namespace, Name: name})
}

// OnSourceChange enqueues a reconcile when the source ConfigMap changes.
func (r *Reconciler) OnSourceChange(namespace, name string) {
	r.queue.Add(Key{Namespace: namespace, Name: name})
}

// OnNamespaceLabelChange enqueues a reconcile for selection changes.
func (r *Reconciler) OnNamespaceLabelChange(namespace, name string) {
	r.queue.Add(Key{Namespace: namespace, Name: name})
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
	sourceConfigData, err := client.GetSourceConfigMap(spec.SourceRef.Namespace, spec.SourceRef.Name)
	if err != nil {
		return core.RolloutResult{}, fmt.Errorf("get source: %w", err)
	}

	effectiveData := computeEffective(sourceConfigData, spec.DataKeys)

	targetNamespaces, err := listTargets(client, spec.NamespaceSelector)
	if err != nil {
		return core.RolloutResult{}, fmt.Errorf("list namespaces: %w", err)
	}
	sort.Strings(targetNamespaces)

	batchSize := int32(5)
	if spec.Strategy.BatchSize != nil {
		batchSize = *spec.Strategy.BatchSize
	}

	rolloutHash := core.HashData(effectiveData)

	plannedNamespaces, completedBefore := planTargets(planner, key, rolloutHash, targetNamespaces, spec.Strategy.Type, batchSize)

	err = syncTargets(client, plannedNamespaces, spec.SourceRef.Name, effectiveData, rolloutHash, spec.SourceRef.Namespace, spec.ConflictPolicy)
	if err != nil {
		return core.RolloutResult{}, err
	}

	completedTargetCount := completedBefore
	if spec.Strategy.Type == core.StrategyRolling && len(plannedNamespaces) > 0 {
		completedTargetCount = planner.MarkCompleted(core.NamespacedName{Namespace: key.Namespace, Name: key.Name}, rolloutHash, plannedNamespaces)
	}
	if spec.Strategy.Type == core.StrategyImmediate {
		planner.Forget(core.NamespacedName{Namespace: key.Namespace, Name: key.Name})
		completedTargetCount = len(targetNamespaces)
	}
	// Cleanup deselected namespaces per prune policy
	if err := cleanupDeselected(client, spec, targetNamespaces); err != nil {
		return core.RolloutResult{}, err
	}
	result := core.RolloutResult{
		Planned:        plannedNamespaces,
		TotalTargets:   len(targetNamespaces),
		CompletedCount: completedTargetCount,
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

func listTargets(client adapters.KubeClient, selector *core.LabelSelector) ([]string, error) {
	var selectorRequirements []adapters.LabelSelectorRequirement

	for _, expression := range selector.MatchExpressions {
		requirement := adapters.LabelSelectorRequirement{Key: expression.Key, Operator: expression.Operator, Values: expression.Values}
		selectorRequirements = append(selectorRequirements, requirement)
	}

	return client.ListNamespacesBySelector(nilIfEmpty(selector.MatchLabels), selectorRequirements)
}

func syncTargets(client adapters.KubeClient, plannedNamespaces []string, configMapName string, effectiveData map[string]string, contentHash string, sourceNamespace string, conflictPolicy string) error {
	labels := map[string]string{core.ManagedLabel: "true"}
	sourceConfigMap := fmt.Sprintf("%s/%s", sourceNamespace, configMapName)

	annotations := map[string]string{
		core.SourceAnnotation: sourceConfigMap,
		core.HashAnnotation:   contentHash,
	}

	for _, targetNamespace := range plannedNamespaces {
		_, targetLabels, targetAnnotations, targetFound, err := client.GetTargetConfigMap(targetNamespace, configMapName)
		if err != nil {
			return fmt.Errorf("get target %s/%s: %w", targetNamespace, configMapName, err)
		}

		if targetFound {
			if targetLabels[core.ManagedLabel] != "true" && targetAnnotations[core.SourceAnnotation] != sourceConfigMap {
				continue
			}

			if targetAnnotations[core.HashAnnotation] == contentHash {
				continue
			}

			if conflictPolicy == core.ConflictSkip {
				continue
			}
		}

		if err := client.UpsertConfigMap(targetNamespace, configMapName, effectiveData, labels, annotations); err != nil {
			return fmt.Errorf("upsert %s/%s: %w", targetNamespace, configMapName, err)
		}
	}
	return nil
}

func planTargets(planner *core.RolloutPlanner, key Key, rolloutHash string, candidateNamespaces []string, strategy string, batchSize int32) ([]string, int) {
	id := core.NamespacedName{Namespace: key.Namespace, Name: key.Name}
	return planner.Plan(id, rolloutHash, strategy, batchSize, candidateNamespaces)
}

// cleanupDeselected removes or detaches targets in namespaces that were previously managed
// but are no longer selected by the label selector.
func cleanupDeselected(client adapters.KubeClient, spec *core.ConfigPropagationSpec, currentlySelectedNamespaces []string) error {
	shouldPrune := true
	if spec.Prune != nil {
		shouldPrune = *spec.Prune
	}

	sourceIdentifier := fmt.Sprintf("%s/%s", spec.SourceRef.Namespace, spec.SourceRef.Name)

	managedNamespaces, err := client.ListManagedTargetNamespaces(sourceIdentifier, spec.SourceRef.Name)
	if err != nil {
		return fmt.Errorf("list managed: %w", err)
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
			if err := client.DeleteConfigMap(namespace, spec.SourceRef.Name); err != nil {
				return fmt.Errorf("delete %s/%s: %w", namespace, spec.SourceRef.Name, err)
			}
		} else {
			// Detach: remove managed markers
			labels := map[string]string{}
			annotations := map[string]string{}

			if err := client.UpdateConfigMapMetadata(namespace, spec.SourceRef.Name, labels, annotations); err != nil {
				return fmt.Errorf("detach %s/%s: %w", namespace, spec.SourceRef.Name, err)
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

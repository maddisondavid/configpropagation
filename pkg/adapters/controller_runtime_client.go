package adapters

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"configpropagation/pkg/core"
)

type controllerRuntimeClient struct {
	client client.Client
}

// NewControllerRuntimeClient returns a KubeClient backed by a controller-runtime client.Client.
func NewControllerRuntimeClient(kubeClient client.Client) KubeClient {
	return &controllerRuntimeClient{client: kubeClient}
}

func (clientAdapter *controllerRuntimeClient) GetSourceConfigMap(namespace, name string) (map[string]string, error) {
	requestContext := context.Background()

	var configMap corev1.ConfigMap

	if err := clientAdapter.client.Get(requestContext, types.NamespacedName{Namespace: namespace, Name: name}, &configMap); err != nil {
		return nil, err
	}

	return copyStringMap(configMap.Data), nil
}

func (clientAdapter *controllerRuntimeClient) ListNamespacesBySelector(matchLabels map[string]string, selectorRequirements []LabelSelectorRequirement) ([]string, error) {
	requestContext := context.Background()

	selector := labels.NewSelector()

	for labelKey, labelValue := range matchLabels {
		requirement, err := labels.NewRequirement(labelKey, selection.Equals, []string{labelValue})
		if err != nil {
			return nil, err
		}

		selector = selector.Add(*requirement)
	}

	for _, requirement := range selectorRequirements {
		selectionOperator, err := toSelectionOperator(requirement.Operator)
		if err != nil {
			return nil, err
		}

		typedRequirement, err := labels.NewRequirement(requirement.Key, selectionOperator, requirement.Values)
		if err != nil {
			return nil, err
		}

		selector = selector.Add(*typedRequirement)
	}

	var namespaces corev1.NamespaceList

	if err := clientAdapter.client.List(requestContext, &namespaces); err != nil {
		return nil, err
	}

	var namespaceNames []string

	for _, namespaceItem := range namespaces.Items {
		if selector.Empty() || selector.Matches(labels.Set(namespaceItem.Labels)) {
			namespaceNames = append(namespaceNames, namespaceItem.Name)
		}
	}

	return namespaceNames, nil
}

func (clientAdapter *controllerRuntimeClient) UpsertConfigMap(namespace, name string, data map[string]string, labelsMap, annotations map[string]string) error {
	requestContext := context.Background()

	var existingConfigMap corev1.ConfigMap

	err := clientAdapter.client.Get(requestContext, types.NamespacedName{Namespace: namespace, Name: name}, &existingConfigMap)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		configMap := corev1.ConfigMap{}
		configMap.Namespace = namespace
		configMap.Name = name
		configMap.Data = copyStringMap(data)
		configMap.Labels = copyStringMap(labelsMap)
		configMap.Annotations = copyStringMap(annotations)

		return clientAdapter.client.Create(requestContext, &configMap)
	}

	existingConfigMap.Data = copyStringMap(data)

	if existingConfigMap.Labels == nil {
		existingConfigMap.Labels = map[string]string{}
	}

	if existingConfigMap.Annotations == nil {
		existingConfigMap.Annotations = map[string]string{}
	}

	for key := range existingConfigMap.Labels {
		delete(existingConfigMap.Labels, key)
	}

	for key := range existingConfigMap.Annotations {
		delete(existingConfigMap.Annotations, key)
	}

	for key, value := range labelsMap {
		existingConfigMap.Labels[key] = value
	}

	for key, value := range annotations {
		existingConfigMap.Annotations[key] = value
	}

	return clientAdapter.client.Update(requestContext, &existingConfigMap)
}

func (clientAdapter *controllerRuntimeClient) GetTargetConfigMap(namespace, name string) (map[string]string, map[string]string, map[string]string, bool, error) {
	requestContext := context.Background()

	var configMap corev1.ConfigMap

	if err := clientAdapter.client.Get(requestContext, types.NamespacedName{Namespace: namespace, Name: name}, &configMap); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil, nil, false, nil
		}

		return nil, nil, nil, false, err
	}

	return copyStringMap(configMap.Data), copyStringMap(configMap.Labels), copyStringMap(configMap.Annotations), true, nil
}

func (clientAdapter *controllerRuntimeClient) ListManagedTargetNamespaces(source string, name string) ([]string, error) {
	requestContext := context.Background()

	var configMapList corev1.ConfigMapList

	if err := clientAdapter.client.List(requestContext, &configMapList, client.MatchingLabels{core.ManagedLabel: "true"}); err != nil {
		return nil, err
	}

	var namespaces []string

	for _, configMap := range configMapList.Items {
		if configMap.Name != name {
			continue
		}

		if configMap.Annotations[core.SourceAnnotation] != source {
			continue
		}

		namespaces = append(namespaces, configMap.Namespace)
	}

	return namespaces, nil
}

func (clientAdapter *controllerRuntimeClient) DeleteConfigMap(namespace, name string) error {
	requestContext := context.Background()

	configMap := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}

	return client.IgnoreNotFound(clientAdapter.client.Delete(requestContext, &configMap))
}

func (clientAdapter *controllerRuntimeClient) UpdateConfigMapMetadata(namespace, name string, labelsMap, annotations map[string]string) error {
	requestContext := context.Background()

	var configMap corev1.ConfigMap

	if err := clientAdapter.client.Get(requestContext, types.NamespacedName{Namespace: namespace, Name: name}, &configMap); err != nil {
		return err
	}

	configMap.Labels = copyStringMap(labelsMap)
	configMap.Annotations = copyStringMap(annotations)

	return clientAdapter.client.Update(requestContext, &configMap)
}

func toSelectionOperator(operator string) (selection.Operator, error) {
	switch operator {
	case "In":
		return selection.In, nil
	case "NotIn":
		return selection.NotIn, nil
	case "Exists":
		return selection.Exists, nil
	case "DoesNotExist":
		return selection.DoesNotExist, nil
	default:
		return selection.Operator(""), fmt.Errorf("unsupported operator %s", operator)
	}
}

func copyStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}

	copied := make(map[string]string, len(source))

	for key, value := range source {
		copied[key] = value
	}

	return copied
}

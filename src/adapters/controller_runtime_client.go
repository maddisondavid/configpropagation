package adapters

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"codex/src/core"
)

type controllerRuntimeClient struct {
	client client.Client
}

// NewControllerRuntimeClient returns a KubeClient backed by a controller-runtime client.Client.
func NewControllerRuntimeClient(c client.Client) KubeClient {
	return &controllerRuntimeClient{client: c}
}

func (c *controllerRuntimeClient) GetSourceConfigMap(namespace, name string) (map[string]string, error) {
	ctx := context.Background()
	var cm corev1.ConfigMap
	if err := c.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &cm); err != nil {
		return nil, err
	}
	return copyStringMap(cm.Data), nil
}

func (c *controllerRuntimeClient) ListNamespacesBySelector(matchLabels map[string]string, exprs []LabelSelectorRequirement) ([]string, error) {
	ctx := context.Background()
	selector := labels.NewSelector()
	for k, v := range matchLabels {
		req, err := labels.NewRequirement(k, selection.Equals, []string{v})
		if err != nil {
			return nil, err
		}
		selector = selector.Add(*req)
	}
	for _, expr := range exprs {
		op, err := toSelectionOperator(expr.Operator)
		if err != nil {
			return nil, err
		}
		req, err := labels.NewRequirement(expr.Key, op, expr.Values)
		if err != nil {
			return nil, err
		}
		selector = selector.Add(*req)
	}
	var namespaces corev1.NamespaceList
	if err := c.client.List(ctx, &namespaces); err != nil {
		return nil, err
	}
	var result []string
	for _, ns := range namespaces.Items {
		if selector.Empty() || selector.Matches(labels.Set(ns.Labels)) {
			result = append(result, ns.Name)
		}
	}
	return result, nil
}

func (c *controllerRuntimeClient) UpsertConfigMap(namespace, name string, data map[string]string, labelsMap, annotations map[string]string) error {
	ctx := context.Background()
	var existing corev1.ConfigMap
	err := c.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		cm := corev1.ConfigMap{}
		cm.Namespace = namespace
		cm.Name = name
		cm.Data = copyStringMap(data)
		cm.Labels = copyStringMap(labelsMap)
		cm.Annotations = copyStringMap(annotations)
		return c.client.Create(ctx, &cm)
	}
	existing.Data = copyStringMap(data)
	if existing.Labels == nil {
		existing.Labels = map[string]string{}
	}
	if existing.Annotations == nil {
		existing.Annotations = map[string]string{}
	}
	for k := range existing.Labels {
		delete(existing.Labels, k)
	}
	for k := range existing.Annotations {
		delete(existing.Annotations, k)
	}
	for k, v := range labelsMap {
		existing.Labels[k] = v
	}
	for k, v := range annotations {
		existing.Annotations[k] = v
	}
	return c.client.Update(ctx, &existing)
}

func (c *controllerRuntimeClient) GetTargetConfigMap(namespace, name string) (map[string]string, map[string]string, map[string]string, bool, error) {
	ctx := context.Background()
	var cm corev1.ConfigMap
	if err := c.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &cm); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil, nil, false, nil
		}
		return nil, nil, nil, false, err
	}
	return copyStringMap(cm.Data), copyStringMap(cm.Labels), copyStringMap(cm.Annotations), true, nil
}

func (c *controllerRuntimeClient) ListManagedTargetNamespaces(source string, name string) ([]string, error) {
	ctx := context.Background()
	var cms corev1.ConfigMapList
	if err := c.client.List(ctx, &cms, client.MatchingLabels{core.ManagedLabel: "true"}); err != nil {
		return nil, err
	}
	var namespaces []string
	for _, cm := range cms.Items {
		if cm.Name != name {
			continue
		}
		if cm.Annotations[core.SourceAnnotation] != source {
			continue
		}
		namespaces = append(namespaces, cm.Namespace)
	}
	return namespaces, nil
}

func (c *controllerRuntimeClient) DeleteConfigMap(namespace, name string) error {
	ctx := context.Background()
	cm := corev1.ConfigMap{Namespace: namespace, Name: name}
	return client.IgnoreNotFound(c.client.Delete(ctx, &cm))
}

func (c *controllerRuntimeClient) UpdateConfigMapMetadata(namespace, name string, labelsMap, annotations map[string]string) error {
	ctx := context.Background()
	var cm corev1.ConfigMap
	if err := c.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &cm); err != nil {
		return err
	}
	cm.Labels = copyStringMap(labelsMap)
	cm.Annotations = copyStringMap(annotations)
	return c.client.Update(ctx, &cm)
}

func toSelectionOperator(op string) (selection.Operator, error) {
	switch op {
	case "In":
		return selection.In, nil
	case "NotIn":
		return selection.NotIn, nil
	case "Exists":
		return selection.Exists, nil
	case "DoesNotExist":
		return selection.DoesNotExist, nil
	default:
		return selection.Operator(""), fmt.Errorf("unsupported operator %s", op)
	}
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

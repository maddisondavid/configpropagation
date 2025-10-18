package configpropagation

import (
	"context"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"configpropagation/pkg/adapters"
	configv1alpha1 "configpropagation/pkg/api/v1alpha1"
	"configpropagation/pkg/core"
)

type stubKubeClient struct {
	managedNamespaces []string
	deleteCalls       [][2]string
}

func (s *stubKubeClient) GetSourceConfigMap(namespace, name string) (map[string]string, error) {
	return map[string]string{"key": "value"}, nil
}

func (s *stubKubeClient) ListNamespacesBySelector(map[string]string, []adapters.LabelSelectorRequirement) ([]string, error) {
	return nil, nil
}

func (s *stubKubeClient) UpsertConfigMap(string, string, map[string]string, map[string]string, map[string]string) error {
	return nil
}

func (s *stubKubeClient) GetTargetConfigMap(string, string) (map[string]string, map[string]string, map[string]string, bool, error) {
	return nil, nil, nil, false, nil
}

func (s *stubKubeClient) ListManagedTargetNamespaces(string, string) ([]string, error) {
	return append([]string(nil), s.managedNamespaces...), nil
}

func (s *stubKubeClient) DeleteConfigMap(namespace, name string) error {
	s.deleteCalls = append(s.deleteCalls, [2]string{namespace, name})
	return nil
}

func (s *stubKubeClient) UpdateConfigMapMetadata(string, string, map[string]string, map[string]string) error {
	return nil
}

func buildFakeClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}

	if err := configv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add configpropagation scheme: %v", err)
	}

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithStatusSubresource(objects...).Build()
}

func TestControllerAddsFinalizer(t *testing.T) {
	kubeStub := &stubKubeClient{}
	reconciler := NewReconciler(kubeStub, nil, nil)

	configPropagation := &configv1alpha1.ConfigPropagation{
		ObjectMeta: metav1.ObjectMeta{Name: "cp", Namespace: "default"},
		Spec: core.ConfigPropagationSpec{
			SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
			NamespaceSelector: &core.LabelSelector{},
		},
	}

	controller := &ConfigPropagationController{Client: buildFakeClient(t, configPropagation), reconciler: reconciler}

	if _, err := controller.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(configPropagation)}); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	var updated configv1alpha1.ConfigPropagation
	if err := controller.Get(context.Background(), client.ObjectKeyFromObject(configPropagation), &updated); err != nil {
		t.Fatalf("get updated object: %v", err)
	}

	if len(updated.Finalizers) == 0 || updated.Finalizers[0] != core.Finalizer {
		t.Fatalf("expected finalizer to be added, got %+v", updated.Finalizers)
	}
}

func TestControllerFinalizeRemovesFinalizer(t *testing.T) {
	kubeStub := &stubKubeClient{managedNamespaces: []string{"ns1"}}
	reconciler := NewReconciler(kubeStub, nil, nil)

	deletionTime := metav1.NewTime(time.Now())
	configPropagation := &configv1alpha1.ConfigPropagation{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "cp",
			Namespace:         "default",
			Finalizers:        []string{core.Finalizer},
			DeletionTimestamp: &deletionTime,
		},
		Spec: core.ConfigPropagationSpec{
			SourceRef:         core.ObjectRef{Namespace: "src", Name: "cfg"},
			NamespaceSelector: &core.LabelSelector{},
		},
	}

	controller := &ConfigPropagationController{Client: buildFakeClient(t, configPropagation), reconciler: reconciler}

	if _, err := controller.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(configPropagation)}); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	var updated configv1alpha1.ConfigPropagation
	if err := controller.Get(context.Background(), client.ObjectKeyFromObject(configPropagation), &updated); err != nil {
		if !apierrors.IsNotFound(err) {
			t.Fatalf("get updated object: %v", err)
		}
	} else if len(updated.Finalizers) != 0 {
		t.Fatalf("expected finalizer to be removed, got %+v", updated.Finalizers)
	}

	if len(kubeStub.deleteCalls) != 1 || kubeStub.deleteCalls[0] != [2]string{"ns1", "cfg"} {
		t.Fatalf("expected delete for managed target, got %+v", kubeStub.deleteCalls)
	}
}

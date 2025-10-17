package configpropagation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"configpropagation/pkg/adapters"
	configv1alpha1 "configpropagation/pkg/api/v1alpha1"
	"configpropagation/pkg/core"
)

// ConfigPropagationController reconciles ConfigPropagation resources with a controller-runtime manager.
type ConfigPropagationController struct {
	client.Client
	logger     logr.Logger
	reconciler *Reconciler
}

var _ reconcile.Reconciler = &ConfigPropagationController{}

// NewController constructs a ConfigPropagationController wired with the manager's client.
func NewController(manager ctrl.Manager) *ConfigPropagationController {
	kubeClient := adapters.NewControllerRuntimeClient(manager.GetClient())

	return &ConfigPropagationController{
		Client:     manager.GetClient(),
		logger:     ctrl.Log.WithName("controllers").WithName("ConfigPropagation"),
		reconciler: NewReconciler(kubeClient),
	}
}

// Reconcile runs the core reconciliation logic for a ConfigPropagation instance.
func (controller *ConfigPropagationController) Reconcile(requestContext context.Context, reconcileRequest ctrl.Request) (ctrl.Result, error) {
	requestLogger := controller.logger.WithValues("configpropagation", reconcileRequest.NamespacedName)

	var configPropagation configv1alpha1.ConfigPropagation

	if err := controller.Get(requestContext, reconcileRequest.NamespacedName, &configPropagation); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if configPropagation.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(&configPropagation, core.Finalizer) {
			controllerutil.AddFinalizer(&configPropagation, core.Finalizer)

			if err := controller.Update(requestContext, &configPropagation); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(&configPropagation, core.Finalizer) {
			if err := controller.reconciler.Finalize(&configPropagation.Spec); err != nil {
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(&configPropagation, core.Finalizer)

			if err := controller.Update(requestContext, &configPropagation); err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	result, err := controller.reconciler.Reconcile(Key{Namespace: reconcileRequest.Namespace, Name: reconcileRequest.Name}, &configPropagation.Spec)
	if err != nil {
		requestLogger.Error(err, "reconciliation failed")

		return ctrl.Result{}, err
	}

	statusPatch := client.MergeFrom(configPropagation.DeepCopy())

	configPropagation.ApplyRolloutStatus(result)

	if err := controller.Status().Patch(requestContext, &configPropagation, statusPatch); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller with the provided manager.
func SetupWithManager(manager ctrl.Manager) error {
	reconciler := NewController(manager)
	return ctrl.NewControllerManagedBy(manager).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		For(&configv1alpha1.ConfigPropagation{}).
		Complete(reconciler)
}

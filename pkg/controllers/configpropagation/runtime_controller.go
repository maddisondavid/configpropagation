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
	log        logr.Logger
	reconciler *Reconciler
}

var _ reconcile.Reconciler = &ConfigPropagationController{}

// NewController constructs a ConfigPropagationController wired with the manager's client.
func NewController(mgr ctrl.Manager) *ConfigPropagationController {
	kube := adapters.NewControllerRuntimeClient(mgr.GetClient())
	return &ConfigPropagationController{
		Client:     mgr.GetClient(),
		log:        ctrl.Log.WithName("controllers").WithName("ConfigPropagation"),
		reconciler: NewReconciler(kube),
	}
}

// Reconcile runs the core reconciliation logic for a ConfigPropagation instance.
func (c *ConfigPropagationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := c.log.WithValues("configpropagation", req.NamespacedName)
	var cp configv1alpha1.ConfigPropagation
	if err := c.Get(ctx, req.NamespacedName, &cp); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if cp.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(&cp, core.Finalizer) {
			controllerutil.AddFinalizer(&cp, core.Finalizer)
			if err := c.Update(ctx, &cp); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(&cp, core.Finalizer) {
			if err := c.reconciler.Finalize(&cp.Spec); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(&cp, core.Finalizer)
			if err := c.Update(ctx, &cp); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	planned, err := c.reconciler.Reconcile(&cp.Spec)
	if err != nil {
		log.Error(err, "reconciliation failed")
		return ctrl.Result{}, err
	}

	statusPatch := client.MergeFrom(cp.DeepCopy())
	cp.ApplySuccessStatus(len(planned))
	if err := c.Status().Patch(ctx, &cp, statusPatch); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller with the provided manager.
func SetupWithManager(mgr ctrl.Manager) error {
	reconciler := NewController(mgr)
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		For(&configv1alpha1.ConfigPropagation{}).
		Complete(reconciler)
}

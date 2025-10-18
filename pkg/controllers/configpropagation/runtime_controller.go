package configpropagation

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"configpropagation/pkg/adapters"
	configv1alpha1 "configpropagation/pkg/api/v1alpha1"
	"configpropagation/pkg/core"
	observabilitymetrics "configpropagation/pkg/observability/metrics"
)

// ConfigPropagationController reconciles ConfigPropagation resources with a controller-runtime manager.
type ConfigPropagationController struct {
	client.Client
	logger        logr.Logger
	reconciler    *Reconciler
	eventRecorder record.EventRecorder
}

var _ reconcile.Reconciler = &ConfigPropagationController{}

// NewController constructs a ConfigPropagationController wired with the manager's client.
func NewController(manager ctrl.Manager) *ConfigPropagationController {
	kubeClient := adapters.NewControllerRuntimeClient(manager.GetClient())

	return &ConfigPropagationController{
		Client:        manager.GetClient(),
		logger:        ctrl.Log.WithName("controllers").WithName("ConfigPropagation"),
		reconciler:    NewReconciler(kubeClient),
		eventRecorder: manager.GetEventRecorderFor("configpropagation-controller"),
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

	start := time.Now()
	result, err := controller.reconciler.Reconcile(Key{Namespace: reconcileRequest.Namespace, Name: reconcileRequest.Name}, &configPropagation.Spec)
	duration := time.Since(start)

	observabilitymetrics.RecordReconcile(result, duration, err)
	if err != nil {
		requestLogger.Error(err, "reconciliation failed")

		if controller.eventRecorder != nil {
			controller.eventRecorder.Eventf(&configPropagation, corev1.EventTypeWarning, "ReconcileError", "reconciliation failed: %v", err)
		}

		return ctrl.Result{}, err
	}

	emitEvents(controller.eventRecorder, &configPropagation, result)

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

func emitEvents(recorder record.EventRecorder, cp *configv1alpha1.ConfigPropagation, result core.RolloutResult) {
	if recorder == nil || cp == nil {
		return
	}

	if result.Counters.Created > 0 {
		recorder.Eventf(cp, corev1.EventTypeNormal, "ConfigMapsCreated", "created %d ConfigMaps in target namespaces", result.Counters.Created)
	}
	if result.Counters.Updated > 0 {
		recorder.Eventf(cp, corev1.EventTypeNormal, "ConfigMapsUpdated", "updated %d ConfigMaps in target namespaces", result.Counters.Updated)
	}
	if result.Counters.Skipped > 0 {
		recorder.Eventf(cp, corev1.EventTypeWarning, "TargetsSkipped", "skipped %d target namespaces due to conflicts", result.Counters.Skipped)
	}
	if result.Counters.Pruned > 0 {
		recorder.Eventf(cp, corev1.EventTypeNormal, "ConfigMapsPruned", "pruned %d ConfigMaps that were no longer selected", result.Counters.Pruned)
	}

	if result.TotalTargets == 0 {
		recorder.Event(cp, corev1.EventTypeNormal, "NoTargets", "no target namespaces matched the selector")
		return
	}

	if result.CompletedCount == result.TotalTargets && result.Counters.Skipped == 0 {
		recorder.Eventf(cp, corev1.EventTypeNormal, "PropagationComplete", "propagated to %d/%d namespaces", result.CompletedCount, result.TotalTargets)
	}
}

package controller

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GatewayClassController should implement the Reconciler interface
var _ reconcile.Reconciler = &GatewayClassController{}

const (
	GatewayClassControllerFinalizer = "strrl.dev/cloudflare-tunnel-gatewayclass-controller-controlled"
	ControllerName                  = "strrl.dev/cloudflare-tunnel-gatewayclass-controller"
)

type GatewayClassController struct {
	logger     logr.Logger
	kubeClient client.Client
}

func NewGatewayClassController(logger logr.Logger, kubeClient client.Client) *GatewayClassController {
	return &GatewayClassController{logger: logger, kubeClient: kubeClient}
}

func (g *GatewayClassController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	origin := gatewayv1.GatewayClass{}
	err := g.kubeClient.Get(ctx, request.NamespacedName, &origin)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, errors.Wrapf(err, "fetch gatewayclass %s", request.NamespacedName)
	}

	controlled, err := g.isControlledByThisController(ctx, origin)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, errors.Wrapf(err, "check if gatewayclass %s is controlled by this controller", request.NamespacedName)
	}

	if !controlled {
		g.logger.V(1).Info("gatewayclass is NOT controlled by this controller",
			"gatewayclass", request.NamespacedName,
			"controlled-controller-name", ControllerName,
		)
		return reconcile.Result{
			Requeue: false,
		}, nil
	}

	g.logger.V(1).Info("gatewayclass is controlled by this controller",
		"gatewayclass", request.NamespacedName,
		"controlled-controller-name", ControllerName,
	)

	// Update GatewayClass status
	if err := g.updateGatewayClassStatus(ctx, &origin); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "update status for gatewayclass %s", request.NamespacedName)
	}

	g.logger.Info("update cloudflare tunnel config", "triggered-by", request.NamespacedName)

	err = g.attachFinalizer(ctx, *(origin.DeepCopy()))
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "attach finalizer to gatewayclass %s", request.NamespacedName)
	}

	// Add your custom logic here

	if origin.DeletionTimestamp != nil {
		err = g.cleanFinalizer(ctx, origin)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "clean finalizer from gatewayclass %s", request.NamespacedName)
		}
	}

	g.logger.V(3).Info("reconcile completed", "triggered-by", request.NamespacedName)
	return reconcile.Result{}, nil
}

func (g *GatewayClassController) isControlledByThisController(ctx context.Context, target gatewayv1.GatewayClass) (bool, error) {
	if string(target.Spec.ControllerName) == ControllerName {
		return true, nil
	}
	return false, nil
}

func (g *GatewayClassController) attachFinalizer(ctx context.Context, gatewayClass gatewayv1.GatewayClass) error {
	if stringSliceContains(gatewayClass.Finalizers, GatewayClassControllerFinalizer) {
		return nil
	}
	gatewayClass.Finalizers = append(gatewayClass.Finalizers, GatewayClassControllerFinalizer)
	err := g.kubeClient.Update(ctx, &gatewayClass)
	if err != nil {
		return errors.Wrapf(err, "attach finalizer for %s", gatewayClass.Name)
	}
	return nil
}

func (g *GatewayClassController) cleanFinalizer(ctx context.Context, gatewayClass gatewayv1.GatewayClass) error {
	if !stringSliceContains(gatewayClass.Finalizers, GatewayClassControllerFinalizer) {
		return nil
	}
	gatewayClass.Finalizers = removeStringFromSlice(gatewayClass.Finalizers, GatewayClassControllerFinalizer)
	err := g.kubeClient.Update(ctx, &gatewayClass)
	if err != nil {
		return errors.Wrapf(err, "clean finalizer for %s", gatewayClass.Name)
	}
	return nil
}

// Update the updateGatewayClassStatus method
func (g *GatewayClassController) updateGatewayClassStatus(ctx context.Context, gatewayClass *gatewayv1.GatewayClass) error {
	newCondition := metav1.Condition{
		Type:               string(gatewayv1.GatewayClassConditionStatusAccepted),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: gatewayClass.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             string(gatewayv1.GatewayClassReasonAccepted),
		Message:            "GatewayClass has been accepted by the controller",
	}

	meta.SetStatusCondition(&gatewayClass.Status.Conditions, newCondition)

	return g.kubeClient.Status().Update(ctx, gatewayClass)
}

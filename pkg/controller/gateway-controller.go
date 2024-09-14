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

var _ reconcile.Reconciler = &GatewayController{}

type GatewayController struct {
	logger     logr.Logger
	kubeClient client.Client
}

func NewGatewayController(logger logr.Logger, kubeClient client.Client) *GatewayController {
	return &GatewayController{logger: logger, kubeClient: kubeClient}
}

func (g *GatewayController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	gateway := gatewayv1.Gateway{}
	err := g.kubeClient.Get(ctx, request.NamespacedName, &gateway)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, errors.Wrapf(err, "fetch gateway %s", request.NamespacedName)
	}

	// Check if the Gateway is associated with our GatewayClass
	isOurs, err := g.isOurGatewayClass(ctx, gateway)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to check if Gateway is associated with our GatewayClass")
	}
	if !isOurs {
		g.logger.V(1).Info("Gateway is not associated with our GatewayClass", "gateway", request.NamespacedName)
		return reconcile.Result{}, nil
	}

	g.logger.V(1).Info("Processing Gateway associated with our GatewayClass", "gateway", request.NamespacedName)

	// Add your custom logic here for handling Gateway resources

	// Update Gateway status
	if err := g.updateGatewayStatus(ctx, &gateway); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "update status for gateway %s", request.NamespacedName)
	}

	g.logger.V(3).Info("reconcile completed", "triggered-by", request.NamespacedName)
	return reconcile.Result{}, nil
}

func (g *GatewayController) updateGatewayStatus(ctx context.Context, gateway *gatewayv1.Gateway) error {
	// Update Gateway conditions
	meta.SetStatusCondition(&gateway.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayConditionProgrammed),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: gateway.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             string(gatewayv1.GatewayReasonProgrammed),
		Message:            "Gateway has been programmed",
	})
	meta.SetStatusCondition(&gateway.Status.Conditions, metav1.Condition{
		Type:               string(gatewayv1.GatewayConditionAccepted),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: gateway.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             string(gatewayv1.GatewayReasonAccepted),
		Message:            "Gateway has been programmed",
	})

	// Update existing listener statuses
	var updatedListeners []gatewayv1.ListenerStatus
	for _, listener := range gateway.Spec.Listeners {
		listenerStatus := gatewayv1.ListenerStatus{
			Name:           listener.Name,
			SupportedKinds: []gatewayv1.RouteGroupKind{},
		}

		allKindsSupported := true
		for _, kind := range listener.AllowedRoutes.Kinds {
			if kind.Kind == "HTTPRoute" {
				listenerStatus.SupportedKinds = append(listenerStatus.SupportedKinds, gatewayv1.RouteGroupKind{
					Group: (*gatewayv1.Group)(&gatewayv1.GroupVersion.Group),
					Kind:  "HTTPRoute",
				})
			} else {
				allKindsSupported = false
			}
		}

		// Set ListenerConditionResolvedRefs based on supported kinds
		if allKindsSupported {
			meta.SetStatusCondition(&listenerStatus.Conditions, metav1.Condition{
				Type:               string(gatewayv1.ListenerConditionResolvedRefs),
				Status:             metav1.ConditionTrue,
				ObservedGeneration: gateway.Generation,
				LastTransitionTime: metav1.Now(),
				Reason:             string(gatewayv1.ListenerReasonResolvedRefs),
				Message:            "All route kinds are supported",
			})
		} else {
			meta.SetStatusCondition(&listenerStatus.Conditions, metav1.Condition{
				Type:               string(gatewayv1.ListenerConditionResolvedRefs),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: gateway.Generation,
				LastTransitionTime: metav1.Now(),
				Reason:             string(gatewayv1.ListenerReasonInvalidRouteKinds),
				Message:            "Some route kinds are not supported",
			})
		}

		updatedListeners = append(updatedListeners, listenerStatus)
	}

	// Update the Gateway status with the filtered listeners
	gateway.Status.Listeners = updatedListeners
	// Update the Gateway status
	return g.kubeClient.Status().Update(ctx, gateway)
}

func (g *GatewayController) isOurGatewayClass(ctx context.Context, gateway gatewayv1.Gateway) (bool, error) {
	gatewayClass := gatewayv1.GatewayClass{}
	err := g.kubeClient.Get(ctx, client.ObjectKey{Name: string(gateway.Spec.GatewayClassName)}, &gatewayClass)
	if err != nil {
		if apierrors.IsNotFound(err) {
			g.logger.V(1).Info("GatewayClass not found", "gatewayClassName", gateway.Spec.GatewayClassName)
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to get GatewayClass %s", gateway.Spec.GatewayClassName)
	}

	return string(gatewayClass.Spec.ControllerName) == ControllerName, nil
}

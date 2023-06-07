package controller

import (
	"context"
	cloudflarecontroller "github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/cloudflare-controller"
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// IngressController should implement the Reconciler interface
var _ reconcile.Reconciler = &IngressController{}

const WellKnownIngressAnnotation = "kubernetes.io/ingress.class"

type IngressController struct {
	logger              logr.Logger
	kubeClient          client.Client
	ingressClassName    string
	controllerClassName string
	tunnelClient        *cloudflarecontroller.TunnelClient
}

func NewIngressController(logger logr.Logger, kubeClient client.Client, ingressClassName string, controllerClassName string, tunnelClient *cloudflarecontroller.TunnelClient) *IngressController {
	return &IngressController{logger: logger, kubeClient: kubeClient, ingressClassName: ingressClassName, controllerClassName: controllerClassName, tunnelClient: tunnelClient}
}

func (i *IngressController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	origin := networkingv1.Ingress{}
	err := i.kubeClient.Get(ctx, request.NamespacedName, &origin)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, errors.Wrapf(err, "fetch ingress %s", request.NamespacedName)
	}

	controlled, err := i.isControlledByThisController(ctx, origin)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, errors.Wrapf(err, "check if ingress %s is controlled by this controller", request.NamespacedName)
	}

	if !controlled {
		i.logger.V(1).Info("ingress is NOT controlled by this controller",
			"ingress", request.NamespacedName,
			"controlled-ingress-class", i.ingressClassName,
			"controlled-controller-class", i.controllerClassName,
		)
		return reconcile.Result{
			Requeue: false,
		}, nil
	}

	i.logger.V(1).Info("ingress is controlled by this controller",
		"ingress", request.NamespacedName,
		"controlled-ingress-class", i.ingressClassName,
		"controlled-controller-class", i.controllerClassName,
	)

	// TODO: trigger the regeneration of the ingress
	ingresses, err := i.listControlledIngresses(ctx)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "list controlled ingresses")
	}
	var allExposures []exposure.Exposure
	for _, ingress := range ingresses {
		exposures, err := i.fromIngressToExposure(ctx, ingress)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "extract exposures from ingress %s", ingress.Name)
		}
		allExposures = append(allExposures, exposures...)
	}

	err = i.tunnelClient.PutExposures(ctx, allExposures)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "put exposures")
	}

	return reconcile.Result{}, nil
}

func (i *IngressController) isControlledByThisController(ctx context.Context, target networkingv1.Ingress) (bool, error) {
	if i.ingressClassName == target.GetAnnotations()[WellKnownIngressAnnotation] {
		return true, nil
	}

	if target.Spec.IngressClassName == nil {
		return false, nil
	}

	controlledIngressClasses, err := i.listControlledIngressClasses(ctx)
	if err != nil {
		return false, errors.Wrapf(err, "fetch controlled ingress classes with controller name %s", i.controllerClassName)
	}

	var controlledIngressClassNames []string
	for _, controlledIngressClass := range controlledIngressClasses {
		controlledIngressClassNames = append(controlledIngressClassNames, controlledIngressClass.Name)
	}

	if stringSliceContains(controlledIngressClassNames, *target.Spec.IngressClassName) {
		return true, nil
	}

	return false, nil
}

func (i *IngressController) listControlledIngressClasses(ctx context.Context) ([]networkingv1.IngressClass, error) {
	list := networkingv1.IngressClassList{}
	err := i.kubeClient.List(ctx, &list)
	if err != nil {
		return nil, errors.Wrap(err, "list ingress classes")
	}
	return list.Items, nil
}

func stringSliceContains(slice []string, element string) bool {
	for _, sliceElement := range slice {
		if sliceElement == element {
			return true
		}
	}
	return false
}

func (i *IngressController) listControlledIngresses(ctx context.Context) ([]networkingv1.Ingress, error) {
	controlledIngressClasses, err := i.listControlledIngressClasses(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "fetch controlled ingress classes with controller name %s", i.controllerClassName)
	}

	var controlledIngressClassNames []string
	for _, controlledIngressClass := range controlledIngressClasses {
		controlledIngressClassNames = append(controlledIngressClassNames, controlledIngressClass.Name)
	}

	var result []networkingv1.Ingress
	list := networkingv1.IngressList{}
	err = i.kubeClient.List(ctx, &list)
	if err != nil {
		return nil, errors.Wrap(err, "list ingresses")
	}

	for _, ingress := range list.Items {
		func() {
			if i.ingressClassName == ingress.GetAnnotations()[WellKnownIngressAnnotation] {
				result = append(result, ingress)
				return
			}

			if ingress.Spec.IngressClassName == nil {
				return
			}

			if stringSliceContains(controlledIngressClassNames, *ingress.Spec.IngressClassName) {
				result = append(result, ingress)
				return
			}
		}()
	}

	return result, nil
}

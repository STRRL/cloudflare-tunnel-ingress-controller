package controller

import (
	cloudflarecontroller "github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/cloudflare-controller"
	"github.com/go-logr/logr"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type IngressControllerOptions struct {
	IngressClassName    string
	ControllerClassName string
	CFTunnelClient      *cloudflarecontroller.TunnelClient
}

func RegisterIngressController(logger logr.Logger, mgr manager.Manager, options IngressControllerOptions) error {
	controller := NewIngressController(logger.WithName("ingress-controller"), mgr.GetClient(), options.IngressClassName, options.ControllerClassName, options.CFTunnelClient)
	err := builder.
		ControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Complete(controller)

	if err != nil {
		logger.WithName("register-controller").Error(err, "could not register ingress controller")
		return err
	}

	return nil
}

func RegisterGatewayClassController(logger logr.Logger, mgr manager.Manager) error {
	controller := NewGatewayClassController(logger.WithName("gatewayclass-controller"), mgr.GetClient())
	err := builder.
		ControllerManagedBy(mgr).
		For(&gatewayv1.GatewayClass{}).
		Complete(controller)

	if err != nil {
		logger.WithName("register-controller").Error(err, "could not register gatewayclass controller")
		return err
	}

	return nil
}

func RegisterGatewayController(logger logr.Logger, mgr manager.Manager) error {
	controller := NewGatewayController(logger.WithName("gateway-controller"), mgr.GetClient())
	err := builder.
		ControllerManagedBy(mgr).
		For(&gatewayv1.Gateway{}).
		Complete(controller)

	if err != nil {
		logger.WithName("register-controller").Error(err, "could not register gateway controller")
		return err
	}

	return nil
}

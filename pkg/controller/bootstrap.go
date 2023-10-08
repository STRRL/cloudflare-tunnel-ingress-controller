package controller

import (
	"github.com/go-logr/logr"
	cloudflarecontroller "github.com/oliverbaehler/cloudflare-tunnel-ingress-controller/pkg/cloudflare-controller"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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

	if err != nil {
		logger.WithName("register-controller").Error(err, "could not register ingress class controller")
		return err
	}

	return nil
}

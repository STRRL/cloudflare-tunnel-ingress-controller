package controller

import (
	"github.com/go-logr/logr"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func RegisterIngressController(logger logr.Logger, mgr manager.Manager) error {
	logger = logger.WithName("register-controller")

	err := builder.
		ControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Complete(&IngressController{})

	if err != nil {
		logger.Error(err, "could not register ingress controller")
		return err
	}

	err = builder.
		ControllerManagedBy(mgr).          // Create the ControllerManagedBy
		For(&networkingv1.IngressClass{}). // ReplicaSet is the Application API
		Complete(&IngressClassController{})

	if err != nil {
		logger.Error(err, "could not register ingress class controller")
		return err
	}

	return nil
}

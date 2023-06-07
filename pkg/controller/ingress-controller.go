package controller

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// IngressController should implement the Reconciler interface
var _ reconcile.Reconciler = &IngressController{}

type IngressController struct {
}

func (i *IngressController) Reconcile(context.Context, reconcile.Request) (reconcile.Result, error) {
	panic("unimplemented")
}

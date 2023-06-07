package controller

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// IngressClassController should implement the Reconciler interface
var _ reconcile.Reconciler = &IngressClassController{}

type IngressClassController struct {
}

func (i *IngressClassController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	//TODO implement me
	panic("implement me")
}

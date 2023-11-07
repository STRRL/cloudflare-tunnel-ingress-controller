package controller

import (
	"context"

	"github.com/pkg/errors"
	networkingv1 "k8s.io/api/networking/v1"
)

const IngressControllerFinalizer = "strrl.dev/cloudflare-tunnel-ingress-controller-controlled"

func (i *IngressController) hasFinalizer(ctx context.Context, ingress networkingv1.Ingress) bool {
	return stringSliceContains(ingress.Finalizers, IngressControllerFinalizer)
}

func (i *IngressController) attachFinalizer(ctx context.Context, ingress networkingv1.Ingress) error {
	if stringSliceContains(ingress.Finalizers, IngressControllerFinalizer) {
		return nil
	}
	ingress.Finalizers = append(ingress.Finalizers, IngressControllerFinalizer)
	err := i.kubeClient.Update(ctx, &ingress)
	if err != nil {
		return errors.Wrapf(err, "attach finalizer for %s/%s", ingress.Namespace, ingress.Name)
	}
	return nil
}

func (i *IngressController) cleanFinalizer(ctx context.Context, ingress networkingv1.Ingress) error {
	if !stringSliceContains(ingress.Finalizers, IngressControllerFinalizer) {
		return nil
	}
	ingress.Finalizers = removeStringFromSlice(ingress.Finalizers, IngressControllerFinalizer)
	err := i.kubeClient.Update(ctx, &ingress)
	if err != nil {
		return errors.Wrapf(err, "clean finalizer for %s/%s", ingress.Namespace, ingress.Name)
	}
	return nil
}

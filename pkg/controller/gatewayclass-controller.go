package controller

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Keys of the GatewayClass parameters ConfigMap.
const (
	// GatewayClassBaseDomainKey holds the domain below which every Gateway
	// gets its address, normally the Cloudflare zone itself.
	GatewayClassBaseDomainKey = "baseDomain"
	// GatewayClassLabelSuffixKey is optional, it is appended to the first
	// label of every Gateway address. It keeps the addresses of several
	// installations apart in one zone.
	GatewayClassLabelSuffixKey = "labelSuffix"
)

// gatewayAddressing is how the Gateways of a class get their addresses.
type gatewayAddressing struct {
	baseDomain  string
	labelSuffix string
}

// GatewayClassController accepts GatewayClasses of this controller whose
// parametersRef points at a valid parameters ConfigMap.
type GatewayClassController struct {
	logger         logr.Logger
	kubeClient     client.Client
	controllerName string
	namespace      string
}

var _ reconcile.Reconciler = &GatewayClassController{}

func (c *GatewayClassController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	class := &gatewayv1.GatewayClass{}
	err := c.kubeClient.Get(ctx, request.NamespacedName, class)
	if apierrors.IsNotFound(err) {
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "get gatewayclass %s", request.Name)
	}
	if string(class.Spec.ControllerName) != c.controllerName {
		return reconcile.Result{}, nil
	}

	_, reason, err := gatewayClassAddressing(ctx, c.kubeClient, class, c.namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	updated := class.DeepCopy()
	if reason == "" {
		setCondition(&updated.Status.Conditions, string(gatewayv1.GatewayClassConditionStatusAccepted), metav1.ConditionTrue, string(gatewayv1.GatewayClassReasonAccepted), "gatewayclass is accepted", class.Generation)
	} else {
		setCondition(&updated.Status.Conditions, string(gatewayv1.GatewayClassConditionStatusAccepted), metav1.ConditionFalse, string(gatewayv1.GatewayClassReasonInvalidParameters), reason, class.Generation)
	}
	if reflect.DeepEqual(updated.Status.Conditions, class.Status.Conditions) {
		return reconcile.Result{}, nil
	}
	if err := c.kubeClient.Status().Update(ctx, updated); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "update gatewayclass %s status", class.Name)
	}
	c.logger.Info("gatewayclass status updated", "gatewayclass", class.Name, "accepted", reason == "", "reason", reason)
	return reconcile.Result{}, nil
}

// gatewayClassAddressing reads the parameters ConfigMap of a GatewayClass. A non empty reason means the parameters are invalid.
// The ConfigMap must live in the controller namespace, the only namespace
// whose ConfigMaps the controller reads.
func gatewayClassAddressing(ctx context.Context, kubeClient client.Client, class *gatewayv1.GatewayClass, namespace string) (gatewayAddressing, string, error) {
	ref := class.Spec.ParametersRef
	if ref == nil {
		return gatewayAddressing{}, "parametersRef is required and must point at a ConfigMap with key " + GatewayClassBaseDomainKey, nil
	}
	if ref.Group != "" || ref.Kind != "ConfigMap" {
		return gatewayAddressing{}, fmt.Sprintf("parametersRef must point at a core ConfigMap, got %s/%s", ref.Group, ref.Kind), nil
	}
	if ref.Namespace == nil || string(*ref.Namespace) != namespace {
		return gatewayAddressing{}, fmt.Sprintf("parametersRef must point at a ConfigMap in namespace %s", namespace), nil
	}
	configMap := &corev1.ConfigMap{}
	err := kubeClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ref.Name}, configMap)
	if apierrors.IsNotFound(err) {
		return gatewayAddressing{}, fmt.Sprintf("parameters ConfigMap %s/%s not found", namespace, ref.Name), nil
	}
	if err != nil {
		return gatewayAddressing{}, "", errors.Wrapf(err, "get parameters configmap %s/%s", namespace, ref.Name)
	}
	baseDomain := configMap.Data[GatewayClassBaseDomainKey]
	if baseDomain == "" {
		return gatewayAddressing{}, fmt.Sprintf("parameters ConfigMap %s/%s has no %s", namespace, ref.Name, GatewayClassBaseDomainKey), nil
	}
	return gatewayAddressing{baseDomain: baseDomain, labelSuffix: configMap.Data[GatewayClassLabelSuffixKey]}, "", nil
}

package controller

import (
	"context"
	"crypto/tls"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// secretGetter reads a Secret, it returns a NotFound error when missing.
type secretGetter func(ctx context.Context, key types.NamespacedName) (*corev1.Secret, error)

// buildGatewayState validates a Gateway and its listeners. Routes are
// attached later.
func buildGatewayState(ctx context.Context, gateway *gatewayv1.Gateway, addressing gatewayAddressing, snapshot *clusterSnapshot, getSecret secretGetter) (*gatewayState, error) {
	state := &gatewayState{
		gateway: gateway,
		address: gatewayAddressLabel(gateway.Namespace, gateway.Name, addressing.labelSuffix) + "." + addressing.baseDomain,
	}

	acceptedListeners := 0
	for _, listener := range gateway.Spec.Listeners {
		item, err := validateListener(ctx, gateway, listener, snapshot, getSecret)
		if err != nil {
			return nil, err
		}
		if item.accepted {
			acceptedListeners++
		}
		state.listeners = append(state.listeners, item)
	}

	switch {
	case gateway.Spec.Infrastructure != nil && gateway.Spec.Infrastructure.ParametersRef != nil:
		state.reason = gatewayv1.GatewayReasonInvalidParameters
		state.message = "spec.infrastructure.parametersRef is not supported"
	case acceptedListeners == 0:
		state.reason = gatewayv1.GatewayReasonListenersNotValid
		state.message = "no listener is valid"
	case acceptedListeners < len(gateway.Spec.Listeners):
		state.accepted = true
		state.reason = gatewayv1.GatewayReasonListenersNotValid
		state.message = "some listeners are not valid"
	default:
		state.accepted = true
		state.reason = gatewayv1.GatewayReasonAccepted
		state.message = "gateway is accepted"
	}
	return state, nil
}

func validateListener(ctx context.Context, gateway *gatewayv1.Gateway, listener gatewayv1.Listener, snapshot *clusterSnapshot, getSecret secretGetter) (*listenerState, error) {
	item := &listenerState{listener: listener}

	if listener.Protocol != gatewayv1.HTTPProtocolType && listener.Protocol != gatewayv1.HTTPSProtocolType {
		return item, nil
	}
	if listener.TLS != nil && listener.TLS.Mode != nil && *listener.TLS.Mode == gatewayv1.TLSModePassthrough {
		// the Cloudflare edge always terminates TLS
		return item, nil
	}
	item.accepted = true

	httpRouteKind := gatewayv1.RouteGroupKind{Group: new(gatewayv1.Group(gatewayGroup)), Kind: kindHTTPRte}
	if listener.AllowedRoutes == nil || len(listener.AllowedRoutes.Kinds) == 0 {
		item.supportedKinds = append(item.supportedKinds, httpRouteKind)
	} else {
		for _, kind := range listener.AllowedRoutes.Kinds {
			if (kind.Group == nil || string(*kind.Group) == gatewayGroup) && kind.Kind == kindHTTPRte {
				item.supportedKinds = append(item.supportedKinds, httpRouteKind)
			} else {
				item.refsReason = gatewayv1.ListenerReasonInvalidRouteKinds
				item.refsMessage = fmt.Sprintf("route kind %s is not supported", kind.Kind)
			}
		}
	}
	if item.refsReason != "" || listener.Protocol != gatewayv1.HTTPSProtocolType || listener.TLS == nil {
		return item, nil
	}

	// certificates are served by the Cloudflare edge, the references are
	// still validated so a broken configuration is reported
	for _, ref := range listener.TLS.CertificateRefs {
		reason, message, err := validateCertificateRef(ctx, gateway, ref, snapshot, getSecret)
		if err != nil {
			return nil, err
		}
		if reason != "" {
			item.refsReason = reason
			item.refsMessage = message
			break
		}
	}
	return item, nil
}

func validateCertificateRef(ctx context.Context, gateway *gatewayv1.Gateway, ref gatewayv1.SecretObjectReference, snapshot *clusterSnapshot, getSecret secretGetter) (gatewayv1.ListenerConditionReason, string, error) {
	group := ""
	if ref.Group != nil {
		group = string(*ref.Group)
	}
	kind := kindSecret
	if ref.Kind != nil {
		kind = string(*ref.Kind)
	}
	if group != "" || kind != kindSecret {
		return gatewayv1.ListenerReasonInvalidCertificateRef, fmt.Sprintf("certificateRef %s/%s is not a Secret", group, kind), nil
	}
	namespace := gateway.Namespace
	if ref.Namespace != nil {
		namespace = string(*ref.Namespace)
	}
	if namespace != gateway.Namespace && !referenceGrantAllows(snapshot.referenceGrants, gatewayGroup, kindGateway, gateway.Namespace, "", kindSecret, namespace, string(ref.Name)) {
		return gatewayv1.ListenerReasonRefNotPermitted, fmt.Sprintf("certificateRef to secret %s/%s is not permitted by any ReferenceGrant", namespace, ref.Name), nil
	}
	secret, err := getSecret(ctx, types.NamespacedName{Namespace: namespace, Name: string(ref.Name)})
	if apierrors.IsNotFound(err) {
		return gatewayv1.ListenerReasonInvalidCertificateRef, fmt.Sprintf("secret %s/%s not found", namespace, ref.Name), nil
	}
	if err != nil {
		return "", "", errors.Wrapf(err, "get secret %s/%s", namespace, ref.Name)
	}
	if _, err := tls.X509KeyPair(secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey]); err != nil {
		return gatewayv1.ListenerReasonInvalidCertificateRef, fmt.Sprintf("secret %s/%s has no valid certificate: %s", namespace, ref.Name, err.Error()), nil
	}
	return "", "", nil
}

// gatewayStatusFor builds the new status of a Gateway from its state. The
// existing status is the starting point, so condition transition times stay
// stable while nothing changes.
// syncProblem, when not empty, is added to the Programmed message: the
// Gateway keeps its last good state while a Cloudflare sync is retried.
func gatewayStatusFor(state *gatewayState, programmed bool, syncProblem string, existing gatewayv1.GatewayStatus) gatewayv1.GatewayStatus {
	generation := state.gateway.Generation
	status := gatewayv1.GatewayStatus{Conditions: slicesCloneConditions(existing.Conditions)}

	acceptedStatus := metav1.ConditionFalse
	if state.accepted {
		acceptedStatus = metav1.ConditionTrue
	}
	setCondition(&status.Conditions, string(gatewayv1.GatewayConditionAccepted), acceptedStatus, string(state.reason), state.message, generation)

	switch {
	case !state.accepted:
		setCondition(&status.Conditions, string(gatewayv1.GatewayConditionProgrammed), metav1.ConditionFalse, string(gatewayv1.GatewayReasonInvalid), "gateway is not accepted", generation)
	case programmed:
		message := "proxy is ready and the address is published in Cloudflare"
		if syncProblem != "" {
			message += "; " + syncProblem
		}
		setCondition(&status.Conditions, string(gatewayv1.GatewayConditionProgrammed), metav1.ConditionTrue, string(gatewayv1.GatewayReasonProgrammed), message, generation)
		status.Addresses = []gatewayv1.GatewayStatusAddress{{
			Type:  new(gatewayv1.HostnameAddressType),
			Value: state.address,
		}}
	default:
		message := "waiting for the proxy and the Cloudflare DNS record"
		if syncProblem != "" {
			message += "; " + syncProblem
		}
		setCondition(&status.Conditions, string(gatewayv1.GatewayConditionProgrammed), metav1.ConditionFalse, string(gatewayv1.GatewayReasonPending), message, generation)
	}

	for _, item := range state.listeners {
		var conditions []metav1.Condition
		for _, old := range existing.Listeners {
			if old.Name == item.listener.Name {
				conditions = slicesCloneConditions(old.Conditions)
			}
		}

		if item.accepted {
			setCondition(&conditions, string(gatewayv1.ListenerConditionAccepted), metav1.ConditionTrue, string(gatewayv1.ListenerReasonAccepted), "listener is accepted", generation)
		} else {
			setCondition(&conditions, string(gatewayv1.ListenerConditionAccepted), metav1.ConditionFalse, string(gatewayv1.ListenerReasonUnsupportedProtocol), fmt.Sprintf("protocol %s is not supported", item.listener.Protocol), generation)
		}

		if item.refsReason == "" {
			setCondition(&conditions, string(gatewayv1.ListenerConditionResolvedRefs), metav1.ConditionTrue, string(gatewayv1.ListenerReasonResolvedRefs), "all references resolved", generation)
		} else {
			setCondition(&conditions, string(gatewayv1.ListenerConditionResolvedRefs), metav1.ConditionFalse, string(item.refsReason), item.refsMessage, generation)
		}

		switch {
		case !item.accepted || item.refsReason != "":
			setCondition(&conditions, string(gatewayv1.ListenerConditionProgrammed), metav1.ConditionFalse, string(gatewayv1.ListenerReasonInvalid), "listener is not valid", generation)
		case programmed:
			setCondition(&conditions, string(gatewayv1.ListenerConditionProgrammed), metav1.ConditionTrue, string(gatewayv1.ListenerReasonProgrammed), "listener is programmed", generation)
		default:
			setCondition(&conditions, string(gatewayv1.ListenerConditionProgrammed), metav1.ConditionFalse, string(gatewayv1.ListenerReasonPending), "waiting for the proxy and the Cloudflare DNS record", generation)
		}
		setCondition(&conditions, string(gatewayv1.ListenerConditionConflicted), metav1.ConditionFalse, string(gatewayv1.ListenerReasonNoConflicts), "no conflicts", generation)

		status.Listeners = append(status.Listeners, gatewayv1.ListenerStatus{
			Name:           item.listener.Name,
			SupportedKinds: item.supportedKinds,
			AttachedRoutes: item.attachedRoutes,
			Conditions:     conditions,
		})
	}
	return status
}

// routeParentStatusesFor builds the parent statuses of a route: entries of
// other controllers stay as they are, ours are replaced.
func routeParentStatusesFor(route *gatewayv1.HTTPRoute, controllerName string, results []parentResult, refsReason gatewayv1.RouteConditionReason, refsMessage string) []gatewayv1.RouteParentStatus {
	parents := []gatewayv1.RouteParentStatus{}
	for _, parent := range route.Status.Parents {
		if string(parent.ControllerName) != controllerName {
			parents = append(parents, parent)
		}
	}
	for _, result := range results {
		var conditions []metav1.Condition
		for _, old := range route.Status.Parents {
			if string(old.ControllerName) == controllerName && reflect.DeepEqual(old.ParentRef, result.ref) {
				conditions = slicesCloneConditions(old.Conditions)
			}
		}
		acceptedStatus := metav1.ConditionFalse
		if result.accepted {
			acceptedStatus = metav1.ConditionTrue
		}
		setCondition(&conditions, string(gatewayv1.RouteConditionAccepted), acceptedStatus, string(result.reason), result.message, route.Generation)
		if refsReason == "" {
			setCondition(&conditions, string(gatewayv1.RouteConditionResolvedRefs), metav1.ConditionTrue, string(gatewayv1.RouteReasonResolvedRefs), "all references resolved", route.Generation)
		} else {
			setCondition(&conditions, string(gatewayv1.RouteConditionResolvedRefs), metav1.ConditionFalse, string(refsReason), refsMessage, route.Generation)
		}
		parents = append(parents, gatewayv1.RouteParentStatus{
			ParentRef:      result.ref,
			ControllerName: gatewayv1.GatewayController(controllerName),
			Conditions:     conditions,
		})
	}
	return parents
}

func setCondition(conditions *[]metav1.Condition, conditionType string, status metav1.ConditionStatus, reason string, message string, generation int64) {
	meta.SetStatusCondition(conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
	})
}

func slicesCloneConditions(conditions []metav1.Condition) []metav1.Condition {
	if conditions == nil {
		return nil
	}
	result := make([]metav1.Condition, len(conditions))
	copy(result, conditions)
	return result
}

// updateGatewayStatus writes the status when it changed, retrying on
// conflicts with a fresh copy.
func updateGatewayStatus(ctx context.Context, kubeClient client.Client, reader client.Reader, gateway *gatewayv1.Gateway, build func(existing gatewayv1.GatewayStatus) gatewayv1.GatewayStatus) error {
	current := gateway.DeepCopy()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		desired := build(current.Status)
		if reflect.DeepEqual(desired, current.Status) {
			return nil
		}
		updated := current.DeepCopy()
		updated.Status = desired
		err := kubeClient.Status().Update(ctx, updated)
		if apierrors.IsConflict(err) {
			if getErr := reader.Get(ctx, client.ObjectKeyFromObject(current), current); getErr != nil {
				return getErr
			}
		}
		return err
	})
}

// updateRouteStatus writes the route parents when they changed, retrying on
// conflicts with a fresh copy.
func updateRouteStatus(ctx context.Context, kubeClient client.Client, reader client.Reader, route *gatewayv1.HTTPRoute, build func(current *gatewayv1.HTTPRoute) []gatewayv1.RouteParentStatus) error {
	current := route.DeepCopy()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		desired := build(current)
		if reflect.DeepEqual(desired, current.Status.Parents) || (len(desired) == 0 && len(current.Status.Parents) == 0) {
			return nil
		}
		updated := current.DeepCopy()
		updated.Status.Parents = desired
		err := kubeClient.Status().Update(ctx, updated)
		if apierrors.IsConflict(err) {
			if getErr := reader.Get(ctx, client.ObjectKeyFromObject(current), current); getErr != nil {
				return getErr
			}
		}
		return err
	})
}

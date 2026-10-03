package controller

import (
	"context"
	"slices"
	"time"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// gatewayReconcileKey is the only key the Gateway reconciler ever sees. Every
// watched change recomputes the whole Gateway API state: route status depends
// on several Gateways and Gateway status depends on several routes, so one
// serial loop avoids races, and bursts of events collapse into one run.
var gatewayReconcileKey = reconcile.Request{NamespacedName: types.NamespacedName{Name: "gateway-api"}}

// gatewayPendingRequeue is how often a Gateway that waits for its proxy or
// DNS record is checked again.
const gatewayPendingRequeue = 2 * time.Second

// gatewayResyncPeriod catches changes the controller does not watch, like
// Secrets outside the controller namespace.
const gatewayResyncPeriod = 30 * time.Second

// GatewayReconciler programs Gateways and HTTPRoutes of this controller.
type GatewayReconciler struct {
	logger         logr.Logger
	kubeClient     client.Client
	apiReader      client.Reader
	controllerName string
	namespace      string
	clusterDomain  string
	proxyConfig    func(ctx context.Context) (GatewayProxyConfig, error)
	tunnelSync     *TunnelSync
}

var _ reconcile.Reconciler = &GatewayReconciler{}

// GatewayControllerOptions configures the Gateway API support.
type GatewayControllerOptions struct {
	ControllerName string
	Namespace      string
	ClusterDomain  string
	// ProxyConfig returns the proxy Deployment settings, it is called on
	// every reconcile so the owner reference can be resolved lazily.
	ProxyConfig func(ctx context.Context) (GatewayProxyConfig, error)
	TunnelSync  *TunnelSync
}

// RegisterGatewayControllers adds the GatewayClass and Gateway reconcilers.
func RegisterGatewayControllers(logger logr.Logger, mgr manager.Manager, options GatewayControllerOptions) error {
	classController := &GatewayClassController{
		logger:         logger.WithName("gatewayclass-controller"),
		kubeClient:     mgr.GetClient(),
		controllerName: options.ControllerName,
		namespace:      options.Namespace,
	}
	enqueueAllClasses := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []reconcile.Request {
		list := &gatewayv1.GatewayClassList{}
		if err := mgr.GetClient().List(ctx, list); err != nil {
			return nil
		}
		var requests []reconcile.Request
		for _, item := range list.Items {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: item.Name}})
		}
		return requests
	})
	err := builder.ControllerManagedBy(mgr).
		Named("gatewayclass").
		For(&gatewayv1.GatewayClass{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&corev1.ConfigMap{}, enqueueAllClasses).
		Complete(classController)
	if err != nil {
		return errors.Wrap(err, "register gatewayclass controller")
	}

	gatewayReconciler := &GatewayReconciler{
		logger:         logger.WithName("gateway-controller"),
		kubeClient:     mgr.GetClient(),
		apiReader:      mgr.GetAPIReader(),
		controllerName: options.ControllerName,
		namespace:      options.Namespace,
		clusterDomain:  options.ClusterDomain,
		proxyConfig:    options.ProxyConfig,
		tunnelSync:     options.TunnelSync,
	}
	enqueueOne := handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
		return []reconcile.Request{gatewayReconcileKey}
	})
	isProxyResource := predicate.NewPredicateFuncs(func(object client.Object) bool {
		_, ok := object.GetLabels()[gatewayProxyLabelKey]
		return ok
	})
	err = builder.ControllerManagedBy(mgr).
		Named("gateway").
		Watches(&gatewayv1.GatewayClass{}, enqueueOne, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&gatewayv1.Gateway{}, enqueueOne, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&gatewayv1.HTTPRoute{}, enqueueOne, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&gatewayv1.ReferenceGrant{}, enqueueOne).
		Watches(&corev1.Namespace{}, enqueueOne, builder.WithPredicates(predicate.LabelChangedPredicate{})).
		Watches(&corev1.Service{}, enqueueOne).
		Watches(&discoveryv1.EndpointSlice{}, enqueueOne).
		Watches(&corev1.ConfigMap{}, enqueueOne).
		Watches(&appsv1.Deployment{}, enqueueOne, builder.WithPredicates(isProxyResource)).
		Complete(gatewayReconciler)
	if err != nil {
		return errors.Wrap(err, "register gateway controller")
	}
	return nil
}

func (r *GatewayReconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	started := time.Now()

	snapshot, err := r.loadSnapshot(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}

	// classes of this controller with valid parameters
	addressings := map[string]gatewayAddressing{}
	classes := &gatewayv1.GatewayClassList{}
	if err := r.kubeClient.List(ctx, classes); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "list gatewayclasses")
	}
	for i := range classes.Items {
		class := &classes.Items[i]
		if string(class.Spec.ControllerName) != r.controllerName {
			continue
		}
		addressing, reason, err := gatewayClassAddressing(ctx, r.kubeClient, class, r.namespace)
		if err != nil {
			return reconcile.Result{}, err
		}
		if reason == "" {
			addressings[class.Name] = addressing
		}
	}

	gateways := &gatewayv1.GatewayList{}
	if err := r.kubeClient.List(ctx, gateways); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "list gateways")
	}
	states := map[types.NamespacedName]*gatewayState{}
	var orderedStates []*gatewayState
	for i := range gateways.Items {
		gateway := &gateways.Items[i]
		addressing, ok := addressings[string(gateway.Spec.GatewayClassName)]
		if !ok || gateway.DeletionTimestamp != nil {
			continue
		}
		state, err := buildGatewayState(ctx, gateway, addressing, snapshot, r.getSecret)
		if err != nil {
			return reconcile.Result{}, err
		}
		states[client.ObjectKeyFromObject(gateway)] = state
		orderedStates = append(orderedStates, state)
	}

	routes := &gatewayv1.HTTPRouteList{}
	if err := r.kubeClient.List(ctx, routes); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "list httproutes")
	}
	routeResults := attachRoutes(states, routes.Items, snapshot)

	proxyConfig, err := r.proxyConfig(ctx)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "resolve gateway proxy config")
	}
	proxyReady := map[types.NamespacedName]bool{}
	keepProxies := map[string]bool{}
	var exposures []exposure.Exposure
	for _, state := range orderedStates {
		if !state.accepted {
			continue
		}
		key := client.ObjectKeyFromObject(state.gateway)
		table := compileTable(state, snapshot)
		ready, err := ensureGatewayProxy(ctx, r.kubeClient, state.gateway, r.namespace, proxyConfig, table)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "ensure proxy of gateway %s", key)
		}
		proxyReady[key] = ready
		keepProxies[gatewayProxyName(state.gateway)] = true
		exposures = append(exposures, exposure.Exposure{
			Hostname:      state.address,
			ServiceTarget: gatewayProxyServiceTarget(state.gateway, r.namespace, r.clusterDomain),
		})
	}

	syncErr := r.tunnelSync.SyncGateway(ctx, exposures)
	if syncErr != nil {
		r.logger.Error(syncErr, "sync gateway exposures to cloudflare")
	}

	pending := 0
	for _, state := range orderedStates {
		key := client.ObjectKeyFromObject(state.gateway)
		programmed := state.accepted && proxyReady[key] && syncErr == nil
		if state.accepted && !programmed {
			pending++
		}
		err := updateGatewayStatus(ctx, r.kubeClient, r.apiReader, state.gateway, func(existing gatewayv1.GatewayStatus) gatewayv1.GatewayStatus {
			return gatewayStatusFor(state, programmed, existing)
		})
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "update status of gateway %s", key)
		}
	}

	for i := range routes.Items {
		route := &routes.Items[i]
		results, ok := routeResults[client.ObjectKeyFromObject(route)]
		// a route that no longer points at our Gateways still needs its
		// stale entries of this controller removed
		if !ok && !slices.ContainsFunc(route.Status.Parents, func(parent gatewayv1.RouteParentStatus) bool {
			return string(parent.ControllerName) == r.controllerName
		}) {
			continue
		}
		refsReason, refsMessage := routeResolvedRefs(route, snapshot)
		err := updateRouteStatus(ctx, r.kubeClient, r.apiReader, route, func(current *gatewayv1.HTTPRoute) []gatewayv1.RouteParentStatus {
			return routeParentStatusesFor(current, r.controllerName, results, refsReason, refsMessage)
		})
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "update status of httproute %s/%s", route.Namespace, route.Name)
		}
	}

	if err := deleteStaleGatewayProxies(ctx, r.kubeClient, r.namespace, keepProxies); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.V(1).Info("gateway reconcile completed", "gateways", len(orderedStates), "routes", len(routes.Items), "pending", pending, "duration", time.Since(started).String())
	if syncErr != nil {
		return reconcile.Result{}, errors.Wrap(syncErr, "sync gateway exposures")
	}
	if pending > 0 {
		return reconcile.Result{RequeueAfter: gatewayPendingRequeue}, nil
	}
	return reconcile.Result{RequeueAfter: gatewayResyncPeriod}, nil
}

// getSecret reads Secrets straight from the API server: the cache only holds
// Secrets of the controller namespace.
func (r *GatewayReconciler) getSecret(ctx context.Context, key types.NamespacedName) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := r.apiReader.Get(ctx, key, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

func (r *GatewayReconciler) loadSnapshot(ctx context.Context) (*clusterSnapshot, error) {
	snapshot := &clusterSnapshot{
		namespaces:     map[string]*corev1.Namespace{},
		services:       map[types.NamespacedName]*corev1.Service{},
		endpointSlices: map[types.NamespacedName][]discoveryv1.EndpointSlice{},
		clusterDomain:  r.clusterDomain,
	}

	namespaces := &corev1.NamespaceList{}
	if err := r.kubeClient.List(ctx, namespaces); err != nil {
		return nil, errors.Wrap(err, "list namespaces")
	}
	for i := range namespaces.Items {
		snapshot.namespaces[namespaces.Items[i].Name] = &namespaces.Items[i]
	}

	services := &corev1.ServiceList{}
	if err := r.kubeClient.List(ctx, services); err != nil {
		return nil, errors.Wrap(err, "list services")
	}
	for i := range services.Items {
		snapshot.services[client.ObjectKeyFromObject(&services.Items[i])] = &services.Items[i]
	}

	slices := &discoveryv1.EndpointSliceList{}
	if err := r.kubeClient.List(ctx, slices); err != nil {
		return nil, errors.Wrap(err, "list endpointslices")
	}
	for _, slice := range slices.Items {
		serviceName := slice.Labels[discoveryv1.LabelServiceName]
		if serviceName == "" {
			continue
		}
		key := types.NamespacedName{Namespace: slice.Namespace, Name: serviceName}
		snapshot.endpointSlices[key] = append(snapshot.endpointSlices[key], slice)
	}

	grants := &gatewayv1.ReferenceGrantList{}
	if err := r.kubeClient.List(ctx, grants); err != nil {
		return nil, errors.Wrap(err, "list referencegrants")
	}
	snapshot.referenceGrants = grants.Items
	return snapshot, nil
}

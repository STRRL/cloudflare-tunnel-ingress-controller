package controller

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/apis/cloudflareaccess/v1alpha1"
	cloudflarecontroller "github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/cloudflare-controller"
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/metrics"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AccessFinalizer guards Cloudflare side cleanup when a
// CloudflareAccess object is deleted.
const AccessFinalizer = "cloudflare-tunnel-ingress-controller.strrl.dev/access-cleanup"

// accessTargetIndexKey indexes CloudflareAccess objects by the
// namespaced names of the Ingresses they reference.
const accessTargetIndexKey = "spec.targetRefs.ingress"

// accessResyncInterval is how often a healthy object is reconciled
// again to correct drift on the Cloudflare side.
const accessResyncInterval = 10 * time.Minute

var _ reconcile.Reconciler = &AccessController{}

// AccessController reconciles CloudflareAccess objects.
type AccessController struct {
	logger       logr.Logger
	kubeClient   client.Client
	recorder     record.EventRecorder
	accessClient *cloudflarecontroller.AccessClient
}

func NewAccessController(logger logr.Logger, kubeClient client.Client, recorder record.EventRecorder, accessClient *cloudflarecontroller.AccessClient) *AccessController {
	return &AccessController{
		logger:       logger,
		kubeClient:   kubeClient,
		recorder:     recorder,
		accessClient: accessClient,
	}
}

func (a *AccessController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	access := v1alpha1.CloudflareAccess{}
	err := a.kubeClient.Get(ctx, request.NamespacedName, &access)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, errors.Wrapf(err, "fetch cloudflareaccess %s", request.NamespacedName)
	}

	applicationName := cloudflarecontroller.AccessApplicationName(a.accessClient.TunnelName(), access.Namespace, access.Name)

	if !access.DeletionTimestamp.IsZero() {
		return a.cleanup(ctx, &access, applicationName)
	}

	if controllerutil.AddFinalizer(&access, AccessFinalizer) {
		err = a.kubeClient.Update(ctx, &access)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "attach finalizer to cloudflareaccess %s", request.NamespacedName)
		}
	}

	status := access.Status.DeepCopy()
	status.ObservedGeneration = access.Generation

	hostnames, missingTargets, err := a.resolveTargets(ctx, &access)
	if err != nil {
		return reconcile.Result{}, err
	}

	conflictingOwner, err := a.findConflictingOwner(ctx, &access, hostnames)
	if err != nil {
		return reconcile.Result{}, err
	}
	if conflictingOwner != "" {
		message := fmt.Sprintf("hostnames overlap with older CloudflareAccess %s", conflictingOwner)
		setAccessCondition(status, access.Generation, v1alpha1.ConditionAccepted, metav1.ConditionFalse, v1alpha1.ReasonConflicted, message)
		a.recorder.Event(&access, corev1.EventTypeWarning, v1alpha1.ReasonConflicted, message)
		return a.patchStatus(ctx, &access, status)
	}

	if len(hostnames) == 0 {
		message := "no hostnames found on the referenced Ingresses, the access application stays unchanged"
		if len(missingTargets) > 0 {
			setAccessCondition(status, access.Generation, v1alpha1.ConditionResolvedRefs, metav1.ConditionFalse, v1alpha1.ReasonTargetNotFound, fmt.Sprintf("missing targets: %v", missingTargets))
		} else {
			setAccessCondition(status, access.Generation, v1alpha1.ConditionResolvedRefs, metav1.ConditionFalse, v1alpha1.ReasonNoHostnames, message)
		}
		setAccessCondition(status, access.Generation, v1alpha1.ConditionAccepted, metav1.ConditionTrue, v1alpha1.ReasonAccepted, "accepted, waiting for resolvable targets")
		a.recorder.Event(&access, corev1.EventTypeWarning, v1alpha1.ReasonNoHostnames, message)
		return a.patchStatus(ctx, &access, status)
	}

	policyRefs := make([]cloudflarecontroller.AccessPolicyReference, 0, len(access.Spec.Policies))
	for _, ref := range access.Spec.Policies {
		policyRefs = append(policyRefs, cloudflarecontroller.AccessPolicyReference{Name: ref.Name, ID: ref.ID})
	}
	policyIDs, err := a.accessClient.ResolvePolicies(ctx, policyRefs)
	if err != nil {
		var resolution *cloudflarecontroller.AccessPolicyResolutionError
		if errors.As(err, &resolution) {
			reason := v1alpha1.ReasonPolicyNotFound
			if resolution.Ambiguous {
				reason = v1alpha1.ReasonAmbiguousPolicy
			}
			setAccessCondition(status, access.Generation, v1alpha1.ConditionResolvedRefs, metav1.ConditionFalse, reason, resolution.Error())
			setAccessCondition(status, access.Generation, v1alpha1.ConditionAccepted, metav1.ConditionTrue, v1alpha1.ReasonAccepted, "accepted, waiting for resolvable policies")
			a.recorder.Event(&access, corev1.EventTypeWarning, reason, resolution.Error())
			return a.patchStatus(ctx, &access, status)
		}
		setAccessCondition(status, access.Generation, v1alpha1.ConditionAccepted, metav1.ConditionFalse, v1alpha1.ReasonCloudflareError, err.Error())
		_, patchErr := a.patchStatus(ctx, &access, status)
		if patchErr != nil {
			a.logger.Error(patchErr, "patch status after cloudflare error", "cloudflareaccess", request.NamespacedName)
		}
		return reconcile.Result{}, err
	}

	autoRedirect := access.Spec.AutoRedirectToIdentity != nil && *access.Spec.AutoRedirectToIdentity
	applicationID, aud, err := a.accessClient.EnsureApplication(ctx, access.Status.ApplicationID, cloudflarecontroller.DesiredAccessApplication{
		Name:                     applicationName,
		Hostnames:                hostnames,
		PolicyIDs:                policyIDs,
		SessionDuration:          access.Spec.SessionDuration,
		AllowedIdentityProviders: access.Spec.AllowedIdentityProviders,
		AutoRedirectToIdentity:   autoRedirect,
	})
	if err != nil {
		setAccessCondition(status, access.Generation, v1alpha1.ConditionAccepted, metav1.ConditionFalse, v1alpha1.ReasonCloudflareError, err.Error())
		a.recorder.Event(&access, corev1.EventTypeWarning, v1alpha1.ReasonCloudflareError, err.Error())
		_, patchErr := a.patchStatus(ctx, &access, status)
		if patchErr != nil {
			a.logger.Error(patchErr, "patch status after cloudflare error", "cloudflareaccess", request.NamespacedName)
		}
		return reconcile.Result{}, err
	}

	if access.Status.ApplicationID == "" {
		a.recorder.Eventf(&access, corev1.EventTypeNormal, "Created", "created access application %s", applicationID)
	}

	status.ApplicationID = applicationID
	status.AUD = aud
	status.Hostnames = hostnames
	if len(missingTargets) > 0 {
		setAccessCondition(status, access.Generation, v1alpha1.ConditionResolvedRefs, metav1.ConditionFalse, v1alpha1.ReasonTargetNotFound, fmt.Sprintf("missing targets: %v", missingTargets))
	} else {
		setAccessCondition(status, access.Generation, v1alpha1.ConditionResolvedRefs, metav1.ConditionTrue, v1alpha1.ReasonResolved, "all references resolved")
	}
	setAccessCondition(status, access.Generation, v1alpha1.ConditionAccepted, metav1.ConditionTrue, v1alpha1.ReasonAccepted, "access application is managed")
	return a.patchStatus(ctx, &access, status)
}

func (a *AccessController) cleanup(ctx context.Context, access *v1alpha1.CloudflareAccess, applicationName string) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(access, AccessFinalizer) {
		return reconcile.Result{}, nil
	}

	err := a.accessClient.DeleteApplication(ctx, access.Status.ApplicationID, applicationName)
	if err != nil {
		a.recorder.Event(access, corev1.EventTypeWarning, v1alpha1.ReasonCloudflareError, err.Error())
		return reconcile.Result{}, errors.Wrapf(err, "delete access application for %s/%s", access.Namespace, access.Name)
	}
	a.recorder.Event(access, corev1.EventTypeNormal, "Deleted", "deleted access application")

	controllerutil.RemoveFinalizer(access, AccessFinalizer)
	err = a.kubeClient.Update(ctx, access)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "remove finalizer from cloudflareaccess %s/%s", access.Namespace, access.Name)
	}
	return reconcile.Result{}, nil
}

// resolveTargets fetches the referenced Ingresses and collects their
// hostnames. Missing Ingresses are reported, they do not abort the
// reconcile.
func (a *AccessController) resolveTargets(ctx context.Context, access *v1alpha1.CloudflareAccess) ([]string, []string, error) {
	var ingresses []networkingv1.Ingress
	var missing []string
	for _, ref := range access.Spec.TargetRefs {
		ingress := networkingv1.Ingress{}
		err := a.kubeClient.Get(ctx, types.NamespacedName{Namespace: access.Namespace, Name: ref.Name}, &ingress)
		if err != nil {
			if apierrors.IsNotFound(err) {
				missing = append(missing, ref.Name)
				continue
			}
			return nil, nil, errors.Wrapf(err, "fetch target ingress %s/%s", access.Namespace, ref.Name)
		}
		ingresses = append(ingresses, ingress)
	}
	return collectHostnames(ingresses), missing, nil
}

// findConflictingOwner returns the identifier of an older
// CloudflareAccess object whose hostnames overlap with ours, or an
// empty string when there is none. The oldest object wins a conflict;
// ties break by namespace/name order. Recovery after the winner goes
// away happens on the periodic resync.
func (a *AccessController) findConflictingOwner(ctx context.Context, access *v1alpha1.CloudflareAccess, hostnames []string) (string, error) {
	if len(hostnames) == 0 {
		return "", nil
	}

	all := v1alpha1.CloudflareAccessList{}
	err := a.kubeClient.List(ctx, &all)
	if err != nil {
		return "", errors.Wrap(err, "list cloudflareaccess objects")
	}

	ours := map[string]struct{}{}
	for _, hostname := range hostnames {
		ours[hostname] = struct{}{}
	}

	for i := range all.Items {
		other := &all.Items[i]
		if other.Namespace == access.Namespace && other.Name == access.Name {
			continue
		}
		if !other.DeletionTimestamp.IsZero() {
			continue
		}
		if !accessPrecedes(other, access) {
			continue
		}
		otherHostnames, _, err := a.resolveTargets(ctx, other)
		if err != nil {
			return "", err
		}
		for _, hostname := range otherHostnames {
			if _, overlap := ours[hostname]; overlap {
				return fmt.Sprintf("%s/%s", other.Namespace, other.Name), nil
			}
		}
	}
	return "", nil
}

func (a *AccessController) patchStatus(ctx context.Context, access *v1alpha1.CloudflareAccess, status *v1alpha1.CloudflareAccessStatus) (reconcile.Result, error) {
	base := access.DeepCopy()
	access.Status = *status
	err := a.kubeClient.Status().Patch(ctx, access, client.MergeFrom(base))
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "patch status of cloudflareaccess %s/%s", access.Namespace, access.Name)
	}
	metrics.ManagedAccessApplications.Set(a.countManagedApplications(ctx))
	return reconcile.Result{RequeueAfter: accessResyncInterval}, nil
}

func (a *AccessController) countManagedApplications(ctx context.Context) float64 {
	all := v1alpha1.CloudflareAccessList{}
	err := a.kubeClient.List(ctx, &all)
	if err != nil {
		return 0
	}
	count := 0
	for i := range all.Items {
		if all.Items[i].Status.ApplicationID != "" {
			count++
		}
	}
	return float64(count)
}

// collectHostnames returns the sorted set of hostnames found on the
// rules of the given Ingresses. Rules without a host are skipped.
func collectHostnames(ingresses []networkingv1.Ingress) []string {
	seen := map[string]struct{}{}
	var hostnames []string
	for _, ingress := range ingresses {
		for _, rule := range ingress.Spec.Rules {
			if rule.Host == "" {
				continue
			}
			if _, ok := seen[rule.Host]; ok {
				continue
			}
			seen[rule.Host] = struct{}{}
			hostnames = append(hostnames, rule.Host)
		}
	}
	sort.Strings(hostnames)
	return hostnames
}

// accessPrecedes reports whether a wins a conflict against b. The
// older object wins, ties break by namespace/name order.
func accessPrecedes(a *v1alpha1.CloudflareAccess, b *v1alpha1.CloudflareAccess) bool {
	if !a.CreationTimestamp.Equal(&b.CreationTimestamp) {
		return a.CreationTimestamp.Before(&b.CreationTimestamp)
	}
	aKey := a.Namespace + "/" + a.Name
	bKey := b.Namespace + "/" + b.Name
	return aKey < bKey
}

func setAccessCondition(status *v1alpha1.CloudflareAccessStatus, generation int64, conditionType string, conditionStatus metav1.ConditionStatus, reason string, message string) {
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
	})
}

// RegisterAccessController wires the CloudflareAccess reconciler into
// the manager: an index from referenced Ingresses to the objects that
// reference them, a watch on CloudflareAccess objects and a watch on
// Ingresses feeding through that index.
func RegisterAccessController(logger logr.Logger, mgr manager.Manager, accessClient *cloudflarecontroller.AccessClient) error {
	err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1alpha1.CloudflareAccess{}, accessTargetIndexKey, func(object client.Object) []string {
		access, ok := object.(*v1alpha1.CloudflareAccess)
		if !ok {
			return nil
		}
		var keys []string
		for _, ref := range access.Spec.TargetRefs {
			keys = append(keys, access.Namespace+"/"+ref.Name)
		}
		return keys
	})
	if err != nil {
		return errors.Wrap(err, "index cloudflareaccess target refs")
	}

	controller := NewAccessController(logger.WithName("access-controller"), mgr.GetClient(), mgr.GetEventRecorderFor("cloudflare-tunnel-ingress-controller"), accessClient)

	mapIngress := func(ctx context.Context, object client.Object) []reconcile.Request {
		referencing := v1alpha1.CloudflareAccessList{}
		err := mgr.GetClient().List(ctx, &referencing, client.MatchingFields{
			accessTargetIndexKey: object.GetNamespace() + "/" + object.GetName(),
		})
		if err != nil {
			logger.Error(err, "list cloudflareaccess objects referencing ingress", "ingress", object.GetNamespace()+"/"+object.GetName())
			return nil
		}
		var requests []reconcile.Request
		for i := range referencing.Items {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: referencing.Items[i].Namespace,
					Name:      referencing.Items[i].Name,
				},
			})
		}
		return requests
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.CloudflareAccess{}).
		Watches(&networkingv1.Ingress{}, handler.EnqueueRequestsFromMapFunc(mapIngress)).
		Complete(controller)
}

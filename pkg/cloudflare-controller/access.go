package cloudflarecontroller

import (
	"context"
	"fmt"
	"strings"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/metrics"
	"github.com/cloudflare/cloudflare-go"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

// AccessOwnershipTag marks every Access Application managed by this
// controller. It shows up as a filterable tag in the Cloudflare
// dashboard.
const AccessOwnershipTag = "managed-by-cloudflare-tunnel-ingress-controller"

// defaultAccessSessionDuration matches the Cloudflare default. The
// controller always sends an explicit value so removing the field from
// the spec falls back to this default instead of keeping an old value.
const defaultAccessSessionDuration = "24h"

// AccessApplicationName builds the deterministic name of the managed
// Access Application. The tunnel name keeps two clusters that share one
// Cloudflare account from colliding on the same namespace and object
// name.
func AccessApplicationName(tunnelName string, namespace string, name string) string {
	return fmt.Sprintf("ctic:%s:%s/%s", tunnelName, namespace, name)
}

// AccessPolicyReference references one reusable Access Policy by name
// or by ID. Exactly one of the fields is set.
type AccessPolicyReference struct {
	Name string
	ID   string
}

// AccessPolicyResolutionError reports one reference that failed to
// resolve. Ambiguous tells a duplicated name apart from a missing one.
type AccessPolicyResolutionError struct {
	Reference AccessPolicyReference
	Ambiguous bool
}

func (e *AccessPolicyResolutionError) Error() string {
	ref := e.Reference.Name
	if ref == "" {
		ref = e.Reference.ID
	}
	if e.Ambiguous {
		return fmt.Sprintf("more than one reusable access policy is named %q", ref)
	}
	return fmt.Sprintf("reusable access policy %q not found", ref)
}

// DesiredAccessApplication is the desired state of one managed Access
// Application.
type DesiredAccessApplication struct {
	// Name is the deterministic application name.
	Name string
	// Hostnames become the public destinations. Must not be empty.
	Hostnames []string
	// PolicyIDs are attached in ascending order of precedence.
	PolicyIDs []string
	// SessionDuration is empty for the Cloudflare default.
	SessionDuration string
	// AllowedIdentityProviders is empty for all providers of the account.
	AllowedIdentityProviders []string
	// AutoRedirectToIdentity skips the identity provider selection page.
	AutoRedirectToIdentity bool
}

// AccessClient manages Access Applications for CloudflareAccess objects.
type AccessClient struct {
	logger     logr.Logger
	cfClient   *cloudflare.API
	accountId  string
	tunnelName string
}

func NewAccessClient(logger logr.Logger, cfClient *cloudflare.API, accountId string, tunnelName string) *AccessClient {
	return &AccessClient{
		logger:     logger,
		cfClient:   cfClient,
		accountId:  accountId,
		tunnelName: tunnelName,
	}
}

func (a *AccessClient) TunnelName() string {
	return a.tunnelName
}

// ResolvePolicies maps references to reusable Access Policy IDs,
// keeping the input order. It fails on the first reference that does
// not resolve to exactly one policy.
func (a *AccessClient) ResolvePolicies(ctx context.Context, refs []AccessPolicyReference) ([]string, error) {
	rc := cloudflare.AccountIdentifier(a.accountId)
	// Listing without an application ID returns only the reusable
	// policies of the account. Pagination is handled by the SDK.
	reusable, _, err := a.cfClient.ListAccessPolicies(ctx, rc, cloudflare.ListAccessPoliciesParams{})
	if err != nil {
		metrics.CloudflareAPIErrors.WithLabelValues("list_access_policies").Inc()
		return nil, errors.Wrap(err, "list reusable access policies")
	}

	byID := make(map[string]struct{}, len(reusable))
	countByName := make(map[string]int, len(reusable))
	idByName := make(map[string]string, len(reusable))
	for _, policy := range reusable {
		byID[policy.ID] = struct{}{}
		countByName[policy.Name]++
		idByName[policy.Name] = policy.ID
	}

	var ids []string
	for _, ref := range refs {
		if ref.ID != "" {
			if _, ok := byID[ref.ID]; !ok {
				return nil, &AccessPolicyResolutionError{Reference: ref}
			}
			ids = append(ids, ref.ID)
			continue
		}
		switch countByName[ref.Name] {
		case 0:
			return nil, &AccessPolicyResolutionError{Reference: ref}
		case 1:
			ids = append(ids, idByName[ref.Name])
		default:
			return nil, &AccessPolicyResolutionError{Reference: ref, Ambiguous: true}
		}
	}
	return ids, nil
}

// EnsureApplication creates or updates the Access Application to match
// the desired state. knownID is the application ID remembered in the
// object status, empty when unknown. It returns the application ID and
// the application audience tag.
func (a *AccessClient) EnsureApplication(ctx context.Context, knownID string, desired DesiredAccessApplication) (string, string, error) {
	if len(desired.Hostnames) == 0 {
		return "", "", errors.New("desired access application has no hostnames")
	}

	rc := cloudflare.AccountIdentifier(a.accountId)

	err := a.ensureOwnershipTag(ctx)
	if err != nil {
		return "", "", err
	}

	existing, err := a.findApplication(ctx, knownID, desired.Name)
	if err != nil {
		return "", "", err
	}

	destinations := make([]cloudflare.AccessDestination, 0, len(desired.Hostnames))
	for _, hostname := range desired.Hostnames {
		destinations = append(destinations, cloudflare.AccessDestination{
			Type: cloudflare.AccessDestinationPublic,
			URI:  hostname,
		})
	}

	sessionDuration := desired.SessionDuration
	if sessionDuration == "" {
		sessionDuration = defaultAccessSessionDuration
	}
	autoRedirect := desired.AutoRedirectToIdentity
	policyIDs := append([]string(nil), desired.PolicyIDs...)

	if existing == nil {
		created, err := a.cfClient.CreateAccessApplication(ctx, rc, cloudflare.CreateAccessApplicationParams{
			Name:                   desired.Name,
			Type:                   cloudflare.SelfHosted,
			Domain:                 desired.Hostnames[0],
			Destinations:           destinations,
			SessionDuration:        sessionDuration,
			AllowedIdps:            desired.AllowedIdentityProviders,
			AutoRedirectToIdentity: &autoRedirect,
			Tags:                   []string{AccessOwnershipTag},
			Policies:               policyIDs,
		})
		if err != nil {
			metrics.CloudflareAPIErrors.WithLabelValues("create_access_application").Inc()
			return "", "", errors.Wrapf(err, "create access application %s", desired.Name)
		}
		a.logger.Info("created access application", "name", desired.Name, "id", created.ID)
		return created.ID, created.AUD, nil
	}

	updated, err := a.cfClient.UpdateAccessApplication(ctx, rc, cloudflare.UpdateAccessApplicationParams{
		ID:                     existing.ID,
		Name:                   desired.Name,
		Type:                   cloudflare.SelfHosted,
		Domain:                 desired.Hostnames[0],
		Destinations:           destinations,
		SessionDuration:        sessionDuration,
		AllowedIdps:            desired.AllowedIdentityProviders,
		AutoRedirectToIdentity: &autoRedirect,
		Tags:                   []string{AccessOwnershipTag},
		Policies:               &policyIDs,
	})
	if err != nil {
		metrics.CloudflareAPIErrors.WithLabelValues("update_access_application").Inc()
		return "", "", errors.Wrapf(err, "update access application %s", desired.Name)
	}
	a.logger.V(1).Info("updated access application", "name", desired.Name, "id", updated.ID)
	return updated.ID, updated.AUD, nil
}

// DeleteApplication removes the Access Application. A missing
// application is not an error, so deletion stays idempotent.
func (a *AccessClient) DeleteApplication(ctx context.Context, knownID string, name string) error {
	rc := cloudflare.AccountIdentifier(a.accountId)

	existing, err := a.findApplication(ctx, knownID, name)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}

	err = a.cfClient.DeleteAccessApplication(ctx, rc, existing.ID)
	if err != nil {
		metrics.CloudflareAPIErrors.WithLabelValues("delete_access_application").Inc()
		return errors.Wrapf(err, "delete access application %s", name)
	}
	a.logger.Info("deleted access application", "name", name, "id", existing.ID)
	return nil
}

// findApplication looks the managed application up by its remembered ID
// first, then by its deterministic name plus the ownership tag.
func (a *AccessClient) findApplication(ctx context.Context, knownID string, name string) (*cloudflare.AccessApplication, error) {
	rc := cloudflare.AccountIdentifier(a.accountId)

	if knownID != "" {
		app, err := a.cfClient.GetAccessApplication(ctx, rc, knownID)
		if err == nil {
			return &app, nil
		}
		var notFound *cloudflare.NotFoundError
		if !errors.As(err, &notFound) {
			metrics.CloudflareAPIErrors.WithLabelValues("get_access_application").Inc()
			return nil, errors.Wrapf(err, "get access application %s", knownID)
		}
	}

	apps, _, err := a.cfClient.ListAccessApplications(ctx, rc, cloudflare.ListAccessApplicationsParams{})
	if err != nil {
		metrics.CloudflareAPIErrors.WithLabelValues("list_access_applications").Inc()
		return nil, errors.Wrap(err, "list access applications")
	}
	for i := range apps {
		if apps[i].Name == name && hasOwnershipTag(apps[i].Tags) {
			return &apps[i], nil
		}
	}
	return nil, nil
}

func hasOwnershipTag(tags []string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, AccessOwnershipTag) {
			return true
		}
	}
	return false
}

// ensureOwnershipTag creates the ownership tag when it does not exist
// yet. Tag creation is idempotent from the caller point of view.
func (a *AccessClient) ensureOwnershipTag(ctx context.Context) error {
	rc := cloudflare.AccountIdentifier(a.accountId)

	_, err := a.cfClient.GetAccessTag(ctx, rc, AccessOwnershipTag)
	if err == nil {
		return nil
	}
	var notFound *cloudflare.NotFoundError
	if !errors.As(err, &notFound) {
		metrics.CloudflareAPIErrors.WithLabelValues("get_access_tag").Inc()
		return errors.Wrap(err, "get access ownership tag")
	}

	_, err = a.cfClient.CreateAccessTag(ctx, rc, cloudflare.CreateAccessTagParams{
		Name: AccessOwnershipTag,
	})
	if err != nil {
		metrics.CloudflareAPIErrors.WithLabelValues("create_access_tag").Inc()
		return errors.Wrap(err, "create access ownership tag")
	}
	a.logger.Info("created access ownership tag", "tag", AccessOwnershipTag)
	return nil
}

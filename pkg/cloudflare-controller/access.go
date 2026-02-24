package cloudflarecontroller

import (
	"context"
	"fmt"
	"strings"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go"
	"github.com/pkg/errors"
)

const accessAppNamePrefix = "ctic"

// accessAppName returns the deterministic name for an owned Access Application.
// Format: ctic:<tunnelName>:<hostname>
func accessAppName(tunnelName, hostname string) string {
	return fmt.Sprintf("%s:%s:%s", accessAppNamePrefix, tunnelName, hostname)
}

// isOwnedAccessApp returns true if the Access Application was created by this controller instance.
func isOwnedAccessApp(app cloudflare.AccessApplication, tunnelName string) bool {
	prefix := fmt.Sprintf("%s:%s:", accessAppNamePrefix, tunnelName)
	return strings.HasPrefix(app.Name, prefix)
}

// hostnameFromAccessAppName extracts the hostname from an owned Access Application name.
func hostnameFromAccessAppName(name, tunnelName string) string {
	prefix := fmt.Sprintf("%s:%s:", accessAppNamePrefix, tunnelName)
	return strings.TrimPrefix(name, prefix)
}

const defaultSessionDuration = "24h"

// desiredAccessApp holds the desired state for an Access Application on a given hostname.
// Group fields contain names from annotations; IDs are resolved at reconcile time.
type desiredAccessApp struct {
	AllowedGroupIDs []string
	DeniedGroupIDs  []string
	Bypass          bool
	SessionDuration string
	AutoRedirect    *bool
}

// resolveGroupNames maps group names to IDs using the provided lookup map.
// Unknown names are logged and skipped.
func resolveGroupNames(names []string, nameToID map[string]string) []string {
	var ids []string
	for _, name := range names {
		if id, ok := nameToID[name]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// desiredAccessApps computes the desired Access Applications from exposures.
// For each hostname, it unions all AllowedAccessGroupIDs and DeniedAccessGroupIDs
// from non-deleted exposures. Hostnames with no allowed groups are omitted.
func desiredAccessApps(exposures []exposure.Exposure) map[string]desiredAccessApp {
	result := make(map[string]desiredAccessApp)

	for _, e := range exposures {
		if e.IsDeleted {
			continue
		}

		hasGroups := len(e.AllowedAccessGroupIDs) > 0 || len(e.DeniedAccessGroupIDs) > 0
		if !hasGroups && !e.AccessBypass {
			continue
		}

		existing := result[e.Hostname]
		existing.AllowedGroupIDs = unionStrings(existing.AllowedGroupIDs, e.AllowedAccessGroupIDs)
		existing.DeniedGroupIDs = unionStrings(existing.DeniedGroupIDs, e.DeniedAccessGroupIDs)
		if e.AccessBypass {
			existing.Bypass = true
		}
		if e.AccessSessionDuration != "" {
			existing.SessionDuration = e.AccessSessionDuration
		}
		if e.AccessAutoRedirect != nil {
			existing.AutoRedirect = e.AccessAutoRedirect
		}
		result[e.Hostname] = existing
	}

	// Remove entries with no allowed groups and no bypass
	for hostname, app := range result {
		if len(app.AllowedGroupIDs) == 0 && !app.Bypass {
			delete(result, hostname)
		}
	}

	return result
}

// unionStrings returns the union of two string slices, preserving order and deduplicating.
func unionStrings(a, b []string) []string {
	seen := make(map[string]struct{}, len(a))
	for _, s := range a {
		seen[s] = struct{}{}
	}
	result := append([]string(nil), a...)
	for _, s := range b {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}
	return result
}

// accessGroupIncludeRules builds the include rules for an Access Policy from group IDs.
func accessGroupIncludeRules(groupIDs []string) []interface{} {
	rules := make([]interface{}, len(groupIDs))
	for i, id := range groupIDs {
		rules[i] = cloudflare.AccessGroupAccessGroup{
			Group: struct {
				ID string `json:"id"`
			}{ID: id},
		}
	}
	return rules
}

// updateAccessApplications is the main reconcile loop for Access Applications.
func (t *TunnelClient) updateAccessApplications(ctx context.Context, exposures []exposure.Exposure) error {
	rc := cloudflare.AccountIdentifier(t.accountId)

	// 0. Resolve group names to IDs — annotations contain names, API needs IDs
	allGroups, _, err := t.cfClient.ListAccessGroups(ctx, rc, cloudflare.ListAccessGroupsParams{})
	if err != nil {
		return errors.Wrap(err, "list access groups")
	}
	groupNameToID := make(map[string]string, len(allGroups))
	for _, g := range allGroups {
		groupNameToID[g.Name] = g.ID
	}

	// Resolve names in exposures to IDs
	for i := range exposures {
		exposures[i].AllowedAccessGroupIDs = resolveGroupNames(exposures[i].AllowedAccessGroupIDs, groupNameToID)
		exposures[i].DeniedAccessGroupIDs = resolveGroupNames(exposures[i].DeniedAccessGroupIDs, groupNameToID)
	}

	// 1. List all Access Applications, filter to owned ones
	allApps, _, err := t.cfClient.ListAccessApplications(ctx, rc, cloudflare.ListAccessApplicationsParams{})
	if err != nil {
		return errors.Wrap(err, "list access applications")
	}

	ownedApps := make(map[string]cloudflare.AccessApplication) // hostname -> app
	for _, app := range allApps {
		if isOwnedAccessApp(app, t.tunnelName) {
			hostname := hostnameFromAccessAppName(app.Name, t.tunnelName)
			ownedApps[hostname] = app
		}
	}

	// 2. Compute desired state
	desired := desiredAccessApps(exposures)

	// 3. Create or update to match desired state
	for hostname, want := range desired {
		existingApp, exists := ownedApps[hostname]
		if !exists {
			t.logger.Info("create access application", "hostname", hostname)
			err := t.createAccessApp(ctx, rc, hostname, want)
			if err != nil {
				return errors.Wrapf(err, "create access application for %s", hostname)
			}
		} else {
			t.logger.V(3).Info("update access application", "hostname", hostname, "app-id", existingApp.ID)
			err := t.updateAccessApp(ctx, rc, existingApp, want)
			if err != nil {
				return errors.Wrapf(err, "update access application for %s", hostname)
			}
		}
	}

	// 4. Delete owned apps that are no longer desired
	for hostname, app := range ownedApps {
		if _, wanted := desired[hostname]; !wanted {
			t.logger.Info("delete access application", "hostname", hostname, "app-id", app.ID)
			err := t.cfClient.DeleteAccessApplication(ctx, rc, app.ID)
			if err != nil {
				return errors.Wrapf(err, "delete access application for %s", hostname)
			}
		}
	}

	return nil
}

// createAccessApp creates an Access Application and its policies.
func (t *TunnelClient) createAccessApp(ctx context.Context, rc *cloudflare.ResourceContainer, hostname string, want desiredAccessApp) error {
	sessionDuration := defaultSessionDuration
	if want.SessionDuration != "" {
		sessionDuration = want.SessionDuration
	}

	params := cloudflare.CreateAccessApplicationParams{
		Name:            accessAppName(t.tunnelName, hostname),
		Domain:          hostname,
		Type:            cloudflare.SelfHosted,
		SessionDuration: sessionDuration,
	}
	if want.AutoRedirect != nil {
		params.AutoRedirectToIdentity = want.AutoRedirect
	}

	app, err := t.cfClient.CreateAccessApplication(ctx, rc, params)
	if err != nil {
		return errors.Wrap(err, "create access application")
	}

	if want.Bypass {
		_, err = t.cfClient.CreateAccessPolicy(ctx, rc, cloudflare.CreateAccessPolicyParams{
			ApplicationID: app.ID,
			Name:          "bypass",
			Decision:      "bypass",
			Precedence:    1,
			Include:       []interface{}{cloudflare.AccessGroupEveryone{}},
		})
		if err != nil {
			return errors.Wrap(err, "create bypass policy")
		}
		return nil
	}

	// Create allow policy
	_, err = t.cfClient.CreateAccessPolicy(ctx, rc, cloudflare.CreateAccessPolicyParams{
		ApplicationID: app.ID,
		Name:          "allow",
		Decision:      "allow",
		Precedence:    2,
		Include:       accessGroupIncludeRules(want.AllowedGroupIDs),
	})
	if err != nil {
		return errors.Wrap(err, "create allow policy")
	}

	// Create deny policy if deny groups are specified
	if len(want.DeniedGroupIDs) > 0 {
		_, err = t.cfClient.CreateAccessPolicy(ctx, rc, cloudflare.CreateAccessPolicyParams{
			ApplicationID: app.ID,
			Name:          "deny",
			Decision:      "deny",
			Precedence:    1,
			Include:       accessGroupIncludeRules(want.DeniedGroupIDs),
		})
		if err != nil {
			return errors.Wrap(err, "create deny policy")
		}
	}

	return nil
}

// updateAccessApp updates an existing Access Application's settings and policies.
func (t *TunnelClient) updateAccessApp(ctx context.Context, rc *cloudflare.ResourceContainer, app cloudflare.AccessApplication, want desiredAccessApp) error {
	// Update app-level settings (session duration, auto-redirect)
	sessionDuration := defaultSessionDuration
	if want.SessionDuration != "" {
		sessionDuration = want.SessionDuration
	}
	updateParams := cloudflare.UpdateAccessApplicationParams{
		ID:              app.ID,
		Name:            app.Name,
		Domain:          app.Domain,
		Type:            cloudflare.SelfHosted,
		SessionDuration: sessionDuration,
	}
	if want.AutoRedirect != nil {
		updateParams.AutoRedirectToIdentity = want.AutoRedirect
	}
	_, err := t.cfClient.UpdateAccessApplication(ctx, rc, updateParams)
	if err != nil {
		return errors.Wrap(err, "update access application settings")
	}

	// Update policies
	policies, _, err := t.cfClient.ListAccessPolicies(ctx, rc, cloudflare.ListAccessPoliciesParams{
		ApplicationID: app.ID,
	})
	if err != nil {
		return errors.Wrap(err, "list access policies")
	}

	var allowPolicy, denyPolicy, bypassPolicy *cloudflare.AccessPolicy
	for i := range policies {
		switch policies[i].Decision {
		case "allow":
			allowPolicy = &policies[i]
		case "deny":
			denyPolicy = &policies[i]
		case "bypass":
			bypassPolicy = &policies[i]
		}
	}

	if want.Bypass {
		// Ensure bypass policy exists, remove allow/deny if present
		if bypassPolicy == nil {
			_, err = t.cfClient.CreateAccessPolicy(ctx, rc, cloudflare.CreateAccessPolicyParams{
				ApplicationID: app.ID,
				Name:          "bypass",
				Decision:      "bypass",
				Precedence:    1,
				Include:       []interface{}{cloudflare.AccessGroupEveryone{}},
			})
			if err != nil {
				return errors.Wrap(err, "create bypass policy")
			}
		}
		if allowPolicy != nil {
			_ = t.cfClient.DeleteAccessPolicy(ctx, rc, cloudflare.DeleteAccessPolicyParams{ApplicationID: app.ID, PolicyID: allowPolicy.ID})
		}
		if denyPolicy != nil {
			_ = t.cfClient.DeleteAccessPolicy(ctx, rc, cloudflare.DeleteAccessPolicyParams{ApplicationID: app.ID, PolicyID: denyPolicy.ID})
		}
		return nil
	}

	// Remove bypass policy if switching away from bypass
	if bypassPolicy != nil {
		_ = t.cfClient.DeleteAccessPolicy(ctx, rc, cloudflare.DeleteAccessPolicyParams{ApplicationID: app.ID, PolicyID: bypassPolicy.ID})
	}

	// Update or create allow policy
	if allowPolicy != nil {
		_, err = t.cfClient.UpdateAccessPolicy(ctx, rc, cloudflare.UpdateAccessPolicyParams{
			ApplicationID: app.ID,
			PolicyID:      allowPolicy.ID,
			Name:          "allow",
			Decision:      "allow",
			Precedence:    2,
			Include:       accessGroupIncludeRules(want.AllowedGroupIDs),
		})
		if err != nil {
			return errors.Wrap(err, "update allow policy")
		}
	} else {
		_, err = t.cfClient.CreateAccessPolicy(ctx, rc, cloudflare.CreateAccessPolicyParams{
			ApplicationID: app.ID,
			Name:          "allow",
			Decision:      "allow",
			Precedence:    2,
			Include:       accessGroupIncludeRules(want.AllowedGroupIDs),
		})
		if err != nil {
			return errors.Wrap(err, "create allow policy")
		}
	}

	// Update, create, or delete deny policy
	if len(want.DeniedGroupIDs) > 0 {
		if denyPolicy != nil {
			_, err = t.cfClient.UpdateAccessPolicy(ctx, rc, cloudflare.UpdateAccessPolicyParams{
				ApplicationID: app.ID,
				PolicyID:      denyPolicy.ID,
				Name:          "deny",
				Decision:      "deny",
				Precedence:    1,
				Include:       accessGroupIncludeRules(want.DeniedGroupIDs),
			})
			if err != nil {
				return errors.Wrap(err, "update deny policy")
			}
		} else {
			_, err = t.cfClient.CreateAccessPolicy(ctx, rc, cloudflare.CreateAccessPolicyParams{
				ApplicationID: app.ID,
				Name:          "deny",
				Decision:      "deny",
				Precedence:    1,
				Include:       accessGroupIncludeRules(want.DeniedGroupIDs),
			})
			if err != nil {
				return errors.Wrap(err, "create deny policy")
			}
		}
	} else if denyPolicy != nil {
		err = t.cfClient.DeleteAccessPolicy(ctx, rc, cloudflare.DeleteAccessPolicyParams{
			ApplicationID: app.ID,
			PolicyID:      denyPolicy.ID,
		})
		if err != nil {
			return errors.Wrap(err, "delete deny policy")
		}
	}

	return nil
}

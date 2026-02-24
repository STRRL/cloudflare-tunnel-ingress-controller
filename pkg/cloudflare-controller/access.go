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

// desiredAccessApp holds the desired state for an Access Application on a given hostname.
// Group fields contain names from annotations; IDs are resolved at reconcile time.
type desiredAccessApp struct {
	AllowedGroupIDs []string
	DeniedGroupIDs  []string
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
		if len(e.AllowedAccessGroupIDs) == 0 && len(e.DeniedAccessGroupIDs) == 0 {
			continue
		}

		existing := result[e.Hostname]
		existing.AllowedGroupIDs = unionStrings(existing.AllowedGroupIDs, e.AllowedAccessGroupIDs)
		existing.DeniedGroupIDs = unionStrings(existing.DeniedGroupIDs, e.DeniedAccessGroupIDs)
		result[e.Hostname] = existing
	}

	// Remove entries with no allowed groups — deny-only doesn't make sense without an allow
	for hostname, app := range result {
		if len(app.AllowedGroupIDs) == 0 {
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
			t.logger.V(3).Info("update access application policies", "hostname", hostname, "app-id", existingApp.ID)
			err := t.updateAccessAppPolicies(ctx, rc, existingApp, want)
			if err != nil {
				return errors.Wrapf(err, "update access application policies for %s", hostname)
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

// createAccessApp creates an Access Application and its allow (and optionally deny) policies.
func (t *TunnelClient) createAccessApp(ctx context.Context, rc *cloudflare.ResourceContainer, hostname string, want desiredAccessApp) error {
	app, err := t.cfClient.CreateAccessApplication(ctx, rc, cloudflare.CreateAccessApplicationParams{
		Name:            accessAppName(t.tunnelName, hostname),
		Domain:          hostname,
		Type:            cloudflare.SelfHosted,
		SessionDuration: "24h",
	})
	if err != nil {
		return errors.Wrap(err, "create access application")
	}

	// Create allow policy (precedence 1 = evaluated first by default, but deny at precedence 1 takes priority)
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

	// Create deny policy if deny groups are specified (higher precedence = evaluated first)
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

// updateAccessAppPolicies updates the allow and deny policies on an existing Access Application.
func (t *TunnelClient) updateAccessAppPolicies(ctx context.Context, rc *cloudflare.ResourceContainer, app cloudflare.AccessApplication, want desiredAccessApp) error {
	policies, _, err := t.cfClient.ListAccessPolicies(ctx, rc, cloudflare.ListAccessPoliciesParams{
		ApplicationID: app.ID,
	})
	if err != nil {
		return errors.Wrap(err, "list access policies")
	}

	var allowPolicy *cloudflare.AccessPolicy
	var denyPolicy *cloudflare.AccessPolicy
	for i := range policies {
		switch policies[i].Decision {
		case "allow":
			allowPolicy = &policies[i]
		case "deny":
			denyPolicy = &policies[i]
		}
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
		// No deny groups desired but a deny policy exists — remove it
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

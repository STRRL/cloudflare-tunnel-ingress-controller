Feature: Uninstall cleanup
  Uninstalling the Helm release must not leave managed workloads running in
  the cluster. The connector Deployment and the tunnel token Secret carry an
  owner reference to the controller Deployment, so Kubernetes garbage
  collection removes them together with the release.

  The Cloudflare tunnel itself is intentionally kept: tunnels are addressed
  by name and reused on reinstall, external resources are never touched
  during uninstall.

  The scenario installs a second controller release with its own tunnel and
  ingress class, so the shared suite release keeps serving other scenarios.

  Scenario: helm uninstall garbage collects the managed connector
    Given a dedicated controller release with its own tunnel is installed
    And the dedicated connector deployment eventually becomes available
    When the dedicated release is uninstalled
    Then the dedicated connector resources are gone from the cluster
    And the dedicated tunnel still exists on Cloudflare

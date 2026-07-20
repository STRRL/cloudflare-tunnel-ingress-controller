Feature: HTTP exposure through Cloudflare Tunnel
  The controller turns an Ingress with the cloudflare-tunnel class into a
  publicly reachable hostname served through the Cloudflare edge.

  Scenario: Expose the Kubernetes dashboard over HTTPS
    Given the kubernetes dashboard addon is enabled
    When an ingress exposes the dashboard service at a generated hostname
    Then the ingress status eventually reports a tunnel hostname
    And the generated hostname eventually serves a page containing "Kubernetes Dashboard"

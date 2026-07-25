Feature: Origin request settings
  Ingress annotations map to cloudflared originRequest settings on the
  generated tunnel ingress rule. The scenario verifies the wiring end to end
  by reading the tunnel configuration back from the Cloudflare API; per field
  parsing and mapping are covered by unit tests on both transforms.

  Scenario: annotations are applied to the tunnel ingress rule
    Given an http echo service "echo-origin" replying "origin-request-e2e" is deployed
    And an ingress with origin request annotations exposes "echo-origin" at a generated hostname
    Then the tunnel ingress rule eventually carries the expected origin request settings

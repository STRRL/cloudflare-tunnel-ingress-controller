Feature: Wildcard routing priority
  An exact hostname must win over a wildcard rule no matter the order the
  rules appear in the Ingress spec, and any other hostname under the same
  domain falls back to the wildcard backend.

  Scenario: Exact host wins over a wildcard rule listed first
    Given an http echo service "e2e-echo-exact" replying "exact-backend" is deployed
    And an http echo service "e2e-echo-fallback" replying "wildcard-backend" is deployed
    When an ingress routes a wildcard hostname to "e2e-echo-fallback" before an exact hostname to "e2e-echo-exact"
    Then the exact hostname eventually serves "exact-backend"
    And any other hostname under the wildcard eventually serves "wildcard-backend"

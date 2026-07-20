Feature: Managed DNS record lifecycle
  The controller owns the CNAME and the ownership TXT record for every
  exposed hostname, and actively relinquishes them when DNS management is
  disabled on the Ingress.

  Scenario: Relinquish managed DNS records when management is disabled
    Given an http echo service "e2e-echo-dns" replying "dns-managed-backend" is deployed
    When an ingress exposes "e2e-echo-dns" at a generated hostname
    Then the controller eventually creates the CNAME and ownership TXT records
    When DNS management is disabled on the ingress via annotation
    Then the controller eventually deletes the CNAME and ownership TXT records

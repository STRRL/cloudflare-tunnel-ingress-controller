Feature: Managed DNS record lifecycle
  The controller creates a CNAME and an ownership TXT record for every
  exposed hostname, and deletes them again when DNS management is turned
  off on the Ingress.

  Scenario: Delete managed DNS records when management is turned off
    Given an http echo service "e2e-echo-dns" replying "dns-managed-backend" is deployed
    When an ingress exposes "e2e-echo-dns" at a generated hostname
    Then the controller eventually creates the CNAME and ownership TXT records
    When DNS management is turned off on the ingress via annotation
    Then the controller eventually deletes the CNAME and ownership TXT records

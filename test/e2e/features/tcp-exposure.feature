Feature: TCP exposure through Cloudflare Tunnel
  Non HTTP backends are exposed with the backend-protocol annotation and
  reached through the tunnel edge with cloudflared access tcp.

  Scenario: Expose redis with backend protocol tcp
    Given a redis instance "e2e-redis" is deployed
    When an ingress with backend protocol "tcp" exposes "e2e-redis" on port 6379 at a generated hostname
    Then the ingress status eventually reports a tunnel hostname
    And redis eventually answers PING through the tunnel

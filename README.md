# Cloudflare Tunnel Ingress Controller

TLDR; This project simplifies exposing Kubernetes services to the internet easily and securely using Cloudflare Tunnel.

## Get Started

- You should have a Cloudflare account and a domain configured on Cloudflare.
- Create a Cloudflare API token with the following:
  - Zone:Zone:Read
  - Zone:DNS:Edit
  - Account:Cloudflare Tunnel:Edit
- Fetch the Account ID from the Cloudflare dashboard, follow the instructions [here](https://developers.cloudflare.com/fundamentals/get-started/basic-tasks/find-account-and-zone-ids/).
- Install with Helm:

```bash
```

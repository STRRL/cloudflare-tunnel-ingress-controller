---
title: Cloudflare Credentials
description: Provide the API token, account ID, and tunnel name required by the controller.
---

The controller reads Cloudflare credentials from a Kubernetes secret named `cloudflare-api` in its namespace. The Helm chart can create this secret directly from the values you supply or consume an existing secret.

## Secret keys

| Key                      | Purpose                                                                                                        |
| ------------------------ | -------------------------------------------------------------------------------------------------------------- |
| `api-token`              | Cloudflare API token with `Account.Cloudflare Tunnel:Edit`, `Zone.DNS:Edit`, and `Zone.Zone:Read` permissions. |
| `cloudflare-account-id`  | Account identifier that owns the tunnel.                                                                       |
| `cloudflare-tunnel-name` | Friendly tunnel name created or reused by the controller.                                                      |

To let Helm create the secret, pass the values during installation:

```bash
helm upgrade --install cloudflare-tunnel-ingress-controller \
  strrl.dev/cloudflare-tunnel-ingress-controller \
  --set cloudflare.apiToken="<CLOUDFLARE_API_TOKEN>" \
  --set cloudflare.accountId="<CLOUDFLARE_ACCOUNT_ID>" \
  --set cloudflare.tunnelName="<TUNNEL_NAME>"
```

## Using an existing secret

If you manage credentials outside Helm (for example with External Secrets or Vault), point the chart at your secret:

```yaml
cloudflare:
  secretRef:
    name: cloudflare-external-secret
    accountIDKey: account_id
    tunnelNameKey: tunnel_name
    apiTokenKey: api_token
```

For example, if your API token is `XXXXXXXX`, account ID is `YYYYYY`, and tunnel name is `ZZZZZ`, you would first create the secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cloudflare-external-secret
  namespace: cloudflare-tunnel-system
type: Opaque
stringData:
  api_token: "XXXXXXXX"
  account_id: "YYYYYY"
  tunnel_name: "ZZZZZ"
```

Then configure the Helm chart to reference it:

```yaml
cloudflare:
  secretRef:
    name: cloudflare-external-secret
    accountIDKey: account_id
    tunnelNameKey: tunnel_name
    apiTokenKey: api_token
```

The controller only needs read access to these values. The chart injects them into the controller pod as environment variables, and the controller reads them once at startup, so rotating the secret in place does not refresh a running controller. After updating the secret, restart the controller to pick up the new credentials:

```bash
kubectl rollout restart deployment cloudflare-tunnel-ingress-controller \
  -n cloudflare-tunnel-ingress-controller
```

---
title: Cloudflare Credentials
description: Provide the API token, account ID, and tunnel name required by the controller.
---

The controller reads Cloudflare credentials from a Kubernetes secret named `cloudflare-api` in its namespace. The Helm chart can create this secret directly from the values you supply or consume an existing secret.

## Create an API token

1. Sign in to the Cloudflare dashboard.
2. Open this token creation template:

```text
https://dash.cloudflare.com/profile/api-tokens?permissionGroupKeys=[{"key":"zone","type":"read"},{"key":"dns","type":"edit"},{"key":"argotunnel","type":"edit"}]&name=Cloudflare%20Tunnel%20Ingress%20Controller&accountId=*&zoneId=all
```

3. Confirm that the token has all three required permission scopes:
   1. `Zone:Zone:Read`
   2. `Zone:DNS:Edit`
   3. `Account:Cloudflare Tunnel:Edit`

4. Review the account and zone resources covered by the token.
5. Create the token and copy its value. Store this value under the `api-token` Secret key described below.

## Secret keys

| Key                      | Purpose                                                                      |
| ------------------------ | ---------------------------------------------------------------------------- |
| `api-token`              | Cloudflare API token with the three required permission scopes listed above. |
| `cloudflare-account-id`  | Account identifier that owns the tunnel.                                     |
| `cloudflare-tunnel-name` | Friendly tunnel name created or reused by the controller.                    |

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

The controller only needs read access to these values. The chart injects them into the controller pod as environment variables, and the controller reads them once at startup.

Follow [Rotate Cloudflare credentials](/how-to/rotate-cloudflare-credentials/) to replace a token and restart the workloads that consume it.

# Cloudflare Tunnel Ingress Controller

TLDR; This project simplifies exposing Kubernetes services to the internet easily and securely using Cloudflare Tunnel.

We'd love to hear how the project works for you—please take a minute to fill out the short community survey: [cloudflare-tunnel-ingress-controller feedback](https://forms.gle/GqZomrLdb1vzyVJDA).

## Prerequisites

To use the Cloudflare Tunnel Ingress Controller, you need to have a Cloudflare account and a domain configured on Cloudflare. You also need a Cloudflare API token with `Zone:Zone:Read`, `Zone:DNS:Edit`, and `Account:Cloudflare Tunnel:Edit` permissions, as described in [Cloudflare credentials](https://tunnel.strrl.dev/reference/cloudflare-credentials/).

Additionally, you need to fetch the Account ID from the Cloudflare dashboard.

Finally, you need to have a Kubernetes cluster with public Internet access.

## Get Started

Install the controller with Helm:

```bash
helm upgrade --install --wait \
  cloudflare-tunnel-ingress-controller \
  cloudflare-tunnel-ingress-controller \
  --repo https://helm.strrl.dev \
  --namespace cloudflare-tunnel-ingress-controller \
  --create-namespace \
  --set cloudflare.apiToken="<CLOUDFLARE_API_TOKEN>" \
  --set cloudflare.accountId="<CLOUDFLARE_ACCOUNT_ID>" \
  --set cloudflare.tunnelName="<TUNNEL_NAME>"
```

Follow the [quickstart](https://tunnel.strrl.dev/guides/quickstart/) to publish your first Ingress.

## Configuration

The controller supports CLI flags and matching environment variables. See [controller configuration](https://tunnel.strrl.dev/reference/controller-configuration/) for the complete list and defaults.

## Alternative

There is also an awesome project which could integrate with Cloudflare Tunnel as CRD, check it out [adyanth/cloudflare-operator](https://github.com/adyanth/cloudflare-operator)!

## Contributing

Contributions are welcome! If you find a bug or have a feature request, please open an issue or submit a pull request.
To speed up local development and testing, you can use [Act](https://github.com/nektos/act) to run GitHub Actions workflows locally. For example, to run unit tests using the same workflow as CI:

```bash
act -W .github/workflows/unit-test.yaml
```

You can view all available workflows [here](https://github.com/STRRL/cloudflare-tunnel-ingress-controller/tree/master/.github/workflows).

### Local Development with Skaffold

To run the project locally, Skaffold is integrated into the Makefile. First, install Skaffold by following the instructions at [skaffold.dev](https://skaffold.dev).

Then, start the development environment with:

```bash
skaffold dev
```

> **Important:** The controller pod expects a Kubernetes `Secret` named `cloudflare-api` with credentials to authenticate with Cloudflare.
> If this secret is not present, the pod will fail with:
> `CreateContainerConfigError: secret "cloudflare-api" not found`.

There are two ways to provide the required secret:

1. **Manually create it with kubectl**:

   ```bash
   kubectl create secret generic cloudflare-api \
     -n cloudflare-tunnel-ingress-controller-dev \
     --from-literal=api-token='your_api_token' \
     --from-literal=cloudflare-account-id='your_account_id' \
     --from-literal=cloudflare-tunnel-name='your_tunnel_name'
   ```

2. **(Recommended for local development)** Copy the example file [`hack/dev/cloudflare-api.example.yaml`](./hack/dev/cloudflare-api.example.yaml) to `hack/dev/cloudflare-api.yaml` and fill in your own credentials:

```bash
cp hack/dev/cloudflare-api.example.yaml hack/dev/cloudflare-api.yaml
```

This file is included in `.gitignore`, so your secrets will not be committed to version control.
When you run `skaffold dev`, the secret defined in `cloudflare-api.yaml` will be automatically applied to your cluster.

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
    name: cloudflare-api
    namespace: cloudflare-tunnel-ingress-controller-dev
   stringData:
     api-token: "<your_api_token>"
     cloudflare-account-id: "<your_account_id>"
     cloudflare-tunnel-name: "<your_tunnel_name>"
   ```


## License

This project is licensed under the MIT License. See the LICENSE file for details.

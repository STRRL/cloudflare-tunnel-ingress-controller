# Cloudflare Tunnel Ingress Controller

TLDR; This project simplifies exposing Kubernetes services to the internet easily and securely using Cloudflare Tunnel.

## Prerequisites

To use the Cloudflare Tunnel Ingress Controller, you need to have a Cloudflare account and a domain configured on Cloudflare. You also need to create a Cloudflare API token with the following permissions: `Zone:Zone:Read`, `Zone:DNS:Edit`, and `Account:Cloudflare Tunnel:Edit`.

Additionally, you need to fetch the Account ID from the Cloudflare dashboard.

Finally, you need to have a Kubernetes cluster with public Internet access.

## Get Started

Take a look on this video to see how smoothly and easily it works:

[![Less than 4 minutes! Bootstrap a Kubernetes Cluster and Expose Kubernetes Dashboard to the Internet.](https://markdown-videos.vercel.app/youtube/e-ARlEnS4zQ)](http://www.youtube.com/watch?v=e-ARlEnS4zQ "Less than 4 minutes! Bootstrap a Kubernetes Cluster and Expose Kubernetes Dashboard to the Internet.")

Want to DIY? The following instructions would help your bootstrap a minikube Kubernetes Cluster, then expose the Kubernetes Dashboard to the internet via Cloudflare Tunnel Ingress Controller.

- You should have a Cloudflare account and a domain configured on Cloudflare.
- Create a Cloudflare API token with the following:
  - `Zone:Zone:Read`
  - `Zone:DNS:Edit`
  - `Account:Cloudflare Tunnel:Edit`
- Fetch the Account ID from the Cloudflare dashboard, follow the instructions [here](https://developers.cloudflare.com/fundamentals/get-started/basic-tasks/find-account-and-zone-ids/).
- Bootstrap a minikube cluster

```bash
minikube start
```

- Add Helm Repository;

```bash
helm repo add strrl.dev https://helm.strrl.dev
helm repo update
```

- Install with Helm:

```bash
helm upgrade --install --wait \
  -n cloudflare-tunnel-ingress-controller --create-namespace \
  cloudflare-tunnel-ingress-controller \
  strrl.dev/cloudflare-tunnel-ingress-controller \
  --set=cloudflare.apiToken="<cloudflare-api-token>",cloudflare.accountId="<cloudflare-account-id>",cloudflare.tunnelName="<your-favorite-tunnel-name>" 
```

> if the tunnel does not exist, controller will create it for you.

- Then enable some awesome features in minikube, like kubernetes-dashboard:

```bash
minikube addons enable dashboard
minikube addons enable metrics-server
```

- Then expose the dashboard to the internet by creating an `Ingress`:

```bash
kubectl -n kubernetes-dashboard \
  create ingress dashboard-via-cf-tunnel \
  --rule="<your-favorite-domain>/*=kubernetes-dashboard:80"\
  --class cloudflare-tunnel
```

> for example, I would use `dash.strrl.cloud` as my favorite domain here.

- At last, access the dashboard via the domain you just created:

![dash.strrl.cloud](./static/dash.strrl.cloud.png)

- Done! Enjoy! 🎉

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

2. **(Recommended for local development)** Create a called file [`hack/dev/cloudflare-api.yaml`](./hack/dev/cloudflare-api.yaml) with your credentials (no need to copy this secret, just fill it and it will be applied)

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: cloudflare-api
   stringData:
     api-token: "<your_api_token>"
     cloudflare-account-id: "<your_account_id>"
     cloudflare-tunnel-name: "<your_tunnel_name>"
   ```


## License

This project is licensed under the MIT License. See the LICENSE file for details.

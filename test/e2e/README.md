# E2E Tests

These tests spin up a temporary minikube cluster, deploy the latest built controller, and expose kubernetes-dashboard through Cloudflare Tunnel. The process creates real DNS records and Cloudflare Tunnel configurations, so be sure to use a dedicated test domain/tunnel.

## Prerequisites
- `docker` and `minikube` are installed and can access local container images
- `helm` is available (used to install the controller via Helm Chart)
- `.env.e2e` is located in the repository root, containing:
  - `CLOUDFLARE_API_TOKEN`
  - `CLOUDFLARE_ACCOUNT_ID`
  - `CLOUDFLARE_TUNNEL_NAME`
  - `E2E_BASE_DOMAIN`: the root domain under a Cloudflare Zone (e.g., `strrl.cloud`), the test will generate unique subdomains based on it
- (Optional) Chrome / Chromium installed locally for capturing dashboard screenshots; if missing, only a warning will be logged.

The `E2E_CONTROLLER_IMAGE` environment variable specifies the controller image used for testing, with a default value of `cloudflare-tunnel-ingress-controller:e2e`.
During test execution, random subdomains will be generated based on `E2E_BASE_DOMAIN` (e.g., `cf-dashboard-<timestamp>.strrl.cloud`), and Cloudflare DNS records and tunnel rules will be created accordingly.

## Execution
```bash
make e2e
```

`make e2e` will first build `E2E_CONTROLLER_IMAGE`, then run `go test ./test/e2e`. During the test:
1. Start a uniquely named minikube profile;
2. Validate the Cloudflare Token;
3. Install the controller via Helm Chart;
4. Enable `dashboard` and `metrics-server` addons, and create an Ingress;
5. Poll Cloudflare until the Dashboard is accessible via HTTPS;
6. If available, capture a screenshot of the Dashboard page and save it to `test/e2e/artifacts/`.

After the test completes, the temporary kubeconfig and minikube profile will be automatically deleted. If the run is interrupted, you can manually execute:
```bash
minikube delete -p <profile>
```

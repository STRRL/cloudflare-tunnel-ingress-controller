# E2E Tests

这些测试会拉起一个临时的 minikube 集群、部署最新构建的 controller，并通过 Cloudflare Tunnel 暴露 kubernetes-dashboard。流程会创建真实的 DNS 记录以及 Cloudflare Tunnel 配置，请务必使用专用的测试域名/隧道。

## 先决条件
- `docker` 与 `minikube` 已安装，并能访问到本地容器镜像
- `helm` 可用（安装 controller 使用 Helm Chart）
- `.env.e2e` 位于仓库根目录，包含：
  - `CLOUDFLARE_API_TOKEN`
  - `CLOUDFLARE_ACCOUNT_ID`
  - `CLOUDFLARE_TUNNEL_NAME`
  - `E2E_BASE_DOMAIN`：位于 Cloudflare Zone 下的根域名（例：`strrl.cloud`），测试会基于它生成唯一的子域
- （可选）本地安装 Chrome / Chromium，用于截取 dashboard 的截图；若缺失，仅会记录警告。

`E2E_CONTROLLER_IMAGE` 环境变量用于指定测试使用的 controller 镜像，默认值为 `cloudflare-tunnel-ingress-controller:e2e`。
测试运行过程中会基于 `E2E_BASE_DOMAIN` 生成随机子域（例如 `cf-dashboard-<timestamp>.strrl.cloud`），并据此创建 Cloudflare DNS 记录与隧道规则。

## 执行
```bash
make e2e
```

`make e2e` 会先构建 `E2E_CONTROLLER_IMAGE`，然后运行 `go test ./test/e2e`。测试过程中将：
1. 启动唯一命名的 minikube profile；
2. 校验 Cloudflare Token；
3. 通过 Helm Chart 安装 controller；
4. 启用 `dashboard` 与 `metrics-server` addons，并创建 Ingress；
5. 轮询 Cloudflare 直至 Dashboard 可通过 HTTPS 访问；
6. 若可用，抓取 Dashboard 页面截图，保存在 `test/e2e/artifacts/`。

测试结束后会自动删除临时 kubeconfig 和 minikube profile。若运行被中断，可手动执行：
```bash
minikube delete -p <profile>
```

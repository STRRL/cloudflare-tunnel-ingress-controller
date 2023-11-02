#

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square)

Ingress Controller based on Cloudflare Tunnel

The chart is under active development and may contain bugs/unfinished documentation. Any testing/contributions are welcome! :)

**Homepage:** <https://github.com/oliverbaehler/cloudflare-tunnel-ingress-controller>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| oliverbaehler | <oliverbaehler@hotmail.com> |  |

## Source Code

* <https://github.com/oliverbaehler/cloudflare-tunnel-ingress-controller>

# Major Changes

Major Changes to functions are documented with the version affected. **Before upgrading the dependency version, check this section out!**

| **Change** | **Chart Version** | **Description** | **Commits/PRs** |
| :--------- | :---------------- | :-------------- | :-------------- |
|||||

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Pod Affinity |
| anotations | object | `{}` | Deployment annotations |
| cloudflare.accountId | string | `""` | Cloudflare Account ID |
| cloudflare.apiToken | string | `""` | Cloudflare API Token |
| cloudflare.tunnelName | string | `""` | Cloudflare Tunnel Name |
| existingSecretName | string | `""` | Use an existing secret (Secret must contain `api-token`, `cloudflare-account-id` and  `cloudflare-tunnel-name` as keys) |
| fullnameOverride | string | `""` | Full name override |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy |
| image.repository | string | `"ghcr.io/oliverbaehler/cloudflare-tunnel-ingress-controller"` | Image repository |
| image.tag | string | `""` | Overrides the image tag whose default is the chart appVersion. |
| imagePullSecrets | list | `[]` | Image pull secrets |
| ingressClass.controllerValue | string | `"cloudflare-tunnel-ingress-controller"` | Ingress class controller |
| ingressClass.isDefaultClass | bool | `false` | Cluster default ingress class |
| ingressClass.name | string | `"cloudflare-tunnel"` | Ingress class name |
| labels | object | `{}` | Deployment  |
| nameOverride | string | `""` | Partial name override |
| nodeSelector | object | `{}` | Pod Node Selector |
| podAnnotations | object | `{}` | Additional Pod annotations |
| podLabels | object | `{}` | Additional Pod labels |
| podSecurityContext | object | `{"enabled":true,"fsGroup":65532,"runAsNonRoot":true}` | Pod Security Context |
| replicaCount | int | `1` | Replicas |
| resources | object | `{"limits":{"cpu":"100m","memory":"128Mi"},"requests":{"cpu":"100m","memory":"128Mi"}}` | Container resources |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"enabled":true,"readOnlyRootFilesystem":true,"runAsGroup":65532,"runAsUser":65532}` | Container SecurityContext |
| serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| tolerations | list | `[]` | Pod Tolerations |
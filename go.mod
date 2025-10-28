module github.com/STRRL/cloudflare-tunnel-ingress-controller

go 1.24.0

toolchain go1.24.4

require (
	github.com/chromedp/chromedp v0.14.2
	github.com/cloudflare/cloudflare-go v0.116.0
	github.com/go-logr/logr v1.4.3
	github.com/go-logr/stdr v1.2.2
	github.com/joho/godotenv v1.5.1
	github.com/onsi/ginkgo/v2 v2.27.1
	github.com/onsi/gomega v1.38.2
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.10.1
	github.com/stretchr/testify v1.11.1
	k8s.io/api v0.34.1
	k8s.io/apimachinery v0.34.1
	k8s.io/client-go v0.34.1
	k8s.io/utils v0.0.0-20250604170112-4c0f3b243397
	sigs.k8s.io/controller-runtime v0.22.3
	sigs.k8s.io/yaml v1.6.0
)

require (
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chromedp/cdproto v0.0.0-20250724212937-08a3db8b4327 // indirect
	github.com/chromedp/sysutil v1.1.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-json-experiment/json v0.0.0-20250725192818-e39067aee2d2 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/gobwas/ws v1.4.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/pprof v0.0.0-20250403155104-27863c87afa6 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.22.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.27.0 // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/oauth2 v0.27.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/term v0.34.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	golang.org/x/tools v0.36.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/protobuf v1.36.7 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.34.1 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250710124328-f3f2b991d03b // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0 // indirect
)

replace k8s.io/api => k8s.io/api v0.34.1

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.34.1

replace k8s.io/apimachinery => k8s.io/apimachinery v0.34.1

replace k8s.io/apiserver => k8s.io/apiserver v0.34.1

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.34.1

replace k8s.io/client-go => k8s.io/client-go v0.34.1

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.34.1

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.34.1

replace k8s.io/code-generator => k8s.io/code-generator v0.34.1

replace k8s.io/component-base => k8s.io/component-base v0.34.1

replace k8s.io/component-helpers => k8s.io/component-helpers v0.34.1

replace k8s.io/controller-manager => k8s.io/controller-manager v0.34.1

replace k8s.io/cri-api => k8s.io/cri-api v0.34.1

replace k8s.io/cri-client => k8s.io/cri-client v0.34.1

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.34.1

replace k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.34.1

replace k8s.io/endpointslice => k8s.io/endpointslice v0.34.1

replace k8s.io/externaljwt => k8s.io/externaljwt v0.34.1

replace k8s.io/kms => k8s.io/kms v0.34.1

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.34.1

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.34.1

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.34.1

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.34.1

replace k8s.io/kubectl => k8s.io/kubectl v0.34.1

replace k8s.io/kubelet => k8s.io/kubelet v0.34.1

replace k8s.io/metrics => k8s.io/metrics v0.34.1

replace k8s.io/mount-utils => k8s.io/mount-utils v0.34.1

replace k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.34.1

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.34.1

replace k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.34.1

replace k8s.io/sample-controller => k8s.io/sample-controller v0.34.1

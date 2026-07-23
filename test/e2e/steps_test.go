package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/cucumber/godog"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const redisLocalAddr = "127.0.0.1:16379"

type namespacedName struct {
	namespace string
	name      string
}

// world carries the per scenario state. A pointer travels through the step
// context, scenarios run serially so no synchronisation is needed.
type world struct {
	hostname         string
	exactHostname    string
	probeHostname    string
	wildcardHostname string
	ingress          *namespacedName
	workloads        []namespacedName
	accessCancel     context.CancelFunc
	accessCmd        *exec.Cmd
	cfAPI            *cloudflare.API
	zoneID           string
}

type worldCtxKey struct{}

func worldFromContext(ctx context.Context) *world {
	w, _ := ctx.Value(worldCtxKey{}).(*world)
	return w
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		return context.WithValue(ctx, worldCtxKey{}, &world{}), nil
	})

	ctx.After(func(ctx context.Context, sc *godog.Scenario, scErr error) (context.Context, error) {
		w := worldFromContext(ctx)
		if w == nil {
			return ctx, nil
		}

		if w.accessCancel != nil {
			w.accessCancel()
			if w.accessCmd != nil {
				_ = w.accessCmd.Wait()
			}
		}

		if w.ingress != nil {
			_ = kubeClient.NetworkingV1().Ingresses(w.ingress.namespace).Delete(context.Background(), w.ingress.name, metav1.DeleteOptions{})
			// give the controller a moment to observe the deletion and drop
			// the tunnel rule and DNS records before the workloads disappear
			_ = waitFor(fmt.Sprintf("ingress %s/%s deleted", w.ingress.namespace, w.ingress.name), 2*time.Minute, 5*time.Second, func() error {
				_, err := kubeClient.NetworkingV1().Ingresses(w.ingress.namespace).Get(context.Background(), w.ingress.name, metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("ingress still present")
				}
				return nil
			})
		}

		for _, workload := range w.workloads {
			_ = kubeClient.AppsV1().Deployments(workload.namespace).Delete(context.Background(), workload.name, metav1.DeleteOptions{})
			_ = kubeClient.CoreV1().Services(workload.namespace).Delete(context.Background(), workload.name, metav1.DeleteOptions{})
		}

		return ctx, nil
	})

	ctx.Step(`^the kubernetes dashboard addon is enabled$`, theDashboardAddonIsEnabled)
	ctx.Step(`^an ingress exposes the dashboard service at a generated hostname$`, anIngressExposesTheDashboard)
	ctx.Step(`^the ingress status eventually reports a tunnel hostname$`, theIngressStatusReportsTunnelHostname)
	ctx.Step(`^the generated hostname eventually serves a page containing "([^"]*)"$`, theHostnameServesPageContaining)

	ctx.Step(`^a redis instance "([^"]*)" is deployed$`, aRedisInstanceIsDeployed)
	ctx.Step(`^an ingress with backend protocol "([^"]*)" exposes "([^"]*)" on port (\d+) at a generated hostname$`, anIngressWithBackendProtocolExposes)
	ctx.Step(`^redis eventually answers PING through the tunnel$`, redisAnswersPingThroughTunnel)

	ctx.Step(`^an http echo service "([^"]*)" replying "([^"]*)" is deployed$`, anHTTPEchoServiceIsDeployed)
	ctx.Step(`^an ingress routes a wildcard hostname to "([^"]*)" before an exact hostname to "([^"]*)"$`, anIngressRoutesWildcardBeforeExact)
	ctx.Step(`^the exact hostname eventually serves "([^"]*)"$`, theExactHostnameServes)
	ctx.Step(`^any other hostname under the wildcard eventually serves "([^"]*)"$`, theProbeHostnameServes)

	ctx.Step(`^an ingress exposes "([^"]*)" at a generated hostname$`, anIngressExposesEchoService)
	ctx.Step(`^the controller eventually creates the CNAME and ownership TXT records$`, theControllerCreatesDNSRecords)
	ctx.Step(`^DNS management is turned off on the ingress via annotation$`, dnsManagementIsDisabled)
	ctx.Step(`^the controller eventually deletes the CNAME and ownership TXT records$`, theControllerDeletesDNSRecords)
}

func theDashboardAddonIsEnabled(ctx context.Context) error {
	for _, addon := range []string{"dashboard", "metrics-server"} {
		enableCtx, cancel := context.WithTimeout(suiteCtx, 5*time.Minute)
		err := enableMinikubeAddon(enableCtx, minikubeProfile, addon)
		cancel()
		if err != nil {
			return err
		}
	}

	return waitFor("dashboard deployment ready", 10*time.Minute, 10*time.Second, func() error {
		deployment, err := kubeClient.AppsV1().Deployments("kubernetes-dashboard").Get(context.Background(), "kubernetes-dashboard", metav1.GetOptions{})
		if err != nil {
			return err
		}
		if !isDeploymentAvailable(*deployment) {
			return fmt.Errorf("dashboard deployment has %d available replicas", deployment.Status.AvailableReplicas)
		}
		return nil
	})
}

func anIngressExposesTheDashboard(ctx context.Context) error {
	w := worldFromContext(ctx)

	var servicePort int32
	if err := waitFor("dashboard service ports", 2*time.Minute, 5*time.Second, func() error {
		svc, err := kubeClient.CoreV1().Services("kubernetes-dashboard").Get(context.Background(), "kubernetes-dashboard", metav1.GetOptions{})
		if err != nil {
			return err
		}
		if len(svc.Spec.Ports) == 0 {
			return fmt.Errorf("dashboard service has no ports yet")
		}
		servicePort = svc.Spec.Ports[0].Port
		return nil
	}); err != nil {
		return err
	}

	hostname, err := buildTestHostname("cf-dashboard", baseDomain)
	if err != nil {
		return err
	}
	w.hostname = hostname
	_, _ = fmt.Fprintf(logOut, "using dashboard hostname %s\n", hostname)

	return createScenarioIngress(w, namespacedName{namespace: "kubernetes-dashboard", name: "dashboard-via-cloudflare"}, nil, []networkingv1.IngressRule{
		serviceIngressRule(hostname, "kubernetes-dashboard", servicePort),
	})
}

func theIngressStatusReportsTunnelHostname(ctx context.Context) error {
	w := worldFromContext(ctx)
	if w.ingress == nil {
		return fmt.Errorf("no ingress created in this scenario")
	}

	return waitFor("ingress status hostname", 10*time.Minute, 10*time.Second, func() error {
		current, err := kubeClient.NetworkingV1().Ingresses(w.ingress.namespace).Get(context.Background(), w.ingress.name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if len(current.Status.LoadBalancer.Ingress) == 0 {
			return fmt.Errorf("ingress status has no entries yet")
		}
		for _, lb := range current.Status.LoadBalancer.Ingress {
			if strings.TrimSpace(lb.Hostname) != "" {
				return nil
			}
		}
		return fmt.Errorf("ingress status entries have empty hostnames")
	})
}

func theHostnameServesPageContaining(ctx context.Context, marker string) error {
	w := worldFromContext(ctx)
	url := fmt.Sprintf("https://%s/", w.hostname)

	if err := waitFor("cloudflare https availability", 15*time.Minute, 20*time.Second, func() error {
		return expectHTTPBody(edgeHTTPClient(), url, marker)
	}); err != nil {
		return err
	}

	if path, err := captureDashboardScreenshot(context.Background(), url); err != nil {
		_, _ = fmt.Fprintf(logOut, "warning: failed to capture dashboard screenshot: %v\n", err)
	} else {
		_, _ = fmt.Fprintf(logOut, "dashboard screenshot saved to %s\n", path)
	}
	return nil
}

func aRedisInstanceIsDeployed(ctx context.Context, name string) error {
	w := worldFromContext(ctx)
	if err := createRedis("default", name); err != nil {
		return err
	}
	w.workloads = append(w.workloads, namespacedName{namespace: "default", name: name})

	return waitFor("redis deployment ready", 5*time.Minute, 5*time.Second, func() error {
		deployment, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if !isDeploymentAvailable(*deployment) {
			return fmt.Errorf("redis deployment has %d available replicas", deployment.Status.AvailableReplicas)
		}
		return nil
	})
}

func anIngressWithBackendProtocolExposes(ctx context.Context, protocol string, serviceName string, port int) error {
	w := worldFromContext(ctx)

	hostname, err := buildTestHostname("cf-tcp", baseDomain)
	if err != nil {
		return err
	}
	w.hostname = hostname
	_, _ = fmt.Fprintf(logOut, "using tcp hostname %s\n", hostname)

	annotations := map[string]string{
		"cloudflare-tunnel-ingress-controller.strrl.dev/backend-protocol": protocol,
	}
	return createScenarioIngress(w, namespacedName{namespace: "default", name: "redis-via-cloudflare-tcp"}, annotations, []networkingv1.IngressRule{
		serviceIngressRule(hostname, serviceName, int32(port)),
	})
}

func redisAnswersPingThroughTunnel(ctx context.Context) error {
	w := worldFromContext(ctx)

	accessCtx, cancel := context.WithCancel(suiteCtx)
	accessCmd := exec.CommandContext(accessCtx, "cloudflared", "access", "tcp", "--hostname", w.hostname, "--url", redisLocalAddr)
	accessCmd.Stdout = logOut
	accessCmd.Stderr = logOut
	if err := accessCmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start cloudflared access tcp: %w", err)
	}
	w.accessCancel = cancel
	w.accessCmd = accessCmd

	return waitFor("redis PING via tunnel", 15*time.Minute, 20*time.Second, func() error {
		return redisPing(redisLocalAddr)
	})
}

func anHTTPEchoServiceIsDeployed(ctx context.Context, name string, reply string) error {
	w := worldFromContext(ctx)
	if err := createHTTPEcho("default", name, reply); err != nil {
		return err
	}
	w.workloads = append(w.workloads, namespacedName{namespace: "default", name: name})

	return waitFor(fmt.Sprintf("%s deployment ready", name), 5*time.Minute, 5*time.Second, func() error {
		deployment, err := kubeClient.AppsV1().Deployments("default").Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if !isDeploymentAvailable(*deployment) {
			return fmt.Errorf("%s deployment has %d available replicas", name, deployment.Status.AvailableReplicas)
		}
		return nil
	})
}

func anIngressRoutesWildcardBeforeExact(ctx context.Context, fallbackService string, exactService string) error {
	w := worldFromContext(ctx)

	// all hosts stay one label below the base domain, Universal SSL only
	// covers the zone apex and first-level subdomains, deeper names need
	// paid certificates and would fail the TLS handshake in this test
	exactHostname, err := buildTestHostname("cf-wc-exact", baseDomain)
	if err != nil {
		return err
	}
	probeHostname, err := buildTestHostname("cf-wc-probe", baseDomain)
	if err != nil {
		return err
	}
	w.exactHostname = exactHostname
	w.probeHostname = probeHostname
	w.wildcardHostname = "*." + strings.SplitN(exactHostname, ".", 2)[1]

	// the wildcard rule intentionally comes first in the spec, rule order
	// must not affect routing priority
	return createScenarioIngress(w, namespacedName{namespace: "default", name: "wildcard-routing-via-cloudflare"}, nil, []networkingv1.IngressRule{
		serviceIngressRule(w.wildcardHostname, fallbackService, 80),
		serviceIngressRule(exactHostname, exactService, 80),
	})
}

func theExactHostnameServes(ctx context.Context, marker string) error {
	w := worldFromContext(ctx)
	return waitFor("exact host routing", 15*time.Minute, 20*time.Second, func() error {
		return expectHTTPBody(edgeHTTPClient(), fmt.Sprintf("https://%s/", w.exactHostname), marker)
	})
}

func theProbeHostnameServes(ctx context.Context, marker string) error {
	w := worldFromContext(ctx)
	return waitFor("wildcard fallback routing", 15*time.Minute, 20*time.Second, func() error {
		return expectHTTPBody(edgeHTTPClient(), fmt.Sprintf("https://%s/", w.probeHostname), marker)
	})
}

func anIngressExposesEchoService(ctx context.Context, serviceName string) error {
	w := worldFromContext(ctx)

	hostname, err := buildTestHostname("cf-dns", baseDomain)
	if err != nil {
		return err
	}
	w.hostname = hostname
	_, _ = fmt.Fprintf(logOut, "using dns hostname %s\n", hostname)

	return createScenarioIngress(w, namespacedName{namespace: "default", name: "dns-managed-via-cloudflare"}, nil, []networkingv1.IngressRule{
		serviceIngressRule(hostname, serviceName, 80),
	})
}

func theControllerCreatesDNSRecords(ctx context.Context) error {
	w := worldFromContext(ctx)

	cfAPI, err := newCloudflareAPI()
	if err != nil {
		return err
	}
	zoneID, err := findZoneID(context.Background(), cfAPI, w.hostname)
	if err != nil {
		return err
	}
	w.cfAPI = cfAPI
	w.zoneID = zoneID

	txtName := "_ctic_managed." + w.hostname
	return waitFor("managed DNS records created", 10*time.Minute, 10*time.Second, func() error {
		cnameExists, err := dnsRecordExists(context.Background(), cfAPI, zoneID, "CNAME", w.hostname)
		if err != nil {
			return err
		}
		if !cnameExists {
			return fmt.Errorf("CNAME record for %s not created yet", w.hostname)
		}
		txtExists, err := dnsRecordExists(context.Background(), cfAPI, zoneID, "TXT", txtName)
		if err != nil {
			return err
		}
		if !txtExists {
			return fmt.Errorf("ownership TXT record for %s not created yet", w.hostname)
		}
		return nil
	})
}

func dnsManagementIsDisabled(ctx context.Context) error {
	w := worldFromContext(ctx)
	if w.ingress == nil {
		return fmt.Errorf("no ingress created in this scenario")
	}

	return waitFor("annotate ingress", 2*time.Minute, 5*time.Second, func() error {
		current, err := kubeClient.NetworkingV1().Ingresses(w.ingress.namespace).Get(context.Background(), w.ingress.name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Annotations == nil {
			current.Annotations = map[string]string{}
		}
		current.Annotations["cloudflare-tunnel-ingress-controller.strrl.dev/disable-dns-management"] = "true"
		_, err = kubeClient.NetworkingV1().Ingresses(w.ingress.namespace).Update(context.Background(), current, metav1.UpdateOptions{})
		return err
	})
}

func theControllerDeletesDNSRecords(ctx context.Context) error {
	w := worldFromContext(ctx)
	if w.cfAPI == nil {
		return fmt.Errorf("cloudflare api not initialised in this scenario")
	}

	txtName := "_ctic_managed." + w.hostname
	return waitFor("managed DNS records deleted", 10*time.Minute, 10*time.Second, func() error {
		cnameExists, err := dnsRecordExists(context.Background(), w.cfAPI, w.zoneID, "CNAME", w.hostname)
		if err != nil {
			return err
		}
		if cnameExists {
			return fmt.Errorf("CNAME record for %s still present", w.hostname)
		}
		txtExists, err := dnsRecordExists(context.Background(), w.cfAPI, w.zoneID, "TXT", txtName)
		if err != nil {
			return err
		}
		if txtExists {
			return fmt.Errorf("ownership TXT record for %s still present", w.hostname)
		}
		return nil
	})
}

func createScenarioIngress(w *world, ref namespacedName, annotations map[string]string, rules []networkingv1.IngressRule) error {
	_ = kubeClient.NetworkingV1().Ingresses(ref.namespace).Delete(context.Background(), ref.name, metav1.DeleteOptions{})

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ref.name,
			Namespace:   ref.namespace,
			Annotations: annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To("cloudflare-tunnel"),
			Rules:            rules,
		},
	}

	if _, err := kubeClient.NetworkingV1().Ingresses(ref.namespace).Create(context.Background(), ingress, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create ingress %s/%s: %w", ref.namespace, ref.name, err)
	}
	w.ingress = &ref
	return nil
}

func serviceIngressRule(host string, serviceName string, port int32) networkingv1.IngressRule {
	pathType := networkingv1.PathTypePrefix
	return networkingv1.IngressRule{
		Host: host,
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{
					{
						Path:     "/",
						PathType: &pathType,
						Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: serviceName,
								Port: networkingv1.ServiceBackendPort{Number: port},
							},
						},
					},
				},
			},
		},
	}
}

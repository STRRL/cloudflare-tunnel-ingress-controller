package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/gatewayproxy"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Each Gateway gets a proxy Deployment, a ClusterIP Service and a ConfigMap
// with its routing table, all in the controller namespace and named
// gateway-<hash of namespace/name>. They are linked to the Gateway by
// labels, Gateways live in other namespaces so owner references cannot be
// used.

const (
	// gatewayProxyLabelKey marks proxy resources, its value is the resource
	// name.
	gatewayProxyLabelKey = "strrl.dev/gateway-proxy"
	// gatewayProxySpecHashAnnotation lets updates skip when nothing changed.
	gatewayProxySpecHashAnnotation = "strrl.dev/gateway-proxy-spec-hash"
	gatewayNamespaceAnnotation     = "strrl.dev/gateway-namespace"
	gatewayNameAnnotation          = "strrl.dev/gateway-name"

	gatewayProxyPort       = 8080
	gatewayProxyHealthPort = 8081
)

// GatewayProxyConfig carries the settings of the proxy Deployments.
type GatewayProxyConfig struct {
	Image           string
	ImagePullPolicy string
	ServiceAccount  string
	// Owner binds the proxy resources to the controller Deployment, so they
	// are removed on uninstall. Nil leaves them unowned.
	Owner *metav1.OwnerReference
}

func gatewayProxyName(gateway *gatewayv1.Gateway) string {
	return "gateway-" + shortHash(gateway.Namespace+"/"+gateway.Name)
}

// gatewayProxyServiceTarget is the tunnel origin of a Gateway.
func gatewayProxyServiceTarget(gateway *gatewayv1.Gateway, namespace string, clusterDomain string) string {
	return fmt.Sprintf("http://%s.%s.svc.%s:%d", gatewayProxyName(gateway), namespace, clusterDomain, gatewayProxyPort)
}

func gatewayProxyObjectMeta(gateway *gatewayv1.Gateway, namespace string, config GatewayProxyConfig) metav1.ObjectMeta {
	name := gatewayProxyName(gateway)
	meta := metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    map[string]string{gatewayProxyLabelKey: name},
		Annotations: map[string]string{
			gatewayNamespaceAnnotation: gateway.Namespace,
			gatewayNameAnnotation:      gateway.Name,
		},
	}
	if config.Owner != nil {
		meta.OwnerReferences = []metav1.OwnerReference{*config.Owner}
	}
	return meta
}

func buildGatewayProxyDeployment(gateway *gatewayv1.Gateway, namespace string, config GatewayProxyConfig) *appsv1.Deployment {
	meta := gatewayProxyObjectMeta(gateway, namespace, config)
	name := meta.Name
	selector := map[string]string{gatewayProxyLabelKey: name}
	return &appsv1.Deployment{
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selector},
				Spec: corev1.PodSpec{
					ServiceAccountName: config.ServiceAccount,
					Containers: []corev1.Container{{
						Name:            "proxy",
						Image:           config.Image,
						ImagePullPolicy: corev1.PullPolicy(config.ImagePullPolicy),
						Command: []string{
							"cloudflare-tunnel-ingress-controller",
							"proxy",
							"--configmap-namespace=" + namespace,
							"--configmap-name=" + name,
							fmt.Sprintf("--listen-address=:%d", gatewayProxyPort),
							fmt.Sprintf("--health-address=:%d", gatewayProxyHealthPort),
						},
						Ports: []corev1.ContainerPort{
							{Name: "http", ContainerPort: gatewayProxyPort, Protocol: corev1.ProtocolTCP},
							{Name: "health", ContainerPort: gatewayProxyHealthPort, Protocol: corev1.ProtocolTCP},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{Path: "/readyz", Port: intstr.FromString("health")},
							},
							PeriodSeconds: 1,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("10m"),
								corev1.ResourceMemory: resource.MustParse("32Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
					}},
				},
			},
		},
	}
}

func buildGatewayProxyService(gateway *gatewayv1.Gateway, namespace string, config GatewayProxyConfig) *corev1.Service {
	meta := gatewayProxyObjectMeta(gateway, namespace, config)
	return &corev1.Service{
		ObjectMeta: meta,
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{gatewayProxyLabelKey: meta.Name},
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       gatewayProxyPort,
				TargetPort: intstr.FromString("http"),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}
}

// ensureGatewayProxy creates or updates the proxy resources of a Gateway and
// reports whether the proxy is ready to serve.
func ensureGatewayProxy(ctx context.Context, kubeClient client.Client, gateway *gatewayv1.Gateway, namespace string, config GatewayProxyConfig, table gatewayproxy.Table) (bool, error) {
	encoded, err := gatewayproxy.EncodeTable(table)
	if err != nil {
		return false, err
	}

	configMap := &corev1.ConfigMap{ObjectMeta: gatewayProxyObjectMeta(gateway, namespace, config)}
	configMap.Data = map[string]string{gatewayproxy.TableConfigMapKey: encoded}
	existingConfigMap := &corev1.ConfigMap{}
	err = kubeClient.Get(ctx, client.ObjectKeyFromObject(configMap), existingConfigMap)
	switch {
	case apierrors.IsNotFound(err):
		if err := kubeClient.Create(ctx, configMap); err != nil {
			return false, errors.Wrapf(err, "create configmap %s", configMap.Name)
		}
	case err != nil:
		return false, errors.Wrapf(err, "get configmap %s", configMap.Name)
	case !reflect.DeepEqual(existingConfigMap.Data, configMap.Data):
		existingConfigMap.Data = configMap.Data
		if err := kubeClient.Update(ctx, existingConfigMap); err != nil {
			return false, errors.Wrapf(err, "update configmap %s", configMap.Name)
		}
	}

	service := buildGatewayProxyService(gateway, namespace, config)
	existingService := &corev1.Service{}
	err = kubeClient.Get(ctx, client.ObjectKeyFromObject(service), existingService)
	switch {
	case apierrors.IsNotFound(err):
		if err := kubeClient.Create(ctx, service); err != nil {
			return false, errors.Wrapf(err, "create service %s", service.Name)
		}
	case err != nil:
		return false, errors.Wrapf(err, "get service %s", service.Name)
	}

	deployment := buildGatewayProxyDeployment(gateway, namespace, config)
	specJSON, err := json.Marshal(deployment.Spec)
	if err != nil {
		return false, errors.Wrap(err, "encode proxy deployment spec")
	}
	specHash := shortHash(string(specJSON))
	deployment.Annotations[gatewayProxySpecHashAnnotation] = specHash

	existingDeployment := &appsv1.Deployment{}
	err = kubeClient.Get(ctx, client.ObjectKeyFromObject(deployment), existingDeployment)
	switch {
	case apierrors.IsNotFound(err):
		if err := kubeClient.Create(ctx, deployment); err != nil {
			return false, errors.Wrapf(err, "create deployment %s", deployment.Name)
		}
		return false, nil
	case err != nil:
		return false, errors.Wrapf(err, "get deployment %s", deployment.Name)
	case existingDeployment.Annotations[gatewayProxySpecHashAnnotation] != specHash:
		existingDeployment.Annotations = deployment.Annotations
		existingDeployment.Spec = deployment.Spec
		if err := kubeClient.Update(ctx, existingDeployment); err != nil {
			return false, errors.Wrapf(err, "update deployment %s", deployment.Name)
		}
	}
	return existingDeployment.Status.ReadyReplicas > 0, nil
}

// deleteStaleGatewayProxies removes proxy resources whose Gateway is gone or
// no longer accepted.
func deleteStaleGatewayProxies(ctx context.Context, kubeClient client.Client, namespace string, keep map[string]bool) error {
	lists := []client.ObjectList{&appsv1.DeploymentList{}, &corev1.ServiceList{}, &corev1.ConfigMapList{}}
	for _, list := range lists {
		err := kubeClient.List(ctx, list, client.InNamespace(namespace), client.HasLabels{gatewayProxyLabelKey})
		if err != nil {
			return errors.Wrap(err, "list gateway proxy resources")
		}
		var items []client.Object
		switch typed := list.(type) {
		case *appsv1.DeploymentList:
			for i := range typed.Items {
				items = append(items, &typed.Items[i])
			}
		case *corev1.ServiceList:
			for i := range typed.Items {
				items = append(items, &typed.Items[i])
			}
		case *corev1.ConfigMapList:
			for i := range typed.Items {
				items = append(items, &typed.Items[i])
			}
		}
		for _, item := range items {
			if keep[item.GetLabels()[gatewayProxyLabelKey]] {
				continue
			}
			err := kubeClient.Delete(ctx, item)
			if err != nil && !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "delete gateway proxy resource %s", item.GetName())
			}
		}
	}
	return nil
}

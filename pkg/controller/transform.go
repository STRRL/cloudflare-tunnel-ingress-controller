package controller

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func FromIngressToExposure(ctx context.Context, logger logr.Logger, kubeClient client.Client, ingress networkingv1.Ingress) ([]exposure.Exposure, error) {
	isDeleted := false

	if ingress.DeletionTimestamp != nil {
		isDeleted = true
	}

	if len(ingress.Spec.TLS) > 0 {
		logger.Info("ingress has tls specified, SSL Passthrough is not supported, it will be ignored.")
	}

	var result []exposure.Exposure
	for _, rule := range ingress.Spec.Rules {
		if rule.Host == "" {
			return nil, errors.Errorf("host in ingress %s/%s is empty", ingress.GetNamespace(), ingress.GetName())
		}

		hostname := rule.Host
		scheme := "http"

		if backendProtocol, ok := getAnnotation(ingress.Annotations, AnnotationBackendProtocol); ok {
			scheme = backendProtocol
		}

		var httpHostHeader *string

		if header, ok := getAnnotation(ingress.Annotations, AnnotationHTTPHostHeader); ok {
			httpHostHeader = ptr.To(header)
		}

		var originServerName *string

		if name, ok := getAnnotation(ingress.Annotations, AnnotationOriginServerName); ok {
			originServerName = ptr.To(name)
		}

		var proxySSLVerifyEnabled *bool

		if proxySSLVerify, ok := getAnnotation(ingress.Annotations, AnnotationProxySSLVerify); ok {
			if proxySSLVerify == AnnotationProxySSLVerifyOn {
				proxySSLVerifyEnabled = boolPointer(true)
			} else if proxySSLVerify == AnnotationProxySSLVerifyOff {
				proxySSLVerifyEnabled = boolPointer(false)
			} else {
				return nil, errors.Errorf(
					"invalid value for annotation %s, available values: \"%s\" or \"%s\"",
					AnnotationProxySSLVerify,
					AnnotationProxySSLVerifyOn,
					AnnotationProxySSLVerifyOff,
				)
			}
		}

		for _, path := range rule.HTTP.Paths {
			namespacedName := types.NamespacedName{
				Namespace: ingress.GetNamespace(),
				Name:      path.Backend.Service.Name,
			}
			service := v1.Service{}
			err := kubeClient.Get(ctx, namespacedName, &service)
			if err != nil {
				return nil, errors.Wrapf(err, "fetch service %s", namespacedName)
			}

			host, err := getHostFromService(&service)
			if err != nil {
				return nil, err
			}

			var port int32
			if path.Backend.Service.Port.Name != "" {
				ok, extractedPort := getPortWithName(service.Spec.Ports, path.Backend.Service.Port.Name)
				if !ok {
					return nil, errors.Errorf("service %s has no port named %s", namespacedName, path.Backend.Service.Port.Name)
				}
				port = extractedPort
			} else {
				port = path.Backend.Service.Port.Number
			}

			var supportedPathTypes = map[networkingv1.PathType]struct{}{
				networkingv1.PathTypePrefix:                 {},
				networkingv1.PathTypeImplementationSpecific: {},
			}

			if path.PathType == nil {
				return nil, errors.Errorf("path type in ingress %s/%s is nil", ingress.GetNamespace(), ingress.GetName())
			}

			if _, ok := supportedPathTypes[*path.PathType]; !ok {
				return nil, errors.Errorf("path type in ingress %s/%s is %s, which is not supported", ingress.GetNamespace(), ingress.GetName(), *path.PathType)
			}

			result = append(result, exposure.Exposure{
				Hostname:              hostname,
				ServiceTarget:         fmt.Sprintf("%s://%s:%d", scheme, host, port),
				PathPrefix:            path.Path,
				IsDeleted:             isDeleted,
				ProxySSLVerifyEnabled: proxySSLVerifyEnabled,
				HTTPHostHeader:        httpHostHeader,
				OriginServerName:      originServerName,
			})
		}
	}

	return result, nil
}

func getHostFromService(service *v1.Service) (string, error) {
	if service.Spec.ClusterIP == "None" {
		return "", errors.Errorf("service %s has None for cluster ip, headless service is not supported", client.ObjectKeyFromObject(service))
	}

	if service.Spec.ClusterIP != "" {
		return service.Spec.ClusterIP, nil
	}

	if service.Spec.ExternalName != "" {
		return service.Spec.ExternalName, nil
	}

	return "", errors.Errorf("service %s has no cluster ip nor external name", client.ObjectKeyFromObject(service))
}

func getPortWithName(ports []v1.ServicePort, portName string) (bool, int32) {
	for _, port := range ports {
		if port.Name == portName {
			return true, port.Port
		}
	}
	return false, 0
}

func getAnnotation(annotations map[string]string, key string) (string, bool) {
	value, ok := annotations[key]
	return value, ok
}

func boolPointer(b bool) *bool {
	return &b
}

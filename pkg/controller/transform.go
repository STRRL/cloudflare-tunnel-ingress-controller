package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

		var noTlsVerify *bool
		if annotation, ok := getAnnotation(ingress.Annotations, AnnotationProxySSLVerify); ok {
			if annotation == AnnotationProxySSLVerifyOn {
				noTlsVerify = boolPointer(false)
			} else if annotation == AnnotationProxySSLVerifyOff {
				noTlsVerify = boolPointer(true)
			} else {
				return nil, errors.Errorf(
					"invalid value for annotation %s, available values: \"%s\" or \"%s\"",
					AnnotationProxySSLVerify,
					AnnotationProxySSLVerifyOn,
					AnnotationProxySSLVerifyOff,
				)
			}
		}

		var originServerName *string
		if annotation, ok := getAnnotation(ingress.Annotations, AnnotationOriginServerName); ok {
			originServerName = &annotation
		}

		var caPool *string
		if annotation, ok := getAnnotation(ingress.Annotations, AnnotationCAPool); ok {
			caPool = &annotation
		}

		var tlsTimeout *time.Duration
		if annotation, ok := getAnnotation(ingress.Annotations, AnnotationTLSTimeout); ok {
			duration, err := time.ParseDuration(annotation)
			if err != nil {
				return nil, errors.Errorf("invalid duration for tls timeout %s", annotation)
			}

			tlsTimeout = &duration
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

			if service.Spec.ClusterIP == "" {
				return nil, errors.Errorf("service %s has no cluster ip", namespacedName)
			}

			if service.Spec.ClusterIP == "None" {
				return nil, errors.Errorf("service %s has None for cluster ip, headless service is not supported", namespacedName)
			}

			host := service.Spec.ClusterIP

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

			// TODO: support other path types
			if path.PathType == nil {
				return nil, errors.Errorf("path type in ingress %s/%s is nil", ingress.GetNamespace(), ingress.GetName())
			}
			if *path.PathType != networkingv1.PathTypePrefix {
				return nil, errors.Errorf("path type in ingress %s/%s is %s, which is not supported", ingress.GetNamespace(), ingress.GetName(), *path.PathType)
			}

			result = append(result, exposure.Exposure{
				Hostname:         hostname,
				ServiceTarget:    fmt.Sprintf("%s://%s:%d", scheme, host, port),
				PathPrefix:            path.Path,
				IsDeleted:        isDeleted,
				OriginServerName: originServerName,
				CAPool:           caPool,
				NoTLSVerify:      noTlsVerify,
				TLSTimeout:       tlsTimeout,
			})
		}
	}

	return result, nil
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

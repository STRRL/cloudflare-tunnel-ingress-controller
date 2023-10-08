package controller

import (
	"strconv"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
)

func stringToCloudflareTime(s string) (*cloudflare.TunnelDuration, error) {
	intValue, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		// Handle the error (e.g., log it or return an error)
		return nil, errors.Wrapf(err, "Error converting to int64")
	} else {
		// Convert the int64 to time.Duration. Assuming tlsTimeout represents seconds.
		durationValue := time.Duration(intValue) * time.Second
		tunnelDurationValue := cloudflare.TunnelDuration{Duration: durationValue}
		return &tunnelDurationValue, nil
	}
}

func evalAnnotationBool(s string) (*bool, error) {
	if s == AnnotationValueBooleanTrue {
		return boolPointer(true), nil
	} else if s == AnnotationValueBooleanFalse {
		return boolPointer(false), nil
	} else {
		return nil, errors.Errorf(
			"available values: \"%s\" or \"%s\"",
			AnnotationValueBooleanTrue,
			AnnotationValueBooleanFalse,
		)
	}
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

package controller

import (
	"github.com/cloudflare/cloudflare-go"
	"github.com/pkg/errors"
)

func annotationProperties(annotations map[string]string) (*cloudflare.OriginRequestConfig, error) {
	config := &cloudflare.OriginRequestConfig{}

	for key, value := range annotations {
		switch key {
		case AnnotationProxySSLVerify:
			enabled, err := evalAnnotationBool(value)
			if err != nil {
				return nil, errors.Wrapf(err, "parsing annotation value (%s)", key)
			}
			if *enabled {
				config.NoTLSVerify = boolPointer(false)
			} else {
				config.NoTLSVerify = boolPointer(true)
			}

		case AnnotationTLSTimeout:
			tlsTimeout, err := stringToCloudflareTime(value)
			if err != nil {
				return nil, errors.Wrapf(err, "parsing annotation value (%s)", key)
			}
			config.TLSTimeout = tlsTimeout

		case AnnotationConnectionTimeount:
			connectTimeout, err := stringToCloudflareTime(value)
			if err != nil {
				return nil, errors.Wrapf(err, "parsing annotation value (%s)", key)
			}
			config.ConnectTimeout = connectTimeout

		case AnnotationChunkedEncoding:
			enabled, err := evalAnnotationBool(value)
			if err != nil {
				return nil, errors.Wrapf(err, "parsing annotation value (%s)", key)
			}
			if *enabled {
				config.DisableChunkedEncoding = boolPointer(false)
			} else {
				config.DisableChunkedEncoding = boolPointer(true)
			}

		case AnnotationHappyEyeballs:
			enabled, err := evalAnnotationBool(value)
			if err != nil {
				return nil, errors.Wrapf(err, "parsing annotation value (%s)", key)
			}
			if *enabled {
				config.NoHappyEyeballs = boolPointer(false)
			} else {
				config.NoHappyEyeballs = boolPointer(true)
			}

		case AnnotationHTTP20Origin:
			enabled, err := evalAnnotationBool(value)
			if err != nil {
				return nil, errors.Wrapf(err, "parsing annotation value (%s)", key)
			}
			config.Http2Origin = enabled

		case AnnotationTCPKeepAliveTimeout:
			tunnelDuration, err := stringToCloudflareTime(value)
			if err != nil {
				return nil, errors.Wrapf(err, "parsing annotation value (%s)", key)
			}
			config.KeepAliveTimeout = tunnelDuration

		}
	}

	return config, nil
}

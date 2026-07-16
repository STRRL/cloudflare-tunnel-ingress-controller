package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestManagerOptionsScopeSecretCacheToControllerNamespace(t *testing.T) {
	options := buildManagerOptions(rootCmdFlags{
		leaderElect: true,
		namespace:   "controller-system",
	})

	if options.Client.Cache != nil {
		t.Fatal("manager client cache bypass must not be configured")
	}

	if len(options.Cache.ByObject) != 1 {
		t.Fatalf("expected one object-specific cache, got %d", len(options.Cache.ByObject))
	}

	for object, objectCache := range options.Cache.ByObject {
		if _, ok := object.(*corev1.Secret); !ok {
			t.Fatalf("expected Secret cache configuration, got %T", object)
		}
		if len(objectCache.Namespaces) != 1 {
			t.Fatalf("expected one cached namespace, got %d", len(objectCache.Namespaces))
		}
		if _, ok := objectCache.Namespaces["controller-system"]; !ok {
			t.Fatal("controller namespace is not cached for Secrets")
		}
	}
}

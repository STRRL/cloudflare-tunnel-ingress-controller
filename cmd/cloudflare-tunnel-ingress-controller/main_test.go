package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestManagerOptionsDisableSecretCache(t *testing.T) {
	options := buildManagerOptions(rootCmdFlags{
		leaderElect: true,
		namespace:   "controller-system",
	})

	if options.Client.Cache == nil {
		t.Fatal("manager client cache options must be configured")
	}

	disabledObjects := options.Client.Cache.DisableFor
	if len(disabledObjects) != 1 {
		t.Fatalf("expected one cache-disabled object, got %d", len(disabledObjects))
	}

	if _, ok := disabledObjects[0].(*corev1.Secret); !ok {
		t.Fatalf("expected Secret cache to be disabled, got %T", disabledObjects[0])
	}
}

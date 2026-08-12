// Package v1alpha1 contains the CloudflareAccess API types.
//
// A CloudflareAccess object puts Cloudflare Access authentication in
// front of Ingresses that this controller exposes. See
// docs/design/cloudflare-access.md for the full design.
//
// +kubebuilder:object:generate=true
// +groupName=cloudflare-tunnel-ingress-controller.strrl.dev
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is the group and version of the types in this package.
	GroupVersion = schema.GroupVersion{Group: "cloudflare-tunnel-ingress-controller.strrl.dev", Version: "v1alpha1"}

	// SchemeBuilder collects the types to add to a scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this package to a scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

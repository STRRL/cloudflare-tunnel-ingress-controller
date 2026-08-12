package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TargetRef points at a resource to protect.
type TargetRef struct {
	// Kind of the target resource.
	// +kubebuilder:validation:Enum=Ingress
	Kind string `json:"kind"`

	// Name of the target resource.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// PolicyRef references one reusable Cloudflare Access Policy.
// +kubebuilder:validation:XValidation:rule="has(self.name) != has(self.id)",message="set exactly one of name or id"
type PolicyRef struct {
	// Name of a reusable Access Policy. Resolved to an ID at reconcile
	// time. Resolution fails when no policy or more than one policy has
	// this name.
	// +optional
	Name string `json:"name,omitempty"`

	// ID of a reusable Access Policy. Stable when the policy is renamed.
	// +optional
	ID string `json:"id,omitempty"`
}

// CloudflareAccessSpec is the desired state of a CloudflareAccess.
// +kubebuilder:validation:XValidation:rule="!(has(self.autoRedirectToIdentity) && self.autoRedirectToIdentity) || (has(self.allowedIdentityProviders) && size(self.allowedIdentityProviders) == 1)",message="autoRedirectToIdentity requires exactly one allowedIdentityProviders entry"
type CloudflareAccessSpec struct {
	// TargetRefs selects the Ingresses to protect. Only Ingresses in
	// the same namespace as this object can be referenced. All
	// hostnames of all targets become destinations of one shared
	// Access Application, so one login covers all of them.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +listType=map
	// +listMapKey=kind
	// +listMapKey=name
	TargetRefs []TargetRef `json:"targetRefs"`

	// Policies references reusable Access Policies in ascending order
	// of precedence. The first entry is evaluated first.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=32
	Policies []PolicyRef `json:"policies"`

	// SessionDuration is how long a session lasts before the user has
	// to authenticate again.
	// +optional
	// +kubebuilder:validation:Enum="30m";"6h";"12h";"24h";"168h";"730h"
	SessionDuration string `json:"sessionDuration,omitempty"`

	// AllowedIdentityProviders limits which identity providers users
	// can pick for this application, by identity provider ID. Empty
	// means every provider configured in the account.
	// +optional
	// +kubebuilder:validation:MaxItems=16
	AllowedIdentityProviders []string `json:"allowedIdentityProviders,omitempty"`

	// AutoRedirectToIdentity skips the identity provider selection
	// page. The Cloudflare API requires exactly one entry in
	// allowedIdentityProviders when this is true.
	// +optional
	AutoRedirectToIdentity *bool `json:"autoRedirectToIdentity,omitempty"`
}

// Condition types and reasons reported on CloudflareAccess objects.
const (
	// ConditionAccepted reports whether the object is active and its
	// Access Application is being managed.
	ConditionAccepted = "Accepted"
	// ConditionResolvedRefs reports whether every reference in the spec
	// resolved.
	ConditionResolvedRefs = "ResolvedRefs"

	ReasonAccepted        = "Accepted"
	ReasonConflicted      = "Conflicted"
	ReasonCloudflareError = "CloudflareError"
	ReasonResolved        = "Resolved"
	ReasonTargetNotFound  = "TargetNotFound"
	ReasonNoHostnames     = "NoHostnames"
	ReasonPolicyNotFound  = "PolicyNotFound"
	ReasonAmbiguousPolicy = "AmbiguousPolicy"
)

// CloudflareAccessStatus is the observed state of a CloudflareAccess.
type CloudflareAccessStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ApplicationID of the managed Access Application.
	// +optional
	ApplicationID string `json:"applicationID,omitempty"`

	// AUD is the application audience tag. Origins that validate the
	// Cf-Access-Jwt-Assertion header need this value.
	// +optional
	AUD string `json:"aud,omitempty"`

	// Hostnames currently covered by the Access Application.
	// +optional
	Hostnames []string `json:"hostnames,omitempty"`
}

// CloudflareAccess puts Cloudflare Access authentication in front of
// the referenced Ingresses.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Accepted",type=string,JSONPath=`.status.conditions[?(@.type=="Accepted")].status`
// +kubebuilder:printcolumn:name="Application",type=string,JSONPath=`.status.applicationID`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type CloudflareAccess struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CloudflareAccessSpec `json:"spec"`
	// +optional
	Status CloudflareAccessStatus `json:"status,omitempty"`
}

// CloudflareAccessList is a list of CloudflareAccess objects.
// +kubebuilder:object:root=true
type CloudflareAccessList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CloudflareAccess `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CloudflareAccess{}, &CloudflareAccessList{})
}

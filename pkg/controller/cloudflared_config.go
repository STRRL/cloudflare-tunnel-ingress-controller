package controller

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"

	v1 "k8s.io/api/core/v1"
)

// CloudflaredDeploymentConfig holds customizable fields for the cloudflared Deployment pod spec.
// It is loaded from a JSON config file mounted via ConfigMap.
type CloudflaredDeploymentConfig struct {
	Resources                 *v1.ResourceRequirements      `json:"resources,omitempty"`
	SecurityContext           *v1.SecurityContext           `json:"securityContext,omitempty"`
	PodSecurityContext        *v1.PodSecurityContext        `json:"podSecurityContext,omitempty"`
	PodLabels                 map[string]string             `json:"podLabels,omitempty"`
	PodAnnotations            map[string]string             `json:"podAnnotations,omitempty"`
	NodeSelector              map[string]string             `json:"nodeSelector,omitempty"`
	Tolerations               []v1.Toleration               `json:"tolerations,omitempty"`
	Affinity                  *v1.Affinity                  `json:"affinity,omitempty"`
	TopologySpreadConstraints []v1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
	PriorityClassName         string                        `json:"priorityClassName,omitempty"`
	Probes                    *CloudflaredProbes            `json:"probes,omitempty"`
	Volumes                   []v1.Volume                   `json:"volumes,omitempty"`
	VolumeMounts              []v1.VolumeMount              `json:"volumeMounts,omitempty"`
}

// CloudflaredProbes holds probe configuration for the cloudflared container.
type CloudflaredProbes struct {
	Liveness  *v1.Probe `json:"liveness,omitempty"`
	Readiness *v1.Probe `json:"readiness,omitempty"`
	Startup   *v1.Probe `json:"startup,omitempty"`
}

// LoadCloudflaredDeploymentConfig loads a CloudflaredDeploymentConfig from a JSON file.
// If path is empty, returns an empty config and empty hash.
func LoadCloudflaredDeploymentConfig(path string) (*CloudflaredDeploymentConfig, string, error) {
	if path == "" {
		return &CloudflaredDeploymentConfig{}, "", nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read cloudflared deployment config: %w", err)
	}

	var config CloudflaredDeploymentConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, "", fmt.Errorf("parse cloudflared deployment config: %w", err)
	}

	hash := sha256.Sum256(data)
	return &config, fmt.Sprintf("%x", hash), nil
}

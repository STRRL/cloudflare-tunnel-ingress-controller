package controller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCloudflaredDeploymentConfig_EmptyPath(t *testing.T) {
	config, hash, err := LoadCloudflaredDeploymentConfig("")
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "", hash)
	assert.Nil(t, config.Resources)
	assert.Nil(t, config.SecurityContext)
}

func TestLoadCloudflaredDeploymentConfig_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	configJSON := `{
		"resources": {
			"requests": {"cpu": "100m", "memory": "128Mi"},
			"limits": {"cpu": "200m", "memory": "256Mi"}
		},
		"securityContext": {
			"readOnlyRootFilesystem": true,
			"runAsNonRoot": true
		},
		"podSecurityContext": {
			"runAsNonRoot": true
		},
		"podLabels": {"team": "platform"},
		"podAnnotations": {"prometheus.io/scrape": "true"},
		"nodeSelector": {"kubernetes.io/os": "linux"},
		"priorityClassName": "high-priority",
		"probes": {
			"liveness": {
				"httpGet": {"path": "/ready", "port": 44483},
				"initialDelaySeconds": 10
			},
			"readiness": {
				"httpGet": {"path": "/ready", "port": 44483}
			}
		}
	}`

	err := os.WriteFile(configPath, []byte(configJSON), 0644)
	require.NoError(t, err)

	config, hash, err := LoadCloudflaredDeploymentConfig(configPath)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotNil(t, config.Resources)
	assert.NotNil(t, config.Resources.Requests)
	assert.NotNil(t, config.Resources.Limits)
	assert.NotNil(t, config.SecurityContext)
	assert.True(t, *config.SecurityContext.ReadOnlyRootFilesystem)
	assert.True(t, *config.SecurityContext.RunAsNonRoot)
	assert.NotNil(t, config.PodSecurityContext)
	assert.True(t, *config.PodSecurityContext.RunAsNonRoot)
	assert.Equal(t, map[string]string{"team": "platform"}, config.PodLabels)
	assert.Equal(t, map[string]string{"prometheus.io/scrape": "true"}, config.PodAnnotations)
	assert.Equal(t, map[string]string{"kubernetes.io/os": "linux"}, config.NodeSelector)
	assert.Equal(t, "high-priority", config.PriorityClassName)
	assert.NotNil(t, config.Probes)
	assert.NotNil(t, config.Probes.Liveness)
	assert.NotNil(t, config.Probes.Readiness)
	assert.Nil(t, config.Probes.Startup)
}

func TestLoadCloudflaredDeploymentConfig_EmptyJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	err := os.WriteFile(configPath, []byte("{}"), 0644)
	require.NoError(t, err)

	config, hash, err := LoadCloudflaredDeploymentConfig(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.NotEmpty(t, hash)
	assert.Nil(t, config.Resources)
}

func TestLoadCloudflaredDeploymentConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	err := os.WriteFile(configPath, []byte("not json"), 0644)
	require.NoError(t, err)

	config, hash, err := LoadCloudflaredDeploymentConfig(configPath)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Empty(t, hash)
}

func TestLoadCloudflaredDeploymentConfig_MissingFile(t *testing.T) {
	config, hash, err := LoadCloudflaredDeploymentConfig("/nonexistent/path/config.json")
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Empty(t, hash)
}

func TestLoadCloudflaredDeploymentConfig_HashConsistency(t *testing.T) {
	dir := t.TempDir()

	path1 := filepath.Join(dir, "config1.json")
	err := os.WriteFile(path1, []byte(`{"resources": {}}`), 0644)
	require.NoError(t, err)

	path2 := filepath.Join(dir, "config2.json")
	err = os.WriteFile(path2, []byte(`{"resources": {"requests": {"cpu": "100m"}}}`), 0644)
	require.NoError(t, err)

	_, hash1, err := LoadCloudflaredDeploymentConfig(path1)
	require.NoError(t, err)
	_, hash2, err := LoadCloudflaredDeploymentConfig(path2)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2)
	assert.Len(t, hash1, 64)
	assert.Len(t, hash2, 64)
}

func TestLoadCloudflaredDeploymentConfig_SameContentSameHash(t *testing.T) {
	dir := t.TempDir()

	path1 := filepath.Join(dir, "config1.json")
	err := os.WriteFile(path1, []byte(`{"resources": {}}`), 0644)
	require.NoError(t, err)

	path2 := filepath.Join(dir, "config2.json")
	err = os.WriteFile(path2, []byte(`{"resources": {}}`), 0644)
	require.NoError(t, err)

	_, hash1, err := LoadCloudflaredDeploymentConfig(path1)
	require.NoError(t, err)
	_, hash2, err := LoadCloudflaredDeploymentConfig(path2)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2)
}

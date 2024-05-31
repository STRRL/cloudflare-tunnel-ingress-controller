package conformance

import (
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"
	"os"
	"sigs.k8s.io/gateway-api/conformance"
	conformancev1 "sigs.k8s.io/gateway-api/conformance/apis/v1"
	"sigs.k8s.io/gateway-api/conformance/tests"
	"sigs.k8s.io/gateway-api/conformance/utils/suite"
	"sigs.k8s.io/gateway-api/pkg/features"
	"sigs.k8s.io/yaml"
	"testing"
)

func TestGatewayConformance(t *testing.T) {
	var features []features.SupportedFeature

	opts := conformance.DefaultOptions(t)
	opts.GatewayClassName = "cloudflare-tunnel"
	opts.Debug = true
	opts.SupportedFeatures = sets.New(features...)
	opts.ConformanceProfiles = sets.New(
		suite.GatewayHTTPConformanceProfileName,
	)
	opts.Implementation = conformancev1.Implementation{
		Organization: "strrl",
		Project:      "cloudflare-tunnel-ingress-controller",
		URL:          "https://github.com/STRRL/cloudflare-tunnel-ingress-controller",
		Version:      "latest",
	}

	testSuite, err := suite.NewConformanceTestSuite(opts)
	require.NoError(t, err)

	testSuite.Setup(t, tests.ConformanceTests)
	err = testSuite.Run(t, tests.ConformanceTests)
	require.NoError(t, err)

	report, err := testSuite.Report()
	require.NoError(t, err)

	bytes, err := yaml.Marshal(report)
	require.NoError(t, err)

	err = os.WriteFile("gateway-api-conformance-test-report.yaml", bytes, 0o600)
	require.NoError(t, err)
}

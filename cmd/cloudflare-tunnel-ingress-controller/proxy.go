package main

import (
	"os/signal"
	"syscall"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/gatewayproxy"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// newProxyCommand runs the per Gateway data plane proxy. It ships in the
// controller image, so a Gateway needs no extra image.
func newProxyCommand(rootLogger logr.Logger) *cobra.Command {
	options := gatewayproxy.Options{
		ListenAddress: ":8080",
		HealthAddress: ":8081",
	}
	command := &cobra.Command{
		Use:   "proxy",
		Short: "serve the routing table of one Gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.GetConfig()
			if err != nil {
				return errors.Wrap(err, "load kubeconfig")
			}
			clientset, err := kubernetes.NewForConfig(cfg)
			if err != nil {
				return errors.Wrap(err, "create kubernetes client")
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			return gatewayproxy.Run(ctx, rootLogger.WithName("gateway-proxy"), clientset, options)
		},
	}
	command.Flags().StringVar(&options.Namespace, "configmap-namespace", options.Namespace, "namespace of the routing table ConfigMap")
	command.Flags().StringVar(&options.ConfigMapName, "configmap-name", options.ConfigMapName, "name of the routing table ConfigMap")
	command.Flags().StringVar(&options.ListenAddress, "listen-address", options.ListenAddress, "address serving proxied traffic")
	command.Flags().StringVar(&options.HealthAddress, "health-address", options.HealthAddress, "address serving /healthz and /readyz")
	return command
}

package main

import (
	"context"
	"log"
	"os"
	"time"

	cloudflarecontroller "github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/cloudflare-controller"
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/controller"
	"github.com/cloudflare/cloudflare-go"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type rootCmdFlags struct {
	logger logr.Logger
	// for annotation on Ingress
	ingressClass string
	// for IngressClass.spec.controller
	controllerClass       string
	logLevel              int
	cloudflareAPIToken    string
	cloudflareAccountId   string
	cloudflareTunnelName  string
	namespace             string
	cloudflaredProtocol   string
	cloudflaredExtraArgs  []string
}

func main() {
	var rootLogger = stdr.NewWithOptions(log.New(os.Stderr, "", log.LstdFlags), stdr.Options{LogCaller: stdr.All})

	options := rootCmdFlags{
		logger:              rootLogger.WithName("main"),
		ingressClass:        "cloudflare-tunnel",
		controllerClass:     "strrl.dev/cloudflare-tunnel-ingress-controller",
		logLevel:            0,
		namespace:           "default",
		cloudflaredProtocol: "auto",
	}

	crlog.SetLogger(rootLogger.WithName("controller-runtime"))

	rootCommand := cobra.Command{
		Use: "tunnel-controller",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			stdr.SetVerbosity(options.logLevel)
			logger := options.logger
			logger.Info("logging verbosity", "level", options.logLevel)

			logger.V(3).Info("build cloudflare client with API Token", "api-token", options.cloudflareAPIToken)
			cloudflareClient, err := cloudflare.NewWithAPIToken(options.cloudflareAPIToken)
			if err != nil {
				logger.Error(err, "create cloudflare client")
				os.Exit(1)
			}

			var tunnelClient *cloudflarecontroller.TunnelClient

			logger.V(3).Info("bootstrap tunnel client with tunnel name", "account-id", options.cloudflareAccountId, "tunnel-name", options.cloudflareTunnelName)
			tunnelClient, err = cloudflarecontroller.BootstrapTunnelClientWithTunnelName(ctx, logger.WithName("tunnel-client"), cloudflareClient, options.cloudflareAccountId, options.cloudflareTunnelName)
			if err != nil {
				logger.Error(err, "bootstrap tunnel client with tunnel name")
				os.Exit(1)
			}

			cfg, err := config.GetConfig()
			if err != nil {
				logger.Error(err, "unable to get kubeconfig")
				os.Exit(1)
			}

			mgr, err := manager.New(cfg, manager.Options{})
			if err != nil {
				logger.Error(err, "unable to set up manager")
				os.Exit(1)
			}

			logger.Info("cloudflare-tunnel-ingress-controller start serving")
			err = controller.RegisterIngressController(logger, mgr,
				controller.IngressControllerOptions{
					IngressClassName:    options.ingressClass,
					ControllerClassName: options.controllerClass,
					CFTunnelClient:      tunnelClient,
				})
			if err != nil {
				return err
			}

			ticker := time.NewTicker(10 * time.Second)
			done := make(chan struct{})
			defer close(done)

			go func() {
				for {
					select {
					case <-done:
						return
					case _ = <-ticker.C:
						err := controller.CreateOrUpdateControlledCloudflared(ctx, mgr.GetClient(), tunnelClient, options.namespace, options.cloudflaredProtocol, options.cloudflaredExtraArgs)
						if err != nil {
							logger.WithName("controlled-cloudflared").Error(err, "create controlled cloudflared")
						}
					}
				}
			}()

			// controller-runtime manager would graceful shutdown with signal by itself, no need to provide context
			return mgr.Start(context.Background())
		},
	}

	rootCommand.PersistentFlags().StringVar(&options.ingressClass, "ingress-class", options.ingressClass, "ingress class name")
	rootCommand.PersistentFlags().StringVar(&options.controllerClass, "controller-class", options.controllerClass, "controller class name")
	rootCommand.PersistentFlags().IntVarP(&options.logLevel, "log-level", "v", options.logLevel, "numeric log level")
	rootCommand.PersistentFlags().StringVar(&options.cloudflareAPIToken, "cloudflare-api-token", options.cloudflareAPIToken, "cloudflare api token")
	rootCommand.PersistentFlags().StringVar(&options.cloudflareAccountId, "cloudflare-account-id", options.cloudflareAccountId, "cloudflare account id")
	rootCommand.PersistentFlags().StringVar(&options.cloudflareTunnelName, "cloudflare-tunnel-name", options.cloudflareTunnelName, "cloudflare tunnel name")
	rootCommand.PersistentFlags().StringVar(&options.namespace, "namespace", options.namespace, "namespace to execute cloudflared connector")
	rootCommand.PersistentFlags().StringVar(&options.cloudflaredProtocol, "cloudflared-protocol", options.cloudflaredProtocol, "cloudflared protocol")
	rootCommand.PersistentFlags().StringSliceVar(&options.cloudflaredExtraArgs, "cloudflared-extra-args", options.cloudflaredExtraArgs, "extra arguments to pass to cloudflared")

	err := rootCommand.Execute()
	if err != nil {
		panic(err)
	}
}

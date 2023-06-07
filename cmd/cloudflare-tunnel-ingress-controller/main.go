package main

import (
	"context"
	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/controller"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/spf13/cobra"
	"log"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type rootOptions struct {
	logger logr.Logger
	// for annotation on Ingress
	ingressClass string
	// for IngressClass.spec.controller
	controllerClass string
}

func main() {
	var rootLogger = stdr.NewWithOptions(log.New(os.Stderr, "", log.LstdFlags), stdr.Options{LogCaller: stdr.All})

	options := rootOptions{
		logger:          rootLogger.WithName("main"),
		ingressClass:    "strrl.dev/cloudflare-tunnel",
		controllerClass: "strrl.dev/cloudflare-tunnel-ingress-controller",
	}

	crlog.SetLogger(rootLogger.WithName("controller-runtime"))

	rootCommand := cobra.Command{
		Use: "tunnel-controller",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := options.logger

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
			err = controller.RegisterIngressController(logger, mgr)
			if err != nil {
				return err
			}
			// controller-runtime manager would graceful shutdown with signal by itself, no need to provide context
			return mgr.Start(context.Background())
		},
	}
	err := rootCommand.Execute()
	if err != nil {
		panic(err)
	}
}

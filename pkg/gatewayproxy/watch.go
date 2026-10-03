package gatewayproxy

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Options configures a proxy process.
type Options struct {
	Namespace     string
	ConfigMapName string
	ListenAddress string
	HealthAddress string
}

// Run serves the routing table from the ConfigMap until the context ends.
// The ConfigMap is watched through the API, so updates apply within about a
// second, much faster than a mounted volume.
func Run(ctx context.Context, logger logr.Logger, clientset kubernetes.Interface, options Options) error {
	server := NewServer(logger.WithName("server"))

	watchList := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		"configmaps",
		options.Namespace,
		fields.OneTermEqualSelector("metadata.name", options.ConfigMapName),
	)
	onChange := func(obj any) {
		configMap, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return
		}
		table, err := DecodeTable(configMap.Data[TableConfigMapKey])
		if err != nil {
			logger.Error(err, "ignore invalid route table", "configmap", configMap.Name)
			return
		}
		if err := server.SetTable(table); err != nil {
			logger.Error(err, "ignore invalid route table", "configmap", configMap.Name)
			return
		}
		logger.Info("route table loaded", "routes", len(table.Routes), "resource-version", configMap.ResourceVersion)
	}
	_, informer := cache.NewInformerWithOptions(cache.InformerOptions{
		ListerWatcher: watchList,
		ObjectType:    &corev1.ConfigMap{},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    onChange,
			UpdateFunc: func(_, newObj any) { onChange(newObj) },
		},
	})
	go informer.Run(ctx.Done())

	health := http.NewServeMux()
	health.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !server.Ready() {
			http.Error(w, "route table not loaded", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	health.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	servers := []*http.Server{
		{Addr: options.ListenAddress, Handler: server, ReadHeaderTimeout: 30 * time.Second},
		{Addr: options.HealthAddress, Handler: health, ReadHeaderTimeout: 30 * time.Second},
	}
	failed := make(chan error, len(servers))
	for _, item := range servers {
		go func() {
			failed <- item.ListenAndServe()
		}()
	}
	logger.Info("proxy serving", "listen", options.ListenAddress, "health", options.HealthAddress, "configmap", options.ConfigMapName)

	select {
	case err := <-failed:
		return errors.Wrap(err, "serve")
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, item := range servers {
		_ = item.Shutdown(shutdownCtx)
	}
	return nil
}

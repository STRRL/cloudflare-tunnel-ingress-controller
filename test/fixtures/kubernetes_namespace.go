package fixtures

import (
	"context"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

type KubernetesNamespaceFixtures struct {
	namespacePrefix string
	kubeClient      client.Client
	namespace       string
	lock            sync.Mutex
}

func NewKubernetesNamespaceFixtures(namespacePrefix string, kubeClient client.Client) *KubernetesNamespaceFixtures {
	return &KubernetesNamespaceFixtures{namespacePrefix: namespacePrefix, kubeClient: kubeClient, namespace: "", lock: sync.Mutex{}}
}

func (f *KubernetesNamespaceFixtures) Start(ctx context.Context) (string, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.namespace != "" {
		return f.namespace, nil
	}

	ns := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: f.namespacePrefix + "-",
		},
	}
	err := f.kubeClient.Create(ctx, &ns)
	if err != nil {
		return "", errors.Wrapf(err, "create namespace with generated name %s", f.namespacePrefix)
	}
	f.namespace = ns.Name
	return f.namespace, nil
}

func (f *KubernetesNamespaceFixtures) Stop(ctx context.Context) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	err := f.kubeClient.Delete(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: f.namespace,
		},
	})
	if err != nil {
		return errors.Wrapf(err, "delete namespace %s", f.namespace)
	}
	return nil
}

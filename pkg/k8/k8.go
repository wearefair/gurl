package k8

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/wearefair/gurl/pkg/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

const defaultNamespace = "default"

// Interface for handling K8 operations
type k8Client interface {
	Config() rest.Config
	Service(ctx context.Context, namespace, name string) (*v1.Service, error)
	Endpoints(ctx context.Context, namespace, name string) (*v1.Endpoints, error)
	PortForwarder(url *url.URL, localPort, remotePort string, ready, stop chan struct{}) (k8PortForwarder, error)
}

// K8 portforward interface so we can mock out the portforward's behavior in tests
type k8PortForwarder interface {
	ForwardPorts() error
}

func newK8Client(config *rest.Config) (*k8ClientImpl, error) {
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Errorf("port-forward - failed to create k8 client: %s", err)
		return nil, err
	}

	return &k8ClientImpl{
		config: config,
		client: clientSet,
	}, nil
}

type k8ClientImpl struct {
	config *rest.Config
	client kubernetes.Interface
}

func (k *k8ClientImpl) Config() rest.Config {
	return *k.config
}

func (k *k8ClientImpl) Service(ctx context.Context, namespace, name string) (*v1.Service, error) {
	return k.client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (k *k8ClientImpl) Endpoints(ctx context.Context, namespace, name string) (*v1.Endpoints, error) {
	return k.client.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (k *k8ClientImpl) PortForwarder(url *url.URL, localPort, remotePort string, ready, stop chan struct{}) (k8PortForwarder, error) {
	transport, upgrader, err := spdy.RoundTripperFor(k.config)
	if err != nil {
		log.Errorf("port-forward - failed to create roundtripper: %s", err)
		return nil, err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", url)
	return portforward.New(dialer,
		[]string{fmt.Sprintf("%s:%s", localPort, remotePort)},
		stop,
		ready,
		// TODO: We can make this configurable to maybe wrap logs.
		os.Stdout,
		os.Stderr,
	)
}

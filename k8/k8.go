package main

import (
	"net/http"
	"os"

	"github.com/wearefair/gurl/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

var (
	logger = log.Logger()
)

type K8 struct {
	Client       kubernetes.Interface
	Config       *rest.Config
	RESTClient   *rest.RESTClient
	StopChannel  chan struct{}
	ReadyChannel chan struct{}
}

func New(kubeconfig string) (*K8, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	restcli, err := rest.RESTClientFor(cfg)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return &K8{Client: clientset, Config: cfg, RESTClient: restcli}, nil
}

func (k *K8) Forward(podName string, localPort, remotePort string) error {
	req := k.RESTClient.Post().
		Resource("pods").
		Namespace("default").
		Name(podName).
		SubResource("portforward")
	transport, upgrader, err := spdy.RoundTripperFor(k.Config)
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	dialer := spdy.NewDialer(upgrader,
		&http.Client{Transport: transport},
		"POST",
		req.URL(),
	)

	fw, err := portforward.New(dialer,
		[]string{localPort, remotePort},
		k.StopChannel,
		k.ReadyChannel,
		os.Stdout,
		os.Stderr,
	)
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	return fw.ForwardPorts()
}

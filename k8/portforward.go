package k8

import (
	"fmt"
	"sync"

	"net/http"
	"os"

	"github.com/wearefair/gurl/log"
	"k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type PortForwardRequest struct {
	Namespace  string
	Pod        string
	LocalPort  string
	RemotePort string
}

type PortForward struct {
	StopChannel  chan struct{}
	ReadyChannel chan struct{}
	ErrorChannel chan error
	Once         sync.Once
}

func NewPortForward() *PortForward {
	return &PortForward{
		StopChannel:  make(chan struct{}),
		ReadyChannel: make(chan struct{}),
		ErrorChannel: make(chan error),
		Once:         sync.Once{},
	}
}

func (p *PortForward) Forward(conf clientcmd.ClientConfig, req *PortForwardRequest) error {
	originalConf, err := conf.ClientConfig()
	if err != nil {
		return log.WrapError(err)
	}
	clientConf := restConfigWithDefaults(originalConf)
	restClient, err := rest.RESTClientFor(clientConf)
	if err != nil {
		return log.WrapError(err)
	}
	podReq := restClient.Post().Resource("pods").Namespace(defaultNamespace).Name(req.Pod).SubResource("portforward")
	transport, upgrader, err := spdy.RoundTripperFor(clientConf)
	if err != nil {
		return log.WrapError(err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", podReq.URL())
	fw, err := portforward.New(dialer,
		[]string{fmt.Sprintf("%s:%s", req.LocalPort, req.RemotePort)},
		p.StopChannel,
		p.ReadyChannel,
		os.Stdout,
		os.Stderr,
	)
	if err != nil {
		return log.WrapError(err)
	}
	go func() {
		p.ErrorChannel <- fw.ForwardPorts()
	}()
	return nil
}

func (p *PortForward) Stop() {
	p.Once.Do(func() {
		p.StopChannel <- struct{}{}
	})
}

func restConfigWithDefaults(config *rest.Config) *rest.Config {
	gv := v1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/api"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	return config
}

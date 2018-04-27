package k8

import (
	"sync"

	"net/http"
	"os"

	"github.com/wearefair/gurl/log"
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
	clientConf, err := conf.ClientConfig()
	if err != nil {
		return log.WrapError(err)
	}
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
		[]string{req.LocalPort, req.RemotePort},
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

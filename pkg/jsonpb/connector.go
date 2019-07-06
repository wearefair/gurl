package jsonpb

import (
	"fmt"

	"github.com/wearefair/gurl/pkg/k8"
	"k8s.io/client-go/tools/clientcmd"
)

// Connector wraps logic for connecting to a service, and then closing
// that connection.
type Connector interface {
	// Connect connects sand then returns the address that the connector is expected
	// to connect at, if any
	Connect() (address string, err error)
	Close() error
}

// K8Connector implements the
type K8Connector struct {
	k8Cfg clientcmd.ClientConfig
	req   k8.PortForwardRequest
	pf    *k8.PortForward
}

// NewK8Connector opens up a new portforward request to K8
func NewK8Connector(cfg clientcmd.ClientConfig, req k8.PortForwardRequest) *K8Connector {
	return &K8Connector{
		k8Cfg: cfg,
		req:   req,
	}
}

// Connect on the K8Connector opens up a portforward, and then returns the portforward
// address to dial and connect to
func (c *K8Connector) Connect() (string, error) {
	var address string
	pf, err := k8.StartPortForward(c.k8Cfg, c.req)
	if err != nil {
		return address, err
	}
	c.pf = pf

	address = fmt.Sprintf("localhost:%s", pf.LocalPort())
	return address, nil
}

// Close closest the portforward connection
func (c *K8Connector) Close() error {
	c.pf.Close()

	return nil
}

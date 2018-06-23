package k8

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/golang/glog"
	"github.com/wearefair/gurl/pkg/log"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// PortForwardRequest wraps the information required to portforward to a pod.
// Implements fmt.Stringer so the structure can easily be printed for logging.
type PortForwardRequest struct {
	Context string
	// Namespace the service runs in. Will be "default" if left blank.
	Namespace string
	// Service name.
	Service string
	// Service port.
	Port string
}

// String func to print the request.
func (p PortForwardRequest) String() string {
	return fmt.Sprintf("context: %s, namespace: %s, service: %s, port: %s", p.Context, p.Namespace, p.Service, p.Port)
}

// PortForward encapsulates all K8 portforwarding coordination.
type PortForward struct {
	// The local port we want to hold open to fire off the request from
	localPort       string
	stopChannel     chan struct{}
	stopCoordinator *sync.Once
}

// StartPortForward starts a portforward connection to a pod that is backing the requested service.
// If the connection was successfully established a PortForward will be returned, which exposes
// as methods:
// - The local port it is listening on
// - A channel which will be closed when the backing connection is closed (ex: terminated due to inactivity).
// - A Close method explictely close the connection.
//
// Returns an error if the connection could not be established.
func StartPortForward(config clientcmd.ClientConfig, req PortForwardRequest) (*PortForward, error) {
	rawConfig, err := config.RawConfig()
	if err != nil {
		log.Errorf("port-forward - error getting raw config: %s", err)
		return nil, err
	}

	newConfig := clientcmd.NewDefaultClientConfig(rawConfig, &clientcmd.ConfigOverrides{
		CurrentContext: req.Context,
	})

	clientConfig, err := newConfig.ClientConfig()
	if err != nil {
		log.Errorf("port-forward - failed to get client config: %s", err)
		return nil, err
	}

	client, err := newK8Client(clientConfig)
	if err != nil {
		return nil, err
	}

	localPort, err := getAvailablePort()
	if err != nil {
		log.Errorf("port-forward - failed to get a port: %s", err)
		return nil, err
	}

	return startPortForward(newConfig, req, client, localPort)
}

// Helper for StartPortForward that only takes interfaces so it can be mocked.
func startPortForward(config clientcmd.ClientConfig, req PortForwardRequest, client k8Client, localPort string) (*PortForward, error) {
	// If the caller didn't specify a namespace:
	// - Look in the context for a namespace
	// - Fall back to the default namespace
	if req.Namespace == "" {
		ns, _, err := config.Namespace()
		if err != nil {
			log.Errorf("port-forward - error getting namespace from config: %s", err)
			return nil, err
		}
		if ns == "" {
			if glog.V(2) {
				glog.Infof("port-forward - namespace was empty, now setting to default: %s", defaultNamespace)
			}
			ns = defaultNamespace
		}
		req.Namespace = ns
	}

	pod, remotePort, err := getPodNameAndRemotePort(client, req)
	if err != nil {
		glog.Errorf("port-forward - failed to get pod and remote port: %s", err)
		return nil, err
	}

	activePortForward := &PortForward{
		localPort:       localPort,
		stopChannel:     make(chan struct{}),
		stopCoordinator: &sync.Once{},
	}

	if glog.V(2) {
		glog.Infof("port-forward - setting up connection: namespace=%s, pod=%s, remote-port=%s",
			req.Namespace, pod, remotePort)
	}

	errChan, err := activePortForward.connect(client, req.Namespace, pod, remotePort)
	if err != nil {
		log.Errorf("port-forward - failed to start port forward: %s", err)
		return nil, err
	}

	// Read any errors off the error channel and close the connection.
	// This will result in the stopChannel being closed, which callers can
	// block on to know when the underlying connection has been closed.
	go func() {
		err := <-errChan
		if err != nil {
			log.Errorf("port-forward - error from active connection: %s", err)
		}
		activePortForward.Close()
	}()

	return activePortForward, nil
}

// LocalPort returns the local port used by the port forward struct.
func (p *PortForward) LocalPort() string {
	return p.localPort
}

// StoppedChannel returns a channel that will be closed when the underlying connection
// has been closed.
func (p *PortForward) StoppedChannel() <-chan struct{} {
	return p.stopChannel
}

// Close closes the port forward connection.
func (p *PortForward) Close() {
	p.stopCoordinator.Do(func() {
		close(p.stopChannel)
	})
}

// Starts a portforward connection to the given pod.
// If the connection is successfully established it returns an error channel.
// If the connection fails for any reason, returns an error.
//
// The returned channel will receive an error if the connection is broken for any reason.
func (p *PortForward) connect(client k8Client, namespace, podName, remotePort string) (chan error, error) {
	config := restConfigWithDefaults(client.Config())

	restClient, err := rest.RESTClientFor(&config)
	if err != nil {
		log.Errorf("port-forward - failed to create restclient: %s", err)
		return nil, err
	}

	url := restClient.Post().Resource("pods").Namespace(namespace).Name(podName).SubResource("portforward").URL()
	if glog.V(2) {
		glog.Infof("port-forward - constructed url for pod: %#v", url)
	}

	// Use this channel to block until the connection is ready.
	readyChannel := make(chan struct{}, 1)
	forwarder, err := client.PortForwarder(url, p.localPort, remotePort, readyChannel, p.stopChannel)
	if err != nil {
		log.Errorf("port-forward - failed to create portforward: %s", err)
		return nil, err
	}

	// Create the error channel, and start the port forward in a goroutine.
	errChan := make(chan error, 1)
	go func() {
		errChan <- forwarder.ForwardPorts()
	}()

	// At this point there are 2 possible options:
	// 1. The ForwardPorts() fn exits with an error, because of an issue establishing a connection.
	// 2. After successfully establishing a connection, ForwardPorts() closes the ready channel.
	//
	// This allows us to effectivly block until either the connection is ready, or a failure
	// occured while setting it up.
	select {
	case err := <-errChan:
		close(readyChannel)
		return nil, err
	case <-readyChannel:
		return errChan, nil
	}
}

// Given a service and service port, returns a backing pod name and pod port that match the provided service.
// Returns an error if a pod or port matching could not be determined.
func getPodNameAndRemotePort(client k8Client, req PortForwardRequest) (string, string, error) {
	podName, err := getPodNameFromServiceEndpoints(client, req)
	if err != nil {
		return "", "", err
	}
	targetPort, err := getPodPortFromServicePort(client, req)
	if err != nil {
		return "", "", err
	}
	return podName, targetPort, nil
}

// Returns the pod port that maps to the requested service port.
func getPodPortFromServicePort(client k8Client, req PortForwardRequest) (string, error) {
	service, err := client.Service(req.Namespace, req.Service)
	if err != nil {
		return "", err
	}

	if len(service.Spec.Ports) == 0 {
		return "", fmt.Errorf("No ports found for %s", req.Service)
	}

	portInt, err := strconv.Atoi(req.Port)
	if err != nil {
		return "", err
	}

	for _, port := range service.Spec.Ports {
		if port.Port == int32(portInt) {
			return strconv.Itoa(int(port.TargetPort.IntVal)), nil
		}
	}

	return "", fmt.Errorf("No ports found for %s", req.Service)
}

// Returns the first pod name from the endpoints on the requested service.
// If no pods could be found return error.
func getPodNameFromServiceEndpoints(client k8Client, req PortForwardRequest) (string, error) {
	endpoints, err := client.Endpoints(req.Namespace, req.Service)
	if err != nil {
		return "", err
	}
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			reference := *address.TargetRef
			if reference.Kind != "Pod" {
				continue
			}
			return reference.Name, nil
		}
	}
	return "", fmt.Errorf("No healthy pods found for service %s", req.Service)
}

// Use net/listen to pick a randomly available port for us to use
func getAvailablePort() (string, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", err
	}
	defer listener.Close()

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return "", err
	}
	return port, nil
}

func restConfigWithDefaults(config rest.Config) rest.Config {
	gv := v1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/api"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	return config
}

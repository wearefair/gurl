package k8

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"go.uber.org/zap"

	"github.com/wearefair/gurl/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

var (
	logger = log.Logger()
)

const (
	defaultNamespace = "default"
)

type K8 struct {
	Client kubernetes.Interface
	Config clientcmd.ClientConfig
	//	Config       *rest.Config
	StopChannel  chan struct{}
	ReadyChannel chan struct{}
}

//func New(kubeconfig string) (*K8, error) {
//	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
//	if err != nil {
//		return nil, log.WrapError(err)
//	}
//	clientset, err := kubernetes.NewForConfig(cfg)
//	if err != nil {
//		return nil, log.WrapError(err)
//	}
//	return &K8{
//		Client: clientset,
//		Config: cfg,
//	}, nil
//}

// NewK8 takes a Kubernetes context and constructs a K8 client set up for that
// context to make calls
func NewK8(context string) (*K8, error) {
	// https://godoc.org/k8s.io/client-go/tools/clientcmd
	config := k8Config(context)
	clientCfg, err := config.ClientConfig()
	if err != nil {
		return nil, log.WrapError(err)
	}
	clientset, err := kubernetes.NewForConfig(clientCfg)
	if err != nil {
		return nil, log.WrapError(err)
	}
	return &K8{
		Client: clientset,
		Config: config,
	}, nil
}

// GetPodNameAndRemotePort takes a service name and a port as a string and then returns
// the pod name, the target port for the pod or an error
// TODO: Use endpoint for pod name
// Use service for targetPort for pod
func (k *K8) GetPodNameAndRemotePort(serviceName, port string) (podName string, targetPort string, err error) {
	podName, err = k.getPodNameFromEndpoint(serviceName)
	if err != nil {
		return
	}
	targetPort, err = k.getPodTargetPort(serviceName, port)
	if err != nil {
		return
	}
	return
}

func (k *K8) getPodTargetPort(serviceName, port string) (string, error) {
	service, err := k.Client.CoreV1().Services(defaultNamespace).Get(serviceName, metav1.GetOptions{})
	if err != nil {
		return "", log.WrapError(err)
	}
	ports := service.Spec.Ports
	if len(ports) < 1 {
		err := fmt.Errorf("No ports found for %s", serviceName)
		return "", log.WrapError(err)
	}
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return "", log.WrapError(err)
	}
	for _, port := range ports {
		if port.Port == int32(portInt) {
			return strconv.Itoa(int(port.TargetPort.IntVal)), nil
		}
	}
	err = fmt.Errorf("No ports found for %s", serviceName)
	return "", log.WrapError(err)
}

// Just returning the first pod name found
func (k *K8) getPodNameFromEndpoint(serviceName string) (string, error) {
	endpoint, err := k.Client.CoreV1().Endpoints(defaultNamespace).Get(serviceName, metav1.GetOptions{})
	if err != nil {
		return "", log.WrapError(err)
	}
	for _, subset := range endpoint.Subsets {
		for _, address := range subset.Addresses {
			reference := *address.TargetRef
			if reference.Kind != "Pod" {
				continue
			}
			return reference.Name, nil
		}
	}
	return "", nil
}

func (k *K8) Forward(podName string, localPort, remotePort string) (chan error, error) {
	logger.Debug("Port forwarding", zap.String("pod", podName), zap.String("local-port", localPort), zap.String("remote-port", remotePort))
	// f := cmdutil.NewFactory(nil)
	conf, err := k.Config.ClientConfig()
	if err != nil {
		return nil, log.WrapError(err)
	}
	restClient, err := rest.RESTClientFor(conf)
	if err != nil {
		return nil, log.WrapError(err)
	}

	req := restClient.Post().Resource("pods").Namespace(defaultNamespace).Name(podName).SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(conf)
	if err != nil {
		return nil, log.WrapError(err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	fw, err := portforward.New(dialer,
		[]string{localPort, remotePort},
		k.StopChannel,
		k.ReadyChannel,
		os.Stdout,
		os.Stderr,
	)
	if err != nil {
		return nil, log.WrapError(err)
	}
	errChan := make(chan error)
	go func() {
		errChan <- fw.ForwardPorts()
	}()
	return errChan, nil
}

func k8Config(context string) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		configOverrides.CurrentContext = context
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
}

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
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

var (
	logger = log.Logger()
)

const (
	defaultNamespace = "default"
)

type K8 struct {
	Client       kubernetes.Interface
	Config       *rest.Config
	StopChannel  chan struct{}
	ReadyChannel chan struct{}
}

func New(kubeconfig string) (*K8, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, log.WrapError(err)
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, log.WrapError(err)
	}
	return &K8{
		Client:       clientset,
		Config:       cfg,
		StopChannel:  make(chan struct{}, 1),
		ReadyChannel: make(chan struct{}, 1),
	}, nil
}

// Use endpoint for pod name
// Use service for targetPort for pod
func (k *K8) GetPodNameAndRemotePort(serviceName, port string) (string, string, error) {
	podName, err := k.getPodNameFromEndpoint(serviceName)
	if err != nil {
		return "", "", err
	}
	targetPort, err := k.getPodTargetPort(serviceName, port)
	if err != nil {
		return "", "", err
	}
	return podName, targetPort, nil
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

// TODO: https://github.com/kubernetes/kubernetes/blob/dd9981d038012c120525c9e6df98b3beb3ef19e1/pkg/kubectl/cmd/portforward_test.go
// https://github.com/kubernetes/client-go/blob/master/rest/config.go#L88 -> Client config that we need to refer to and clean the TLS configurations from
// We can probably construct the TLS configs from this - https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/client/kubelet_client.go#L68 maybe?
// Looks like since we have a bearer token, we can construct the roundtripper from this - https://github.com/kubernetes/client-go/blob/master/transport/round_trippers.go#L304
// SPDY is deprecated, apparently - https://github.com/kubernetes-incubator/client-python/issues/58 - needs more investigation, but I don't necessarily think this is the issue - we may just have to construct the restClient.Config another way
// https://github.com/kubernetes/kubernetes/issues/7452 - so how do we portforward without spdy
// Portforwarding via websockets - https://github.com/kubernetes/kubernetes/pull/50428
// https://github.com/kubernetes/features/issues/384
// https://github.com/kubernetes/kubernetes/blob/v1.7.0/pkg/kubectl/cmd/portforward.go - How it's done in the CLI tool
func (k *K8) Forward(podName string, localPort, remotePort string) error {
	logger.Debug("Port forwarding", zap.String("pod", podName), zap.String("local-port", localPort), zap.String("remote-port", remotePort))
	f := cmdutil.NewFactory(nil)
	//clientset, err := f.ClientSet()
	//if err != nil {
	//	return log.WrapError(err)
	//}
	//podClient := clientset.Core()
	conf, err := f.ClientConfig()
	if err != nil {
		return log.WrapError(err)
	}
	restClient, err := f.RESTClient()
	if err != nil {
		return log.WrapError(err)
	}

	//_, err = podClient.Pods(defaultNamespace).Get(podName, metav1.GetOptions{})
	//if err != nil {
	//	return log.WrapError(err)
	//}
	req := restClient.Post().Resource("pods").Namespace(defaultNamespace).Name(podName).SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(conf)
	if err != nil {
		return log.WrapError(err)
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
		return log.WrapError(err)
	}
	errChan := make(chan error)
	go func() {
		errChan <- fw.ForwardPorts()
	}()
	if err := <-errChan; err != nil {
		return log.WrapError(err)
	}
	return nil
}

func k8Config() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
}

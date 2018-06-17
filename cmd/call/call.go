package call

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/spf13/cobra"
	"github.com/wearefair/gurl/pkg/config"
	"github.com/wearefair/gurl/pkg/k8"
	"github.com/wearefair/gurl/pkg/log"
	"github.com/wearefair/gurl/pkg/options"
	"github.com/wearefair/gurl/pkg/protobuf"
	"github.com/wearefair/gurl/pkg/util"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	logger = log.Logger()
	data   string
	// host:port/service_name/method_name
	port int
	uri  string

	callOptions = &options.Options{}
	tlsOptions  = &options.TLS{}
	useTls      bool
)

// RootCmd represents the base command when called without any subcommands
var CallCmd = &cobra.Command{
	Use:   "gurl",
	Short: "Curl your gRPC services",
	RunE:  runCall,
}

func init() {
	flags := CallCmd.Flags()

	flags.StringVarP(&uri, "uri", "u", "", "gRPC URI in the form of host:port/service_name/method_name")
	flags.StringVarP(&data, "data", "d", "", "Data, as JSON string, to send to the gRPC service")
	CallCmd.MarkFlagRequired("uri")
	CallCmd.MarkFlagRequired("data")

	// TLS Options
	flags.BoolVarP(&useTls, "tls", "t", false, "Use TLS to connect to the server")
	flags.BoolVarP(&tlsOptions.Insecure, "tls-insecure", "k", false, "Skip verification of server TLS certificate.")
	flags.StringVarP(&tlsOptions.ServerName, "tls-servername", "N", "", "Override the server name used for the TLS handshake.")
}

func runCall(cmd *cobra.Command, args []string) error {
	if useTls {
		callOptions.TLS = tlsOptions
	}
	// Parse and return the URI in a format we can expect
	parsedURI, err := util.ParseURI(uri)
	if err != nil {
		return err
	}
	logger.Debug("Parsed URI", zap.Any("uri", parsedURI))

	// Walks the proto import and service paths defined in the config and returns all descriptors
	descriptors, err := protobuf.Collect(config.Instance().Local.ImportPaths, config.Instance().Local.ServicePaths)
	if err != nil {
		return err
	}

	// Get the service descriptor that was set in the URI
	collector := protobuf.NewCollector(descriptors)
	serviceDescriptor, err := collector.GetService(parsedURI.Service)
	if err != nil {
		return err
	}

	// Find the RPC attached to the service via the URI
	methodDescriptor := serviceDescriptor.FindMethodByName(parsedURI.RPC)
	if methodDescriptor == nil {
		err := fmt.Errorf("No method %s found", parsedURI.RPC)
		return log.WrapError(err)
	}

	methodProto := methodDescriptor.AsMethodDescriptorProto()
	messageDescriptor, err := collector.GetMessage(
		protobuf.NormalizeMessageName(*methodProto.InputType),
	)
	if err != nil {
		return err
	}

	message, err := protobuf.Construct(messageDescriptor, data)
	if err != nil {
		return err
	}

	if parsedURI.Protocol == util.K8Protocol {
		// Set up port forward, then send request
		req := uriToPortForwardRequest(parsedURI)
		pf, err := k8.StartPortForward(k8Config(), req)
		if err != nil {
			return err
		}
		defer pf.Close()
		// TODO: Don't mutate the state of the URI and pass it down, that's not great.
		parsedURI.Port = pf.LocalPort()
	}

	// Send request and get response
	response, err := sendRequest(parsedURI, methodDescriptor, message)
	if err != nil {
		return err
	}

	// Prettifying JSON of response
	var prettyResponse bytes.Buffer
	err = json.Indent(&prettyResponse, response, "", "  ")
	if err != nil {
		return log.WrapError(err)
	}
	fmt.Printf("Response:\n%s\n", prettyResponse.String())
	return nil
}

// sendRequest takes in the uri, the actual method descriptor, and the dynamically constructed message to send
func sendRequest(uri *util.URI, methodDescriptor *desc.MethodDescriptor, message proto.Message) ([]byte, error) {
	// TODO: A lot of this logic should get pulled out
	address := formatAddress(uri)
	logger.Debug("Dialing request", zap.String("address", address))
	clientConn, err := grpc.Dial(address, callOptions.DialOptions()...)
	if err != nil {
		return nil, log.WrapError(err)
	}
	stub := grpcdynamic.NewStub(clientConn)
	methodProto := methodDescriptor.AsMethodDescriptorProto()

	// Disabled server and client streaming calls
	disableStreaming := false
	methodProto.ClientStreaming = &disableStreaming
	methodProto.ServerStreaming = &disableStreaming
	response, err := stub.InvokeRpc(context.Background(), methodDescriptor, message)
	if err != nil {
		return nil, log.WrapError(err)
	}
	marshaler := &runtime.JSONPb{}
	// Marshals PB response into JSON
	responseJSON, err := marshaler.Marshal(response)
	if err != nil {
		return nil, log.WrapError(err)
	}
	return responseJSON, nil
}

// Helper func to format an address. Right now, this is only needed because K8
// requests get locked to localhost for port-forwarding.
func formatAddress(uri *util.URI) string {
	if uri.Protocol == util.K8Protocol {
		return fmt.Sprintf("localhost:%s", uri.Port)
	}
	return fmt.Sprintf("%s:%s", uri.Host, uri.Port)
}

// Reads K8 config from default location, which is $HOME/.kube/config
func k8Config() clientcmd.ClientConfig {
	// if you want to change the loading rules (which files in which order), you can do so here
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	// if you want to change override values or bind them to flags, there are methods to help you
	configOverrides := &clientcmd.ConfigOverrides{}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
}

func uriToPortForwardRequest(uri *util.URI) k8.PortForwardRequest {
	return k8.PortForwardRequest{
		Context: uri.Context,
		// TODO: Make this namespace configurable via URI
		Namespace: "default",
		Service:   uri.Host,
		Port:      uri.Port,
	}
}

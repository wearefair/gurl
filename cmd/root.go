package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/golang/protobuf/proto"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/spf13/cobra"
	"github.com/wearefair/gurl/config"
	cligrpc "github.com/wearefair/gurl/grpc"
	"github.com/wearefair/gurl/k8"
	"github.com/wearefair/gurl/log"
	"github.com/wearefair/gurl/util"
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
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "gurl",
	Short: "Curl your gRPC services",
	RunE:  gurl,
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	RootCmd.Flags().StringVarP(&uri, "uri", "u", "", "gRPC URI in the form of host:port/service_name/method_name")
	RootCmd.Flags().StringVarP(&data, "data", "d", "", "Data, as JSON string, to send to the gRPC service")
	RootCmd.MarkFlagRequired("uri")
	RootCmd.MarkFlagRequired("data")
}

func initConfig() {
	config.Read()
}

func gurl(cmd *cobra.Command, args []string) error {
	// Parse and return the URI in a format we can expect
	parsedURI, err := util.ParseURI(uri)
	if err != nil {
		return err
	}
	logger.Debug("Parsed URI", zap.Any("uri", parsedURI))

	// Walks the proto import and service paths defined in the config and returns all descriptors
	descriptors, err := cligrpc.Collect(config.Instance().Local.ImportPaths, config.Instance().Local.ServicePaths)
	if err != nil {
		return err
	}

	// Get the service descriptor that was set in the URI
	collector := cligrpc.NewCollector(descriptors)
	serviceDescriptor, err := collector.GetService(parsedURI.Service)
	if err != nil {
		return err
	}

	// Find the RPC attached to the service via the URI
	methodDescriptor := serviceDescriptor.FindMethodByName(parsedURI.RPC)
	if methodDescriptor == nil {
		err := fmt.Errorf("No method %s found", parsedURI.RPC)
		logger.Error(err.Error())
		return err
	}

	methodProto := methodDescriptor.AsMethodDescriptorProto()
	messageDescriptor, err := collector.GetMessage(util.NormalizeMessageName(*methodProto.InputType))
	if err != nil {
		return err
	}

	message, err := cligrpc.Construct(messageDescriptor, data)
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
		// TODO: Determine if this is the best place.
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
	clientConn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return nil, log.WrapError(err)
	}
	stub := grpcdynamic.NewStub(clientConn)
	methodProto := methodDescriptor.AsMethodDescriptorProto()
	// Disabled server and client streaming calls
	methodProto.ClientStreaming = util.PointerifyBool(false)
	methodProto.ServerStreaming = util.PointerifyBool(false)
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

func formatAddress(uri *util.URI) string {
	if uri.Protocol == util.K8Protocol {
		logger.Info("K8 protocol detected")
		return fmt.Sprintf("localhost:%s", uri.Port)
	}
	return fmt.Sprintf("%s:%s", uri.Service, uri.Port)
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

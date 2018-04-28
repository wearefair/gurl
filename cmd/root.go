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
	"google.golang.org/grpc"
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
	RootCmd.Flags().StringVarP(&data, "data", "d", "", "Data, as JSON, to send to the gRPC service")
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

	// Walks the proto import and service paths defined in the config and returns all descriptors
	descriptors, err := cligrpc.Collect(config.Instance().Local.ImportPaths, config.Instance().Local.ServicePaths)
	if err != nil {
		return err
	}

	// Get the service descriptor by the RPC that was set in the URI
	collector := cligrpc.NewCollector(descriptors)
	serviceDescriptor, err := collector.GetService(parsedURI.RPC)
	if err != nil {
		return err
	}

	// Find the method attached to the service via the URI
	methodDescriptor := serviceDescriptor.FindMethodByName(parsedURI.Method)
	if methodDescriptor == nil {
		err := fmt.Errorf("No method %s found", parsedURI.Method)
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

	kube, err := k8.NewK8(parsedURI.Context)
	if err != nil {
		return err
	}

	response, err := sendRequest(kube, parsedURI, methodDescriptor, message)
	if err != nil {
		return err
	}
	var prettyResponse bytes.Buffer
	// Prettifying JSON
	err = json.Indent(&prettyResponse, response, "", "  ")
	if err != nil {
		return log.WrapError(err)
	}
	fmt.Printf("Response:\n%s\n", prettyResponse.String())
	return nil
}

// sendRequest takes in the uri, the actual method descriptor, and the dynamically constructed message to send
func sendRequest(kube *k8.K8, uri *util.URI, methodDescriptor *desc.MethodDescriptor, message proto.Message) ([]byte, error) {
	var pf *k8.PortForward
	address := formatAddress(kube, uri)
	// If protocol is K8, then set up portforwarding
	if uri.Protocol == util.K8Protocol {
		pf = k8.NewPortForward()
		podName, remotePort, err := kube.GetPodNameAndRemotePort(uri.Service, uri.Port)
		if err != nil {
			return nil, err
		}
		localPort, err := util.GetAvailablePort()
		if err != nil {
			return nil, err
		}
		req := &k8.PortForwardRequest{
			Namespace:  "default",
			Pod:        podName,
			LocalPort:  localPort,
			RemotePort: remotePort,
		}
		if err = pf.Forward(kube.Config, req); err != nil {
			return nil, err
		}
	}
	defer func() {
		if pf != nil {
			pf.Stop()
		}
	}()
	go func() {
		// TODO: This is ugly af, fix
		if pf != nil {
			for {
				err := <-pf.ErrorChannel
				if err != nil {
					logger.Error(err.Error())
					pf.Stop()
					break
				}
			}
		}
	}()
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

func formatAddress(kube *k8.K8, uri *util.URI) string {
	if uri.Protocol == util.K8Protocol {
		logger.Info("K8 protocol detected")
		return fmt.Sprintf("localhost:%s", uri.Port)
	}
	return fmt.Sprintf("%s:%s", uri.Service, uri.Port)
}

//func setupPortForwarding(kube *k8.K8, uri *util.URI) (chan error, error) {
//	podName, remotePort, err := kube.GetPodNameAndRemotePort(uri.Service, uri.Port)
//	if err != nil {
//		return nil
//	}
//	kube.Forward(podName, uri.Port, remotePort)
//}

//func closePortForwarding(kube *k8.K8) {
//	kube.StopChannel <- struct{}{}
//}

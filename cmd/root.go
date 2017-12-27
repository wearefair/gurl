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
	RunE:  curl,
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
	RootCmd.Flags().IntVarP(&port, "port", "p", 0, "Local port to set up remote forwarding. Will default to the port set in the uri if not set here")
	RootCmd.Flags().StringVarP(&uri, "uri", "u", "", "gRPC URI in the form of host:port/service_name/method_name")
	RootCmd.Flags().StringVarP(&data, "data", "d", "", "Data, as JSON, to send to the gRPC service")
	RootCmd.MarkFlagRequired("uri")
	RootCmd.MarkFlagRequired("data")
}

func initConfig() {
	config.Read()
}

func curl(cmd *cobra.Command, args []string) error {
	kube, err := k8.New(config.Instance().KubeConfig)
	if err != nil {
		return err
	}
	uriWrapper, err := util.ParseURI(uri)
	if err != nil {
		return err
	}

	descriptors, err := cligrpc.Collect(config.Instance().Local.ImportPaths, config.Instance().Local.ServicePaths)
	if err != nil {
		return err
	}

	collector := cligrpc.NewCollector(descriptors)
	serviceDescriptor, err := collector.GetService(uriWrapper.RPC)
	if err != nil {
		return err
	}

	methodDescriptor := serviceDescriptor.FindMethodByName(uriWrapper.Method)
	if methodDescriptor == nil {
		err := fmt.Errorf("No method %s found", uriWrapper.Method)
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
	response, err := sendRequest(kube, uriWrapper, methodDescriptor, message)
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

func sendRequest(kube *k8.K8, uri *util.URI, methodDescriptor *desc.MethodDescriptor, message proto.Message) ([]byte, error) {
	address, err := formatAddress(kube, uri)
	defer closePortForwarding(kube)
	if err != nil {
		return nil, err
	}
	// Figure out auth later
	clientConn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return nil, log.WrapError(err)
	}
	stub := grpcdynamic.NewStub(clientConn)
	methodProto := methodDescriptor.AsMethodDescriptorProto()
	// TODO: Handle different cases for client, server, and bidi streaming
	methodProto.ClientStreaming = util.PointerifyBool(false)
	methodProto.ServerStreaming = util.PointerifyBool(false)
	response, err := stub.InvokeRpc(context.Background(), methodDescriptor, message)
	if err != nil {
		return nil, log.WrapError(err)
	}
	marshaler := &runtime.JSONPb{}
	responseJSON, err := marshaler.Marshal(response)
	if err != nil {
		return nil, log.WrapError(err)
	}
	return responseJSON, nil
}

func formatAddress(kube *k8.K8, uri *util.URI) (string, error) {
	if uri.Protocol == util.K8Protocol {
		// Setup port forwarding - localhost and port
		err := setupPortForwarding(kube, uri)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("localhost:%s", uri.Port), nil
	}
	return fmt.Sprintf("%s:%s", uri.Service, uri.Port), nil
}

func setupPortForwarding(kube *k8.K8, uri *util.URI) error {
	podName, remotePort, err := kube.GetPodNameAndRemotePort(uri.Service, uri.Port)
	if err != nil {
		return err
	}
	return kube.Forward(podName, uri.Port, remotePort)
}

func closePortForwarding(kube *k8.K8) {
	kube.StopChannel <- struct{}{}
}

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/golang/protobuf/proto"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/spf13/cobra"
	"github.com/wearefair/gurl/config"
	cligrpc "github.com/wearefair/gurl/grpc"
	"github.com/wearefair/gurl/log"
	"github.com/wearefair/gurl/util"
	"google.golang.org/grpc"
)

const (
	// Regexp that matches the expected uri structure - allows for -_. as special characters
	uriRegex = `([a-z]+)(?:\:)([0-9]{5})(?:\/)([0-9a-zA-Z._-]+)(?:\/)([0-9a-zA-Z._-]+)`
)

var (
	logger = log.Logger()
	data   string
	// host:port/service_name/method_name
	uri       string
	uriRegexp = regexp.MustCompile(uriRegex)
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
	RootCmd.Flags().StringVarP(&uri, "uri", "u", "", "gRPC URI in the form of host:port/service_name/method_name")
	RootCmd.Flags().StringVarP(&data, "data", "d", "", "Data, as JSON, to send to the gRPC service")
	RootCmd.MarkFlagRequired("uri")
	RootCmd.MarkFlagRequired("data")
}

func initConfig() {
	config.Read()
}

func curl(cmd *cobra.Command, args []string) error {
	uriWrapper, err := util.ParseURI(uri)
	if err != nil {
		return err
	}

	localwalker := cligrpc.NewProtoWalker()
	servicewalker := cligrpc.NewProtoWalker()
	constructor := cligrpc.NewConstructor()

	localwalker.Collect(config.Instance().Local.ImportPaths)
	servicewalker.Collect(config.Instance().Local.ServicePaths)

	collector := cligrpc.NewCollector(localwalker.GetFileDescriptors())

	// Register service types into known type registry so they can be constructed properly
	constructor.RegisterFileDescriptors(servicewalker.GetFileDescriptors())

	serviceDescriptor, err := collector.GetService(uriWrapper.Service)
	if err != nil {
		logger.Error(err.Error())
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
		err := fmt.Errorf("No message %s found", util.NormalizeMessageName(*methodProto.InputType))
		logger.Error(err.Error())
		return err
	}
	message, err := constructor.Construct(messageDescriptor, data)
	if err != nil {
		return err
	}
	response, err := sendRequest(uriWrapper, methodDescriptor, message)
	if err != nil {
		return err
	}
	var prettyResponse bytes.Buffer
	// Prettifying JSON
	err = json.Indent(&prettyResponse, response, "", "  ")
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	fmt.Printf("Response:\n%s\n", prettyResponse.String())
	return nil
}

func sendRequest(uri *util.URI, methodDescriptor *desc.MethodDescriptor, message proto.Message) ([]byte, error) {
	address := fmt.Sprintf("%s:%s", uri.Host, uri.Port)
	// Figure out auth later
	clientConn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	stub := grpcdynamic.NewStub(clientConn)
	methodProto := methodDescriptor.AsMethodDescriptorProto()
	// TODO: Handle different cases for client, server, and bidi streaming
	methodProto.ClientStreaming = util.PointerifyBool(false)
	methodProto.ServerStreaming = util.PointerifyBool(false)
	response, err := stub.InvokeRpc(context.Background(), methodDescriptor, message)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	marshaler := &runtime.JSONPb{}
	responseJSON, err := marshaler.Marshal(response)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return responseJSON, nil
}

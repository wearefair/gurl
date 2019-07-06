package jsonpb

import (
	"context"
	"fmt"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/wearefair/gurl/pkg/protobuf"
)

// Client handles constructing and dialing a gRPC service
type Client struct {
	collector *protobuf.Collector
}

// NewClient creates a client with a Stub
func NewClient(cfg *Config) (*Client, error) {
	// Walks the proto import and service paths defined in the config and returns all descriptors
	descriptors, err := protobuf.Collect(cfg.ImportPaths, cfg.ServicePaths)
	if err != nil {
		return nil, err
	}

	return &Client{
		collector: protobuf.NewCollector(descriptors),
	}, nil
}

// Invoke makes a unary call across the wire.
func (c *Client) Invoke(ctx context.Context, req *Request) ([]byte, error) {
	// Run any connection logic
	conn, err := req.Connect()
	if err != nil {
		return nil, err
	}
	defer req.Close()

	stub := grpcdynamic.NewStub(conn)

	serviceDescriptor, err := c.collector.GetService(req.Service)
	if err != nil {
		return nil, err
	}

	// Find the RPC attached to the service via the URI
	methodDescriptor := serviceDescriptor.FindMethodByName(req.RPC)
	if methodDescriptor == nil {
		err := fmt.Errorf("No method %s found", req.Service)
		return nil, err
	}

	methodProto := methodDescriptor.AsMethodDescriptorProto()
	messageDescriptor, err := c.collector.GetMessage(
		protobuf.NormalizeMessageName(*methodProto.InputType),
	)
	if err != nil {
		return nil, err
	}

	message, err := protobuf.Construct(messageDescriptor, req.Message)
	if err != nil {
		return nil, err
	}

	// TODO: Allow for streaming calls. This locks us to unary calls
	// Disabled server and client streaming calls
	disableStreaming := false
	methodProto.ClientStreaming = &disableStreaming
	methodProto.ServerStreaming = &disableStreaming

	response, err := stub.InvokeRpc(ctx, methodDescriptor, message)
	if err != nil {
		return nil, err
	}

	marshaler := &runtime.JSONPb{}
	// Marshals PB response into JSON
	responseJSON, err := marshaler.Marshal(response)
	if err != nil {
		return nil, err
	}

	return responseJSON, nil
}

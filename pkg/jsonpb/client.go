package jsonpb

import (
	"context"
	"fmt"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/wearefair/gurl/pkg/protobuf"
	"google.golang.org/grpc"
)

// Client handles constructing and dialing a gRPC service
type Client struct {
	stub grpcdynamic.Stub
	// TODO: Might want to turn this into an interface?
	collector *protobuf.Collector
}

// NewClient creates a client with a Stub
func NewClient(cfg *Config) (*Client, error) {
	conn, err := grpc.Dial(cfg.Address, cfg.DialOptions...)
	if err != nil {
		return nil, err
	}
	// Walks the proto import and service paths defined in the config and returns all descriptors
	descriptors, err := protobuf.Collect(cfg.ImportPaths, cfg.ServicePaths)
	if err != nil {
		return nil, err
	}

	return &Client{
		stub:      grpcdynamic.NewStub(conn),
		collector: protobuf.NewCollector(descriptors),
	}, nil
}

// Call takes in a context, service, RPC, and message as JSON string to convert to protobuf and
// send across the wire.
func (c *Client) Call(ctx context.Context, service, rpc, rawMsg string) ([]byte, error) {
	serviceDescriptor, err := c.collector.GetService(service)
	if err != nil {
		return nil, err
	}

	// Find the RPC attached to the service via the URI
	methodDescriptor := serviceDescriptor.FindMethodByName(rpc)
	if methodDescriptor == nil {
		err := fmt.Errorf("No method %s found", service)
		return nil, err
	}

	methodProto := methodDescriptor.AsMethodDescriptorProto()
	messageDescriptor, err := c.collector.GetMessage(
		protobuf.NormalizeMessageName(*methodProto.InputType),
	)
	if err != nil {
		return nil, err
	}

	message, err := protobuf.Construct(messageDescriptor, rawMsg)
	if err != nil {
		return nil, err
	}

	// TODO: Allow for streaming calls. This locks us to unary calls
	// Disabled server and client streaming calls
	disableStreaming := false
	methodProto.ClientStreaming = &disableStreaming
	methodProto.ServerStreaming = &disableStreaming

	response, err := c.stub.InvokeRpc(ctx, methodDescriptor, message)
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

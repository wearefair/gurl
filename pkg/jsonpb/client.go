package jsonpb

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"google.golang.org/grpc"
)

// Client handles constructing and dialing a gRPC service
type Client struct {
	stub grpcdynamic.Stub
}

// NewClient creates a client with a Stub
func NewClient(cfg *Config) (*Client, error) {
	conn, err := grpc.Dial(cfg.Address, cfg.DialOptions...)
	if err != nil {
		return nil, err
	}
	return &Client{
		stub: grpcdynamic.NewStub(conn),
	}, nil
}

// Call takes in a method descriptor and a proto message and sends it across the wire
func (c *Client) Call(ctx context.Context, methodDescriptor *desc.MethodDescriptor, message proto.Message) ([]byte, error) {
	methodProto := methodDescriptor.AsMethodDescriptorProto()

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

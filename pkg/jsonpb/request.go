package jsonpb

import "google.golang.org/grpc"

// Request encompasses all of the args required to make a request
type Request struct {
	// The address the gRPC client will be making a call to
	Address string
	// DialOptions to pass down to the gRPC client
	DialOptions []grpc.DialOption

	// The FQDN of the gRPC service
	Service string
	// The method name of the RPC to target
	RPC string
	// The JSON message in bytes to send to the service
	Message []byte
}

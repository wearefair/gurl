package jsonpb

import (
	"google.golang.org/grpc"
)

// Request encompasses all of the args required to make a request
type Request struct {
	// Connector handles things like K8 portforward logic
	Connector Connector

	// Address is where something will connect to
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

func (r *Request) Connect() (*grpc.ClientConn, error) {
	var err error
	address := r.Address

	if r.Connector != nil {
		address, err = r.Connector.Connect()
		if err != nil {
			return nil, err
		}
	}

	return grpc.Dial(address, r.DialOptions...)
}

func (r *Request) Close() error {
	if r.Connector == nil {
		return nil
	}

	return r.Connector.Close()
}

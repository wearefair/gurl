package jsonpb

import (
	"google.golang.org/grpc"
)

// Config handles everything necessary to construct a Client
type Config struct {
	Address      string
	DialOptions  []grpc.DialOption
	ImportPaths  []string
	ServicePaths []string
}

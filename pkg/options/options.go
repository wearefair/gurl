package options

import (
	"context"
	"crypto/tls"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

type Options struct {
	Metadata   metadata.MD
	TLS        *TLS
	Kubernetes *Kubernetes
}

func (o Options) DialOptions() []grpc.DialOption {
	options := make([]grpc.DialOption, 0)

	if o.TLS == nil {
		options = append(options, grpc.WithInsecure())
	} else {
		options = append(options, grpc.WithTransportCredentials(credentials.NewTLS(o.TLS.Config())))
	}

	return options
}

func (o Options) ContextWithOptions(ctx context.Context) context.Context {
	if len(o.Metadata) != 0 {
		return metadata.NewOutgoingContext(ctx, o.Metadata)
	}

	return ctx
}

type TLS struct {
	Insecure   bool
	ServerName string
}

func (t TLS) Config() *tls.Config {
	config := &tls.Config{}

	config.InsecureSkipVerify = t.Insecure
	config.ServerName = t.ServerName

	return config
}

type Kubernetes struct {
	Context   string
	Namespace string
}

package util

import (
	"errors"
	"regexp"

	"github.com/wearefair/gurl/pkg/log"
)

const (
	// K8Protocol just represents the protocol used in order to specify that
	// gurl should forward requests to Kubernetes.
	K8Protocol = "k8"
	// TODO: Add support for Kubernetes namespaces.
	//
	// Regexp for extracting the information out of a URI that's passed to gurl
	// in order to properly form a request.
	//
	// The current structure of the regexp:
	// protocol://context/service:port/grpc/rpc/method
	//
	// Protocol - Optional parameter. Currently supports http and k8.
	// k8 will signify to gurl that the request needs to be forwarded
	// to Kubernetes. If not specified, it will default to http
	//
	// Context - Optional parameter for Kubernetes context. If not specified
	// it will default to the default context.
	//
	// Host - Hostname to direct requests. When the request uses the k8://
	// protocol, this will be the K8 service name.
	//
	// Port - Port to direct requests. This will be the service port to target
	// if using the k8 protocol.
	//
	// Service - The FQDN of the gRPC service that you're targeting. This means
	// if you're targeting a service called FooBar in the hello package, you would use
	// hello.FooBar
	//
	// RPC - The name of the RPC to direct the request towards.
	//
	// Examples of valid URI for gurl:
	// http://localhost:50051/hello.world.package.Foo/Bar
	// k8://fake-context/foo-service:50051/hello.world.package.Foo/Bar
	uriRegex = `((?P<protocol>[a-z0-9]{2,5})(?:\:\/\/)((?P<context>[0-9a-z._-]+)(?:\/))?)?(?P<host>[0-9a-z-_.]+)(?:\:)(?P<port>[0-9]{2,5})(?:\/)(?P<service>[0-9a-zA-Z._-]+)(?:\/)(?P<rpc>[0-9a-zA-Z._-]+)`
)

var (
	errInvalidURIFormat = errors.New("URI must be in the form of protocol://host:port/rpc/method")
	uriRegexp           = regexp.MustCompile(uriRegex)
)

// URI represents a deconstructed uri structure
type URI struct {
	Protocol string
	Context  string
	Host     string
	Port     string
	Service  string
	RPC      string
}

// ParseURI parses a string URI against the regexp and returns it in an expected format
func ParseURI(uri string) (*URI, error) {
	namedMatches := make(map[string]string)
	// host:port/service/method
	submatches := uriRegexp.FindStringSubmatch(uri)
	if submatches == nil {
		return nil, log.WrapError(errInvalidURIFormat)
	}

	for i, name := range uriRegexp.SubexpNames() {
		// The first index is going to be the whole match
		if i == 0 || name == "" {
			continue
		}
		namedMatches[name] = submatches[i]
	}
	if len(namedMatches) < 1 {
		return nil, log.WrapError(errInvalidURIFormat)
	}
	uriWrapper := &URI{
		Protocol: namedMatches["protocol"],
		Context:  namedMatches["context"],
		Host:     namedMatches["host"],
		Port:     namedMatches["port"],
		Service:  namedMatches["service"],
		RPC:      namedMatches["rpc"],
	}
	return uriWrapper, nil
}

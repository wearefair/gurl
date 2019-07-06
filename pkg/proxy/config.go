package proxy

import (
	"net/http"

	"github.com/gorilla/mux"
	logmw "github.com/wearefair/gurl/pkg/middleware/log"
	"github.com/wearefair/gurl/pkg/options"
	"google.golang.org/grpc/metadata"
)

// Config wraps all configs for the proxy
type Config struct {
	// Addr is the address that the proxy will run at
	Addr string
	// Options to pass down to the jsonpb client
	Options *options.Options
	// Caller is what's used to pull in the import and service paths for handling
	// the JSON to protobuf behavior
	Caller Caller
	//	// ImportPaths are the import paths to pass down to the jsonpb client
	//	ImportPaths []string
	//	// ServicePaths are the service paths to pass down to the jsonpb client
	//	ServicePaths []string
	// Middlewares you want to register with the router
	Middlewares []func(http.Handler) http.Handler
	// You can override the ProxyTargetHeader to target calls at, if desired.
	ProxyTargetHeader string
	// You can override the default handler. This should only be passed in if
	// you don't want to override the Router.
	ProxyHandler http.HandlerFunc
	// If you want to override the default mux.Router. All of the middleware
	// passed in will be registered on the router on creation of the Proxy struct.
	Router *mux.Router
}

// DefaultConfig returns a sane base template for Proxy configs
func DefaultConfig() *Config {
	return &Config{
		Addr: ":3030",
		Middlewares: []func(http.Handler) http.Handler{
			logmw.Middleware,
		},
		Options:           &options.Options{Metadata: metadata.MD{}},
		Router:            mux.NewRouter(),
		ProxyTargetHeader: DefaultProxyTargetHeader,
	}
}

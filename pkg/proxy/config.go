package proxy

import (
	"net/http"

	"github.com/gorilla/mux"
	logmw "github.com/wearefair/gurl/pkg/middleware/log"
)

// Config wraps all configs for the proxy
type Config struct {
	// Addr is the address that the proxy will run at
	Addr string
	// Middlewares you want to register with the router
	Middlewares []func(http.Handler) http.Handler
	// You can overwrite the ProxyTargetHeader to target calls at, if desired.
	ProxyTargetHeader string
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
		Router:            mux.NewRouter(),
		ProxyTargetHeader: DefaultProxyTargetHeader,
	}
}

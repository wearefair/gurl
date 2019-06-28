package proxy

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/wearefair/gurl/pkg/options"
)

// Proxy encapsulates all gURL proxy server logic
type Proxy struct {
	router            *mux.Router
	server            *http.Server
	opts              *options.Options
	proxyTargetHeader string

	importPaths  []string
	servicePaths []string
}

// New returns an instance of Proxy
func New(cfg *Config) *Proxy {
	configureMiddleware(cfg.Router, cfg.Middlewares)

	s := &http.Server{
		Addr:    cfg.Addr,
		Handler: cfg.Router,
	}

	return &Proxy{
		opts:              cfg.Options,
		proxyTargetHeader: cfg.ProxyTargetHeader,
		router:            cfg.Router,
		server:            s,

		importPaths:  cfg.ImportPaths,
		servicePaths: cfg.ServicePaths,
	}
}

// Run the proxy server
func (p *Proxy) Run() error {
	p.configure()
	return p.server.ListenAndServe()
}

// Proxy configure configures the handler and routes
func (p *Proxy) configure() {
	p.router.HandleFunc("/{service}/{rpc}", p.Handler)
}

func configureMiddleware(r *mux.Router, middlewares []func(http.Handler) http.Handler) {
	for _, middleware := range middlewares {
		r.Use(middleware)
	}
}

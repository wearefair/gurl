package proxy

import (
	"net/http"

	"github.com/gorilla/mux"
)

// Proxy encapsulates all gURL proxy server logic
type Proxy struct {
	router            *mux.Router
	server            *http.Server
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
		proxyTargetHeader: cfg.ProxyTargetHeader,
		router:            cfg.Router,
		server:            s,
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

package proxy

import (
	"net/http"

	"github.com/gorilla/mux"
)

// Config wraps all configs for the proxy
type Config struct {
	Addr         string
	RouteHandler func(http.ResponseWriter, *http.Request)
}

// Proxy encapsulates all gURL proxy server logic
type Proxy struct {
	router  *mux.Router
	server  *http.Server
	handler func(http.ResponseWriter, *http.Request)
}

// New returns an instance of Proxy
func New(cfg *Config) *Proxy {
	// TODO: Allow for overriding of the handler
	r := mux.NewRouter()
	s := &http.Server{
		Addr:    cfg.Addr,
		Handler: r,
	}
	return &Proxy{
		router: r,
		server: s,
	}
}

// Configures routes
func (p *Proxy) configure() error {
	p.router.HandleFunc("/{service}/{method}", p.handler)
	return nil
}

// Run the proxy server
func (p *Proxy) Run() error {
	return p.server.ListenAndServe()
}

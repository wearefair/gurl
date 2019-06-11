package proxy

import (
	"net/http"

	"github.com/gorilla/mux"
	logmw "github.com/wearefair/gurl/pkg/middleware/log"
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
		router:  r,
		server:  s,
		handler: cfg.RouteHandler,
	}
}

// Configures routes
func (p *Proxy) configure() {
	p.router.HandleFunc("/{service}/{method}", p.handler).Methods("POST")
	p.router.Use(logmw.Middleware)
}

// Run the proxy server
func (p *Proxy) Run() error {
	p.configure()
	return p.server.ListenAndServe()
}

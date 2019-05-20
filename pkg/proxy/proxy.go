package proxy

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/wearefair/gurl/pkg/log"
)

// Proxy ...
type Proxy struct {
	router *mux.Router
	port   string
}

// New creates an instance of the Proxy
func New() *Proxy {
	proxy := &Proxy{
		router: mux.NewRouter(),
		// TODO:
		port: ":3030",
	}

	// TODO: Pull this route out
	proxy.router.PathPrefix("/").HandlerFunc(proxyHandler)
	return proxy
}

// Run ...
func (p *Proxy) Run() error {
	log.Infof("Starting server at: %s", p.port)
	return http.ListenAndServe(p.port, p.router)
}

func proxyHandler(wr http.ResponseWriter, req *http.Request) {
	url := req.URL.String()
	log.Infof("[proxyHandler] - url: %s", url)
}

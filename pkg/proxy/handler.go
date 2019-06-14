package proxy

import (
	"context"
	"io/ioutil"
	"net/http"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
)

const (
	ServiceKey = "service"
	RpcKey     = "rpc"
)

// Handler takes in a route resembling the following
// format: <host>:<port>/<gRPC service fully qualified domain name>/<RPC>.
// Ex: localhost:50051/fairapis.pubsub.v1.Publish/ListTopics
// It pulls the service and the RPC name off of the route, generates
// the proto message, and then routes it to the proper destination.
// The destination must be set in the headers under the proxy target header
// which defaults to x-gurl-proxy-target
func (p *Proxy) Handler(rw http.ResponseWriter, req *http.Request) {
	// Pull the host off the headers
	headers := req.Header.Get(p.proxyTargetHeader)

	// If the header is not set... return a 422, because we really
	// can't process this request. Where are we supposed to forward this to?
	if headers == "" {
		rw.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	defer req.Body.Close()
	msg, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	// Pull the variables off of the request
	vars := mux.Vars(req)
	// TODO: Remove
	spew.Dump(vars)

	// TODO: Validation errors on either one of these if they're nil
	service := vars[ServiceKey]
	rpc := vars[RpcKey]

	response, err := p.client.Call(context.Background(), service, rpc, msg)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	rw.WriteHeader(http.StatusOK)
	rw.Write(response)
}

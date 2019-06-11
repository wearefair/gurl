package proxy

import (
	"net/http"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
)

// RPCRouteHandler takes in a route resembling the following
// /<gRPC service fully qualified domain name>/<RPC>
// localhost:50051/fairapis.pubsub.v1.Publish/ListTopics
// It pulls the service and the RPC name off of the route, generates
// the proto message, and then routes it to the proper destination.
// The destination must be set in the headers under the key
// 'x-proxy-host'.
func RPCRouteHandler(rw http.ResponseWriter, req *http.Request) {
	// Pull the host off the headers
	headers := req.Header.Get(ProxyTargetHeader)

	// If the header is not set... return a 422, because we really
	// can't process this request.
	if headers == "" {
		rw.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	// Pull the variables off of the request
	vars := mux.Vars(req)

	spew.Dump(vars)

	rw.WriteHeader(http.StatusOK)
}

package proxy

import (
	"net/http"

	"github.com/gorilla/mux"
)

// RPCRouteHandler takes in a route resembling the following
// <host>:<port>/<gRPC service fully qualified domain name>/<RPC>
// localhost:50051/fairapis.pubsub.v1.Publish/ListTopics
// It pulls the service and the RPC name off of the route, generates
// the proto message, and then routes it to the proper destination.
func RPCRouteHandler(rw http.ResponseWriter, req *http.Request) {
	// Pull the variables off of the request
	vars := mux.Vars(req)
}

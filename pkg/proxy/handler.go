package proxy

import (
	"context"
	"io/ioutil"
	"net/http"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
	"github.com/wearefair/gurl/pkg/jsonpb"
	"github.com/wearefair/gurl/pkg/log"
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
	target := req.Header.Get(p.proxyTargetHeader)

	// If the header is not set... return a 422, because we really
	// can't process this request. Where are we supposed to forward this to?
	if target == "" {
		log.Error("Proxy target header is empty!")
		rw.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	defer req.Body.Close()
	msg, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Error(err.Error())
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

	cfg := &jsonpb.Config{
		Address:      target,
		ImportPaths:  p.importPaths,
		ServicePaths: p.servicePaths,
	}

	client, err := jsonpb.NewClient(cfg)
	if err != nil {
		log.Error(err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	response, err := client.Call(context.Background(), service, rpc, msg)
	if err != nil {
		log.Error(err)
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	rw.WriteHeader(http.StatusOK)
	rw.Write(response)
}

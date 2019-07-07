package proxy

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/wearefair/gurl/pkg/jsonpb"
	"github.com/wearefair/gurl/pkg/k8"
	"github.com/wearefair/gurl/pkg/log"
	"github.com/wearefair/gurl/pkg/util"
	"google.golang.org/grpc/metadata"
	"k8s.io/client-go/tools/clientcmd"
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

	// TODO: Validation errors on either one of these if they're nil
	service := vars[ServiceKey]
	rpc := vars[RpcKey]

	if service == "" || rpc == "" {
		log.Error(fmt.Errorf("Service or RPC cannot be blank, service: %s, rpc: %s\n", service, rpc))
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	parsedURI, err := util.ParseURI(target)
	if err != nil {
		log.Error(err)
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	// TODO: This is duplicated
	var connector jsonpb.Connector
	if parsedURI.Protocol == util.K8Protocol {
		// Set up port forward, then send request
		req := uriToPortForwardRequest(parsedURI)

		connector = jsonpb.NewK8Connector(k8Config(), req)
	}

	jsonpbReq := &jsonpb.Request{
		Connector:   connector,
		Address:     target,
		DialOptions: p.opts.DialOptions(),
		Service:     service,
		RPC:         rpc,
		Message:     msg,
	}

	// TODO: This ctx isn't used properly
	outgoingMd := mergeHttpHeadersToMetadata(p.opts.Metadata, req.Header)
	ctx := metadata.NewOutgoingContext(context.Background(), outgoingMd)

	response, err := p.caller.Invoke(ctx, jsonpbReq)
	if err != nil {
		log.Error(err)
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	rw.WriteHeader(http.StatusOK)
	rw.Write(response)
}

func mergeHttpHeadersToMetadata(md metadata.MD, headers http.Header) metadata.MD {
	mdCopy := md.Copy()
	for key, vals := range headers {
		mdCopy.Append(key, vals...)
	}

	return mdCopy
}

// Reads K8 config from default location, which is $HOME/.kube/config
func k8Config() clientcmd.ClientConfig {
	// if you want to change the loading rules (which files in which order), you can do so here
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	// if you want to change override values or bind them to flags, there are methods to help you
	configOverrides := &clientcmd.ConfigOverrides{}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
}

// TODO: This is duplicated logic
func uriToPortForwardRequest(uri *util.URI) k8.PortForwardRequest {
	return k8.PortForwardRequest{
		Context: uri.Context,
		// TODO: Make this namespace configurable via URI
		Namespace: "default",
		Service:   uri.Host,
		Port:      uri.Port,
	}
}

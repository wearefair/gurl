package call

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wearefair/gurl/pkg/config"
	"github.com/wearefair/gurl/pkg/jsonpb"
	"github.com/wearefair/gurl/pkg/k8"
	"github.com/wearefair/gurl/pkg/log"
	"github.com/wearefair/gurl/pkg/options"
	"github.com/wearefair/gurl/pkg/util"
	"google.golang.org/grpc/metadata"
)

var (
	data string
	// host:port/service_name/method_name
	port int
	uri  string

	callOptions     = &options.Options{Metadata: metadata.MD{}}
	tlsOptions      = &options.TLS{}
	metadataOptions = flagMetadata(callOptions.Metadata)
	useTls          bool
)

// RootCmd represents the base command when called without any subcommands
var CallCmd = &cobra.Command{
	Use:   "gurl",
	Short: "Curl your gRPC services",
	RunE:  runCall,
}

func init() {
	// Add any flags that were registered on the built-in flag package.
	// This is specifically for configuring glog. We want this to be a persistent
	// flag since we want to be able to handle log configuration for subcommands as well.
	CallCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	flags := CallCmd.Flags()
	flags.StringVarP(&uri, "uri", "u", "", "gRPC URI in the form of host:port/service_name/method_name")
	flags.StringVarP(&data, "data", "d", "", "Data, as JSON string, to send to the gRPC service")
	CallCmd.MarkFlagRequired("uri")
	CallCmd.MarkFlagRequired("data")

	// TLS Options
	flags.BoolVarP(&useTls, "tls", "t", false, "Use TLS to connect to the server")
	flags.BoolVarP(&tlsOptions.Insecure, "tls-insecure", "k", false, "Skip verification of server TLS certificate.")
	flags.StringVarP(&tlsOptions.ServerName, "tls-servername", "N", "", "Override the server name used for the TLS handshake.")

	// Metadata options
	flags.VarP(metadataOptions, "header", "H", "Set header in the format '<Header-Name>:<Header-Value>'")
}

func runCall(cmd *cobra.Command, args []string) error {
	if useTls {
		callOptions.TLS = tlsOptions
	}
	log.Infof("Metadata options: %#v", callOptions.Metadata)
	// Parse and return the URI in a format we can expect
	parsedURI, err := util.ParseURI(uri)
	if err != nil {
		return err
	}
	log.Infof("Parsed URI: %#v", parsedURI)

	// Set up connector
	var connector jsonpb.Connector
	if parsedURI.Protocol == util.K8Protocol {
		// Set up port forward, then send request
		req := k8.PortForwardRequest{
			Context: parsedURI.Context,
			// TODO: Make this namespace configurable via URI
			Namespace: "default",
			Service:   parsedURI.Host,
			Port:      parsedURI.Port,
		}

		connector = jsonpb.NewK8Connector(k8.DefaultConfig(), req)
	}

	// Set up the JSONPB client
	cfg := &jsonpb.Config{
		DialOptions:  callOptions.DialOptions(),
		ImportPaths:  config.Instance().Local.ImportPaths,
		ServicePaths: config.Instance().Local.ServicePaths,
	}
	client, err := jsonpb.NewClient(cfg)
	if err != nil {
		return log.LogAndReturn(err)
	}

	// Send request and get response
	address := fmt.Sprintf("%s:%s", parsedURI.Host, parsedURI.Port)
	req := &jsonpb.Request{
		Connector:   connector,
		Address:     address,
		DialOptions: callOptions.DialOptions(),
		Service:     parsedURI.Service,
		RPC:         parsedURI.RPC,
		Message:     []byte(data),
	}

	response, err := client.Invoke(callOptions.ContextWithOptions(context.Background()), req)
	if err != nil {
		return log.LogAndReturn(err)
	}

	// Prettifying JSON of response
	var prettyResponse bytes.Buffer
	err = json.Indent(&prettyResponse, response, "", "  ")
	if err != nil {
		return log.LogAndReturn(err)
	}
	fmt.Printf("Response:\n%s\n", prettyResponse.String())
	return nil
}

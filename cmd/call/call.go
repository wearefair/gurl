package call

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/spf13/pflag"

	"github.com/spf13/cobra"
	"github.com/wearefair/gurl/pkg/config"
	"github.com/wearefair/gurl/pkg/jsonpb"
	"github.com/wearefair/gurl/pkg/k8"
	"github.com/wearefair/gurl/pkg/log"
	"github.com/wearefair/gurl/pkg/options"
	"github.com/wearefair/gurl/pkg/util"
	"google.golang.org/grpc/metadata"
	"k8s.io/client-go/tools/clientcmd"
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
	flags := CallCmd.Flags()
	// Add any flags that were registered on the built-in flag package.
	flags.AddGoFlagSet(flag.CommandLine)

	ConfigurateFlags(flags)
}

//function to configures flags not only in this project but for those projects that import this one
func ConfigurateFlags(flags *pflag.FlagSet){
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

	address := fmt.Sprintf("%s:%s", parsedURI.Host, parsedURI.Port)
	if parsedURI.Protocol == util.K8Protocol {
		// Set up port forward, then send request
		req := uriToPortForwardRequest(parsedURI)
		pf, err := k8.StartPortForward(k8Config(), req)
		if err != nil {
			return err
		}
		defer pf.Close()

		address = fmt.Sprintf("localhost:%s", pf.LocalPort())
	}

	cfg := &jsonpb.Config{
		Address:      address,
		DialOptions:  callOptions.DialOptions(),
		ImportPaths:  config.Instance().Local.ImportPaths,
		ServicePaths: config.Instance().Local.ServicePaths,
	}

	client, err := jsonpb.NewClient(cfg)
	if err != nil {
		return log.LogAndReturn(err)
	}

	// Send request and get response
	response, err := client.Call(callOptions.ContextWithOptions(context.Background()), parsedURI.Service, parsedURI.RPC, []byte(data))
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

// Reads K8 config from default location, which is $HOME/.kube/config
func k8Config() clientcmd.ClientConfig {
	// if you want to change the loading rules (which files in which order), you can do so here
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	// if you want to change override values or bind them to flags, there are methods to help you
	configOverrides := &clientcmd.ConfigOverrides{}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
}

func uriToPortForwardRequest(uri *util.URI) k8.PortForwardRequest {
	return k8.PortForwardRequest{
		Context: uri.Context,
		// TODO: Make this namespace configurable via URI
		Namespace: "default",
		Service:   uri.Host,
		Port:      uri.Port,
	}
}

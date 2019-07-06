package proxy

import (
	"github.com/spf13/cobra"
	"github.com/wearefair/gurl/pkg/config"
	"github.com/wearefair/gurl/pkg/jsonpb"
	"github.com/wearefair/gurl/pkg/log"
	"github.com/wearefair/gurl/pkg/proxy"
)

var (
	port int
)

var ProxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Run the gURL proxy",
	RunE:  runProxy,
}

func init() {
	ProxyCmd.Flags().IntVarP(&port, "port", "p", 3030, "Port for the proxy to run on")
}

func runProxy(cmd *cobra.Command, args []string) error {
	cfg := proxy.DefaultConfig()
	//	cfg.ImportPaths = config.Instance().Local.ImportPaths
	//	cfg.ServicePaths = config.Instance().Local.ServicePaths

	jsonpbCfg := &jsonpb.Config{
		ImportPaths:  config.Instance().Local.ImportPaths,
		ServicePaths: config.Instance().Local.ServicePaths,
	}

	client, err := jsonpb.NewClient(jsonpbCfg)
	if err != nil {
		return err
	}

	cfg.Caller = client

	proxySrv := proxy.New(cfg)
	log.Infof("Starting server at %d\n", port)
	return proxySrv.Run()
}

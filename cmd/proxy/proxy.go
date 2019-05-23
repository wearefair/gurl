package proxy

import (
	"fmt"

	"github.com/spf13/cobra"
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
	cfg := &proxy.Config{
		Addr: fmt.Sprintf(":%d", port),
	}

	proxySrv := proxy.New(cfg)
	log.Infof("Starting server at %d\n", port)
	return proxySrv.Run()
}

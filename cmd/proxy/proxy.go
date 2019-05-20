package proxy

import (
	"github.com/spf13/cobra"
	gproxy "github.com/wearefair/gurl/pkg/proxy"
)

var ProxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Proxy your gRPC services",
	RunE:  proxyCall,
}

func proxyCall(cmd *cobra.Command, args []string) error {
	return gproxy.New().Run()
}

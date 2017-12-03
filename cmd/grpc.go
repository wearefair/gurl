package cmd

import (
	"github.com/spf13/cobra"
	"github.com/wearefair/gurl/config"
	cligrpc "github.com/wearefair/gurl/grpc"
)

var listServicesCmd = &cobra.Command{
	Use:   "list-services",
	Short: "List all services",
	Run:   listServices,
}

func init() {
	RootCmd.AddCommand(listServicesCmd)
}

func listServices(cmd *cobra.Command, args []string) {
	walker := cligrpc.NewProtoWalker()
	paths := append(config.Instance().Local.ImportPaths, config.Instance().Local.ServicePaths...)
	walker.Collect(paths)
	collector := cligrpc.NewCollector(walker.Descriptors)
	collector.ListServices()
}

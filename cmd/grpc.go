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
	walker.Collect(config.Instance().Local.ImportPaths, config.Instance().Local.ServicePaths)
	collector := cligrpc.NewCollector(walker.GetFileDescriptors())
	collector.ListServices()
}

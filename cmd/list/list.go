package list

import (
	"github.com/spf13/cobra"
	"github.com/wearefair/gurl/pkg/config"
	"github.com/wearefair/gurl/pkg/log"
	"github.com/wearefair/gurl/pkg/protobuf"
)

var ListServicesCmd = &cobra.Command{
	Use:   "list-services",
	Short: "List all services",
	Run:   listServices,
}

func listServices(cmd *cobra.Command, args []string) {
	descriptors, err := protobuf.Collect(config.Instance().Local.ImportPaths, config.Instance().Local.ServicePaths)
	if err != nil {
		log.Logger().Fatal(err.Error())
	}
	collector := protobuf.NewCollector(descriptors)
	collector.ListServices()
}

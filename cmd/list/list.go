package list

import (
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/wearefair/gurl/pkg/config"
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
		glog.Fatal(err)
	}
	collector := protobuf.NewCollector(descriptors)
	collector.ListServices()
}

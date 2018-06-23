package cmd

import (
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/wearefair/gurl/cmd/call"
	configcmd "github.com/wearefair/gurl/cmd/config"
	"github.com/wearefair/gurl/cmd/list"
	"github.com/wearefair/gurl/pkg/config"
)

func Execute() {
	if err := call.CallCmd.Execute(); err != nil {
		glog.Exit(err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	call.CallCmd.AddCommand(list.ListServicesCmd)
	call.CallCmd.AddCommand(configcmd.ConfigCmd)
}

func initConfig() {
	config.Read()
}

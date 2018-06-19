package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wearefair/gurl/cmd/call"
	configcmd "github.com/wearefair/gurl/cmd/config"
	"github.com/wearefair/gurl/cmd/list"
	"github.com/wearefair/gurl/pkg/config"
	"github.com/wearefair/gurl/pkg/log"
)

func Execute() {
	if err := call.CallCmd.Execute(); err != nil {
		fmt.Println(err)
		log.Logger().Sync()
		os.Exit(-1)
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

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/wearefair/gurl/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure grpc curl",
	Run:   configure,
}

func init() {
	RootCmd.AddCommand(configCmd)
}

func configure(cmd *cobra.Command, args []string) {
	config.Read()
	err := config.Prompt()
	if err != nil {
		logger.Fatal(err.Error())
	}
	conf := config.Instance()
	err = config.Save(conf)
	if err != nil {
		logger.Fatal(err.Error())
	}

}

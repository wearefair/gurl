package cmd

import (
	"github.com/spf13/cobra"
	"github.com/wearefair/bouncer/k8"
	"github.com/wearefair/gurl/config"
)

var (
	localPort  int
	remotePort int
)

var portforwardCmd = &cobra.Command{
	Use:   "port-forward",
	Short: "Port forward to Kubernetes",
	RunE:  portForward,
}

func init() {
	RootCmd.AddCommand(portForwardCmd)
	portForwardCmd.Flags().IntVarP(&localPort, "local", "l", 3000, "Local port to set up remote forwarding, defaults to 3000")
	portForwardCmd.Flags().IntVarP(&remotePort, "remote", "r", 0, "Remote port for forwarding")
}

func portForward(cmd *cobra.Command, args []string) error {
	k8, err := k8.New(config.Instance().KubeConfig)
	if err != nil {
		return err
	}
}

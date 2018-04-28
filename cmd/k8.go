package cmd

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/cobra"
	"github.com/wearefair/gurl/k8"
)

var (
	localPort  int
	remotePort int
)

var portForwardCmd = &cobra.Command{
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
	k, err := k8.NewK8("")
	if err != nil {
		return err
	}
	spew.Dump(k.GetPodNameAndRemotePort("fair-auth", "50051"))
	return nil
}

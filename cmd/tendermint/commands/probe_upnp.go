package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/go-p2p/upnp"
)

var probeUpnpCmd = &cobra.Command{
	Use:   "probe_upnp",
	Short: "Test UPnP functionality",
	Run:   probeUpnp,
}

func init() {
	RootCmd.AddCommand(probeUpnpCmd)
}

func probeUpnp(cmd *cobra.Command, args []string) {

	capabilities, err := upnp.Probe()
	if err != nil {
		fmt.Println("Probe failed: %v", err)
	} else {
		fmt.Println("Probe success!")
		jsonBytes, err := json.Marshal(capabilities)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(jsonBytes))
	}

}

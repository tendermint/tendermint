package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/p2p/upnp"
)

var ProbeUpnpCmd = &cobra.Command{
	Use:   "probe_upnp",
	Short: "Test UPnP functionality",
	RunE:  probeUpnp,
}

func probeUpnp(cmd *cobra.Command, args []string) error {
	capabilities, err := upnp.Probe(logger)
	if err != nil {
		fmt.Println("Probe failed: %v", err)
	} else {
		fmt.Println("Probe success!")
		jsonBytes, err := json.Marshal(capabilities)
		if err != nil {
			return err
		}
		fmt.Println(string(jsonBytes))
	}
	return nil
}

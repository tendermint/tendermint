package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/version"
)

// VersionCmd ...
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show protocols' and libraries' version numbers",
	Run: func(cmd *cobra.Command, args []string) {
		values, _ := json.MarshalIndent(struct {
			Tendermint    string
			ABCI          string
			BlockProtocol uint64
			P2PProtocol   uint64
		}{
			Tendermint:    version.TMCoreSemVer,
			ABCI:          version.ABCIVersion,
			BlockProtocol: version.BlockProtocol,
			P2PProtocol:   version.P2PProtocol,
		}, "", "  ")

		fmt.Println(string(values))
	},
}

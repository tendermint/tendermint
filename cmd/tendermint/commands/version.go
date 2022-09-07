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
	Short: "Show version info",
	Run: func(cmd *cobra.Command, args []string) {
		if verbose {
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
		} else {
			fmt.Println(version.TMCoreSemVer)
		}
	},
}

func init() {
	VersionCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show protocol and library versions")
}

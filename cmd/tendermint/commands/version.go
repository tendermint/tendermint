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
				tendermint     string
				abci           string
				block_protocol uint64
				p2p_protocol   uint64
			}{
				tendermint:     version.TMCoreSemVer,
				abci:           version.ABCIVersion,
				block_protocol: version.BlockProtocol,
				p2p_protocol:   version.P2PProtocol,
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

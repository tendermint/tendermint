package commands

import (
	"github.com/spf13/cobra"
)

// AddNodeFlags exposes some common configuration options on the command-line
// These are exposed for convenience of commands embedding a tendermint node

func addMoreFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("p2p.addr_book_strict", config.P2P.AddrBookStrict,
		"Set true for strict address routability rules, Set false for private or local networks")
	cmd.Flags().String("p2p.external_address", config.P2P.ExternalAddress,
		"The address to advertise to other peers for them to dial. If empty, will use the same one as the laddr")
	cmd.Flags().Bool("p2p.allow_duplicate_ip", config.P2P.AllowDuplicateIP,
		"Toggle to disable guard against peers connecting from the same ip")
	//pprof flags
	cmd.Flags().String("prof_laddr", config.ProfListenAddress,
		"Node listen address. (0.0.0.0:0 means any interface, any port)")

	cmd.Flags().Duration("consensus.timeout_commit", config.Consensus.TimeoutCommit, "Set node block interval time")
}

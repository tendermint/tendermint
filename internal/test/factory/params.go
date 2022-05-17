package factory

import (
	"time"

	"github.com/tendermint/tendermint/types"
)

// ConsensusParams returns a default set of ConsensusParams that are suitable
// for use in testing
func ConsensusParams() *types.ConsensusParams {
	c := types.DefaultConsensusParams()
	c.Timeout = types.TimeoutParams{
		Commit:              10 * time.Millisecond,
		Propose:             40 * time.Millisecond,
		ProposeDelta:        1 * time.Millisecond,
		Vote:                10 * time.Millisecond,
		VoteDelta:           1 * time.Millisecond,
		BypassCommitTimeout: true,
	}
	c.ABCI.VoteExtensionsEnableHeight = 1
	return c
}

package consensus

import (
	"fmt"
	. "github.com/tendermint/tendermint/config"
)

func GenVoteDocument(voteType byte, height uint32, round uint16, proposalHash []byte) string {
	stepName := ""
	switch voteType {
	case VoteTypeBare:
		stepName = "bare"
	case VoteTypePrecommit:
		stepName = "precommit"
	case VoteTypeCommit:
		stepName = "commit"
	default:
		panic("Unknown vote type")
	}
	return fmt.Sprintf(
		`-----BEGIN TENDERMINT DOCUMENT-----
URI: %v://consensus/%v/%v/%v
ProposalHash: %X
-----END TENDERMINT DOCUMENHT-----`,
		Config.Network, height, round, stepName,
		proposalHash,
	)
}

func GenBlockPartDocument(height uint32, round uint16, index uint16, total uint16, blockPartHash []byte) string {
	return fmt.Sprintf(
		`-----BEGIN TENDERMINT DOCUMENT-----
URI: %v://blockpart/%v/%v/%v
Total: %v
BlockPartHash: %X
-----END TENDERMINT DOCUMENHT-----`,
		Config.Network, height, round, index,
		total,
		blockPartHash,
	)
}

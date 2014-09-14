package blocks

import (
	"fmt"
	. "github.com/tendermint/tendermint/config"
)

func GenVoteDocument(voteType byte, height uint32, round uint16, blockHash []byte) []byte {
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
	return []byte(fmt.Sprintf(
		`!!!!!BEGIN TENDERMINT VOTE!!!!!
Network: %v
Height: %v
Round: %v
Step: %v
BlockHash: %v
!!!!!END TENDERMINT VOTE!!!!!`,
		Config.Network, height, round, stepName, blockHash,
	))
}

func GenProposalDocument(height uint32, round uint16, blockPartsTotal uint16, blockPartsHash []byte,
	polPartsTotal uint16, polPartsHash []byte) []byte {
	return []byte(fmt.Sprintf(
		`!!!!!BEGIN TENDERMINT PROPOSAL!!!!!
Network: %v
Height: %v
Round: %v
BlockPartsTotal: %v
BlockPartsHash: %X
POLPartsTotal: %v
POLPartsHash: %X
!!!!!END TENDERMINT PROPOSAL!!!!!`,
		Config.Network, height, round, blockPartsTotal, blockPartsHash, polPartsTotal, polPartsHash,
	))
}

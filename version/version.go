package version

import tmversion "github.com/tendermint/tendermint/proto/tendermint/version"

var (
	// TMCoreSemVer is the current version of Tendermint Core.
	// It's the Semantic Version of the software.
	TMCoreSemVer string
)

const (
	// ABCISemVer is the semantic version of the ABCI library
	ABCISemVer = "0.17.0"

	ABCIVersion = ABCISemVer
)

var (
	// P2PProtocol versions all p2p behavior and msgs.
	// This includes proposer selection.
	P2PProtocol uint64 = 8

	// BlockProtocol versions all block data structures and processing.
	// This includes validity of blocks and state updates.
	BlockProtocol uint64 = 11
)

type Consensus struct {
	Block uint64 `json:"block"`
	App   uint64 `json:"app"`
}

func (c Consensus) ToProto() tmversion.Consensus {
	return tmversion.Consensus{
		Block: c.Block,
		App:   c.App,
	}
}

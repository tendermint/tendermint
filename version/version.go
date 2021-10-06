package version

import tmversion "github.com/tendermint/tendermint/proto/tendermint/version"

var (
	// TMVersion is the semantic version of Tendermint Core.
	TMVersion = TMVersionDefault

	// ABCISemVer and ABCIVersion give the semantic version of the ABCI library.
	ABCISemVer  = TMVersion
	ABCIVersion = TMVersion

	// P2PProtocol versions all p2p behavior and msgs.
	// This includes proposer selection.
	P2PProtocol uint64 = 8

	// BlockProtocol versions all block data structures and processing.
	// This includes validity of blocks and state updates.
	BlockProtocol uint64 = 11
)

// TMVersionDefault is the used as the fallback version of Tendermint Core
// when not using git describe. It is formatted with semantic versioning.
const TMVersionDefault = "0.34.11"

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

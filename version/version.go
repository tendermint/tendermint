package version

import (
	"strings"

	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
)

var (
	// TMVersion is the semantic version of Tendermint Core.
	TMVersion = TMVersionDefault

	// ABCISemVer and ABCIVersion give the semantic version of the ABCI library.
	ABCISemVer  = tmToABCIVersion(TMVersion)
	ABCIVersion = tmToABCIVersion(TMVersion)

	// P2PProtocol versions all p2p behavior and msgs.
	// This includes proposer selection.
	P2PProtocol uint64 = 8

	// BlockProtocol versions all block data structures and processing.
	// This includes validity of blocks and state updates.
	BlockProtocol uint64 = 11
)

// TMVersionDefault is the used as the fallback version of Tendermint Core
// when not using git describe. It is formatted with semantic versioning.
const TMVersionDefault = "v0.35.0-unreleased"

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

// tmToABCIVersion converts a Tendermint semantic version into the
// corresponding ABCI semantic version. The ABCI version matches the Tendermint
// version on its major and minor revisions, and does not have a patch version.
//
// For example TM "v0.35.11" corresponds to ABCI "v0.35".
// This function will panic if tmVersion does not have a sensible format.
func tmToABCIVersion(tmVersion string) string {
	parts := strings.SplitN(tmVersion, ".", 3)
	if len(parts) < 2 {
		panic("invalid semantic version string: " + tmVersion)
	}
	return strings.Join(parts[:2], ".")
}

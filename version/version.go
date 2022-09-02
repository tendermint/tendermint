package version

var (
	TMCoreSemVer = TMVersionDefault
)

const (
	// TMVersionDefault is the used as the fallback version of Tendermint Core
	// when not using git describe. It is formatted with semantic versioning.
	TMVersionDefault = "0.37.0-alpha.1"
	// ABCISemVer is the semantic version of the ABCI protocol
	ABCISemVer = "1.0.0"

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

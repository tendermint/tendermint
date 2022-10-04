package version

const (
	// TMVersionDefault is the used as the fallback version of Tendermint Core
	// when not using git describe. It is formatted with semantic versioning.
	TMCoreSemVer = "0.38.0-dev"
	// ABCISemVer is the semantic version of the ABCI protocol
	ABCISemVer  = "1.0.0"
	ABCIVersion = ABCISemVer
	// TMGitVer uses git describe and includes the commit hash which is helpful
	// for the engineering team when working with the tendermint binary
	TMGitVer = TMCoreSemVer
)

var (
	// P2PProtocol versions all p2p behavior and msgs.
	// This includes proposer selection.
	P2PProtocol uint64 = 8

	// BlockProtocol versions all block data structures and processing.
	// This includes validity of blocks and state updates.
	BlockProtocol uint64 = 11
)

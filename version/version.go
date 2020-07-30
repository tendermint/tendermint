package version

var (
	// GitCommit is the current HEAD set using ldflags.
	GitCommit string

	// Version is the built softwares version.
	Version = TMCoreSemVer
)

func init() {
	if GitCommit != "" {
		Version += "-" + GitCommit
	}
}

const (
	// TMCoreSemVer is the current version of Tendermint Core.
	// It's the Semantic Version of the software.
	// Must be a string because scripts like dist.sh read this file.
	// XXX: Don't change the name of this variable or you will break
	// automation :)
	TMCoreSemVer = "0.34.0"

	// ABCISemVer is the semantic version of the ABCI library
	ABCISemVer = "0.17.0"

	ABCIVersion = ABCISemVer
)

var (
	// P2PProtocol versions all p2p behaviour and msgs.
	// This includes proposer selection.
	P2PProtocol uint64 = 8

	// BlockProtocol versions all block data structures and processing.
	// This includes validity of blocks and state updates.
	BlockProtocol uint64 = 11
)

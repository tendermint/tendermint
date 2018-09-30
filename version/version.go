package version

var (
	// GitCommit is the current HEAD set using ldflags.
	GitCommit string

	// Version is the built softwares version.
	Version string = TMCoreSemVer
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
	TMCoreSemVer = "0.25.0"

	// ABCISemVer is the semantic version of the ABCI library
	ABCISemVer  = "0.14.0"
	ABCIVersion = ABCISemVer
)

// Protocol is used for implementation agnostic versioning.
type Protocol uint64

var (
	// P2PProtocol versions all p2p behaviour and msgs.
	P2PProtocol Protocol = 4

	// BlockProtocol versions all block data structures and processing.
	BlockProtocol Protocol = 7
)

//------------------------------------------------------------------------
// Version types

// App includes the protocol and software version for the application.
// This information is included in ResponseInfo. The App.Protocol can be
// updated in ResponseEndBlock.
type App struct {
	Protocol Protocol `json:"protocol"`
	Software string   `json:"software"`
}

// Consensus captures the consensus rules for processing a block in the blockchain,
// including all blockchain data structures and the rules of the application's
// state transition machine.
type Consensus struct {
	Block Protocol `json:"block"`
	App   Protocol `json:"app"`
}

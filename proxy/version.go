package proxy

import (
	abcix "github.com/tendermint/tendermint/abcix/types"
	"github.com/tendermint/tendermint/version"
)

// RequestInfo contains all the information for sending
// the abci.RequestInfo message during handshake with the app.
// It contains only compile-time version information.
var RequestInfo = abcix.RequestInfo{
	Version:      version.Version,
	BlockVersion: version.BlockProtocol,
	P2PVersion:   version.P2PProtocol,
}
